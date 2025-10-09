package di

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/adapter/controller/cli"
	agentgateway "github.com/YoshitsuguKoike/deespec/internal/adapter/gateway/agent"
	storagegateway "github.com/YoshitsuguKoike/deespec/internal/adapter/gateway/storage"
	"github.com/YoshitsuguKoike/deespec/internal/adapter/presenter"
	appconfig "github.com/YoshitsuguKoike/deespec/internal/app/config"
	"github.com/YoshitsuguKoike/deespec/internal/application/port/input"
	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
	"github.com/YoshitsuguKoike/deespec/internal/application/service"
	taskusecase "github.com/YoshitsuguKoike/deespec/internal/application/usecase/task"
	workflowusecase "github.com/YoshitsuguKoike/deespec/internal/application/usecase/workflow"
	"github.com/YoshitsuguKoike/deespec/internal/domain/factory"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"github.com/YoshitsuguKoike/deespec/internal/domain/service/strategy"
	sqliterepo "github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence/sqlite"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/transaction"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
)

// Container is the DI container that holds all dependencies
// This implements manual dependency injection for Clean Architecture
type Container struct {
	// Infrastructure Layer - Database
	db *sql.DB

	// Infrastructure Layer - Repositories (SQLite implementations)
	taskRepo      repository.TaskRepository
	epicRepo      repository.EPICRepository
	pbiRepo       repository.PBIRepository
	sbiRepo       repository.SBIRepository
	runLockRepo   repository.RunLockRepository
	stateLockRepo repository.StateLockRepository
	labelRepo     repository.LabelRepository

	// Infrastructure Layer - Gateways
	agentGateway   output.AgentGateway
	storageGateway output.StorageGateway

	// Infrastructure Layer - Transaction Manager
	txManager output.TransactionManager

	// Application Layer - Services
	lockService service.LockService

	// Domain Layer - Factories
	taskFactory *factory.Factory

	// Domain Layer - Strategies
	strategyRegistry *strategy.StrategyRegistry

	// Application Layer - Use Cases
	taskUseCase         input.TaskUseCase
	workflowUseCase     input.WorkflowUseCase
	epicWorkflowUseCase input.EPICWorkflowUseCase
	pbiWorkflowUseCase  input.PBIWorkflowUseCase
	sbiWorkflowUseCase  input.SBIWorkflowUseCase

	// Adapter Layer - Presenters
	presenter output.Presenter

	// Adapter Layer - Controllers
	rootCmd *cobra.Command

	// Configuration
	config Config
}

// Config holds configuration for the container
type Config struct {
	AgentType    string // Agent type (claude-code, gemini-cli, codex)
	OutputFormat string // Output format (cli, json)
	OutputWriter io.Writer
	Version      string
	BuildInfo    string
	DBPath       string // Path to SQLite database file

	// Storage Gateway configuration
	StorageType    string // Storage type: "local", "s3", "mock" (default: "mock")
	StorageBaseDir string // Base directory for local storage (default: ~/.deespec)
	S3Bucket       string // S3 bucket name (for S3 storage)
	S3Prefix       string // S3 key prefix (optional)
	S3Region       string // AWS region (optional, uses default if empty)

	// Lock Service configuration
	LockHeartbeatInterval time.Duration // Heartbeat interval for locks (default: 30s)
	LockCleanupInterval   time.Duration // Cleanup interval for expired locks (default: 60s)

	// Label system configuration
	LabelConfig appconfig.LabelConfig
}

// NewContainer creates and initializes the DI container
func NewContainer(config Config) (*Container, error) {
	c := &Container{
		config: config,
	}

	// Set default output writer
	if c.config.OutputWriter == nil {
		c.config.OutputWriter = os.Stdout
	}

	// Initialize dependencies in dependency order
	if err := c.initializeInfrastructure(); err != nil {
		return nil, fmt.Errorf("failed to initialize infrastructure: %w", err)
	}

	if err := c.initializeDomain(); err != nil {
		return nil, fmt.Errorf("failed to initialize domain: %w", err)
	}

	if err := c.initializeApplication(); err != nil {
		return nil, fmt.Errorf("failed to initialize application: %w", err)
	}

	if err := c.initializeAdapters(); err != nil {
		return nil, fmt.Errorf("failed to initialize adapters: %w", err)
	}

	return c, nil
}

// initializeInfrastructure initializes infrastructure layer components
func (c *Container) initializeInfrastructure() error {
	// 1. Set default database path if not provided
	dbPath := c.config.DBPath
	if dbPath == "" {
		// Default to ~/.deespec/deespec.db
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		dbDir := filepath.Join(homeDir, ".deespec")
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			return fmt.Errorf("failed to create database directory: %w", err)
		}
		dbPath = filepath.Join(dbDir, "deespec.db")
	}

	// 2. Open SQLite database connection
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	c.db = db

	// 3. Run database migrations
	migrator := sqliterepo.NewMigrator(db)
	if err := migrator.Migrate(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// 4. Initialize SQLite Repositories
	c.taskRepo = sqliterepo.NewTaskRepository(db)
	c.epicRepo = sqliterepo.NewEPICRepository(db)
	c.pbiRepo = sqliterepo.NewPBIRepository(db)
	c.sbiRepo = sqliterepo.NewSBIRepository(db)
	c.runLockRepo = sqliterepo.NewRunLockRepository(db)
	c.stateLockRepo = sqliterepo.NewStateLockRepository(db)
	// Note: labelRepo will be initialized when GetLabelRepository() is called
	// This allows it to use the loaded config

	// 5. Initialize SQLite Transaction Manager
	c.txManager = transaction.NewSQLiteTransactionManager(db)

	// 6. Initialize Agent Gateway
	agentType := c.config.AgentType
	if agentType == "" {
		agentType = agentgateway.GetDefaultAgent()
	}

	gateway, err := agentgateway.NewAgentGateway(agentType)
	if err != nil {
		return fmt.Errorf("failed to create agent gateway: %w", err)
	}
	c.agentGateway = gateway

	// 7. Initialize Storage Gateway based on configuration
	storageType := c.config.StorageType
	if storageType == "" {
		storageType = "mock" // Default to mock for backward compatibility
	}

	switch storageType {
	case "local":
		// Use local filesystem storage
		baseDir := c.config.StorageBaseDir
		if baseDir == "" {
			// Default to same directory as database
			baseDir = filepath.Dir(dbPath)
		}
		localGateway, err := storagegateway.NewLocalStorageGateway(baseDir)
		if err != nil {
			return fmt.Errorf("failed to create local storage gateway: %w", err)
		}
		c.storageGateway = localGateway

	case "s3":
		// Use AWS S3 storage
		if c.config.S3Bucket == "" {
			return fmt.Errorf("S3 bucket name is required for S3 storage")
		}
		s3Gateway, err := storagegateway.NewS3StorageGateway(storagegateway.S3Config{
			BucketName: c.config.S3Bucket,
			Prefix:     c.config.S3Prefix,
			Region:     c.config.S3Region,
		})
		if err != nil {
			return fmt.Errorf("failed to create S3 storage gateway: %w", err)
		}
		c.storageGateway = s3Gateway

	case "mock":
		// Use mock storage (in-memory)
		c.storageGateway = storagegateway.NewMockStorageGateway()

	default:
		return fmt.Errorf("unknown storage type: %s", storageType)
	}

	return nil
}

// initializeDomain initializes domain layer components
func (c *Container) initializeDomain() error {
	// 1. Initialize Task Factory
	c.taskFactory = factory.NewFactory()

	// 2. Initialize Strategy Registry
	c.strategyRegistry = strategy.NewStrategyRegistry()

	// 3. Register strategies for each task type
	// TODO: Create and register actual strategy implementations
	// For now, the registry is created but strategies need to be implemented

	return nil
}

// initializeApplication initializes application layer components
func (c *Container) initializeApplication() error {
	// 1. Initialize Task Use Case
	c.taskUseCase = taskusecase.NewTaskUseCaseImpl(
		c.taskRepo,
		c.epicRepo,
		c.pbiRepo,
		c.sbiRepo,
		c.taskFactory,
		c.txManager,
	)

	// 2. Initialize Workflow Use Case
	c.workflowUseCase = workflowusecase.NewWorkflowUseCaseImpl(
		c.taskRepo,
		c.epicRepo,
		c.pbiRepo,
		c.sbiRepo,
		c.strategyRegistry,
		c.txManager,
	)

	// 3. Workflow-specific use cases will be implemented as methods on WorkflowUseCaseImpl
	// For now, we use the main workflow use case which implements all workflow interfaces
	// Type assertions are safe-checked to prevent panics during initialization
	if epic, ok := c.workflowUseCase.(input.EPICWorkflowUseCase); ok {
		c.epicWorkflowUseCase = epic
	}
	if pbi, ok := c.workflowUseCase.(input.PBIWorkflowUseCase); ok {
		c.pbiWorkflowUseCase = pbi
	}
	if sbi, ok := c.workflowUseCase.(input.SBIWorkflowUseCase); ok {
		c.sbiWorkflowUseCase = sbi
	}

	// 4. Initialize Lock Service
	lockConfig := service.LockServiceConfig{
		HeartbeatInterval: c.config.LockHeartbeatInterval,
		CleanupInterval:   c.config.LockCleanupInterval,
	}

	// Set defaults if not configured
	if lockConfig.HeartbeatInterval == 0 {
		lockConfig.HeartbeatInterval = 30 * time.Second
	}
	if lockConfig.CleanupInterval == 0 {
		lockConfig.CleanupInterval = 60 * time.Second
	}

	c.lockService = service.NewLockService(
		c.runLockRepo,
		c.stateLockRepo,
		lockConfig,
	)

	return nil
}

// initializeAdapters initializes adapter layer components
func (c *Container) initializeAdapters() error {
	// 1. Initialize Presenter based on output format
	switch c.config.OutputFormat {
	case "json":
		c.presenter = presenter.NewJSONPresenter(c.config.OutputWriter)
	default: // "cli"
		c.presenter = presenter.NewCLITaskPresenter(c.config.OutputWriter)
	}

	// 2. Initialize Root Command Builder
	rootBuilder := cli.NewRootBuilder(
		c.taskUseCase,
		c.workflowUseCase,
		c.epicWorkflowUseCase,
		c.pbiWorkflowUseCase,
		c.sbiWorkflowUseCase,
		c.presenter,
		c.config.Version,
		c.config.BuildInfo,
	)

	// 3. Build root command with all subcommands
	c.rootCmd = rootBuilder.Build()

	return nil
}

// GetRootCommand returns the root Cobra command
func (c *Container) GetRootCommand() *cobra.Command {
	return c.rootCmd
}

// GetTaskUseCase returns the task use case
func (c *Container) GetTaskUseCase() input.TaskUseCase {
	return c.taskUseCase
}

// GetWorkflowUseCase returns the workflow use case
func (c *Container) GetWorkflowUseCase() input.WorkflowUseCase {
	return c.workflowUseCase
}

// GetPresenter returns the presenter
func (c *Container) GetPresenter() output.Presenter {
	return c.presenter
}

// GetAgentGateway returns the agent gateway
func (c *Container) GetAgentGateway() output.AgentGateway {
	return c.agentGateway
}

// GetStorageGateway returns the storage gateway
func (c *Container) GetStorageGateway() output.StorageGateway {
	return c.storageGateway
}

// GetLockService returns the lock service
func (c *Container) GetLockService() service.LockService {
	return c.lockService
}

// GetLabelRepository returns the label repository
// Initializes on first call with configured LabelConfig
func (c *Container) GetLabelRepository() repository.LabelRepository {
	if c.labelRepo == nil {
		c.labelRepo = sqliterepo.NewLabelRepository(c.db, c.config.LabelConfig)
	}
	return c.labelRepo
}

// Start starts background services (Lock Service, etc.)
func (c *Container) Start(ctx context.Context) error {
	// Start Lock Service for heartbeat and cleanup
	if err := c.lockService.Start(ctx); err != nil {
		return fmt.Errorf("failed to start lock service: %w", err)
	}
	return nil
}

// Close closes all resources held by the container
func (c *Container) Close() error {
	// Stop Lock Service first
	if c.lockService != nil {
		if err := c.lockService.Stop(); err != nil {
			// Log error but continue closing other resources
			fmt.Fprintf(os.Stderr, "Warning: failed to stop lock service: %v\n", err)
		}
	}

	// Close database connection
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}
