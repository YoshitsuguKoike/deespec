package di

import (
	"fmt"
	"io"
	"os"

	"github.com/YoshitsuguKoike/deespec/internal/adapter/controller/cli"
	agentgateway "github.com/YoshitsuguKoike/deespec/internal/adapter/gateway/agent"
	storagegateway "github.com/YoshitsuguKoike/deespec/internal/adapter/gateway/storage"
	"github.com/YoshitsuguKoike/deespec/internal/adapter/presenter"
	"github.com/YoshitsuguKoike/deespec/internal/application/port/input"
	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
	taskusecase "github.com/YoshitsuguKoike/deespec/internal/application/usecase/task"
	workflowusecase "github.com/YoshitsuguKoike/deespec/internal/application/usecase/workflow"
	"github.com/YoshitsuguKoike/deespec/internal/domain/factory"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"github.com/YoshitsuguKoike/deespec/internal/domain/service/strategy"
	mockrepo "github.com/YoshitsuguKoike/deespec/internal/infrastructure/repository/mock"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/transaction"
	"github.com/spf13/cobra"
)

// Container is the DI container that holds all dependencies
// This implements manual dependency injection for Clean Architecture
type Container struct {
	// Infrastructure Layer - Repositories (Mock implementations for Phase 4)
	taskRepo repository.TaskRepository
	epicRepo repository.EPICRepository
	pbiRepo  repository.PBIRepository
	sbiRepo  repository.SBIRepository

	// Infrastructure Layer - Gateways
	agentGateway   output.AgentGateway
	storageGateway output.StorageGateway

	// Infrastructure Layer - Transaction Manager
	txManager output.TransactionManager

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
	// 1. Initialize Mock Repositories (will be replaced with SQLite in Phase 5)
	c.taskRepo = mockrepo.NewMockTaskRepository()
	c.epicRepo = mockrepo.NewMockEPICRepository()
	c.pbiRepo = mockrepo.NewMockPBIRepository()
	c.sbiRepo = mockrepo.NewMockSBIRepository()

	// 2. Initialize Transaction Manager (Mock for Phase 4)
	c.txManager = transaction.NewMockTransactionManager()

	// 3. Initialize Agent Gateway
	agentType := c.config.AgentType
	if agentType == "" {
		agentType = agentgateway.GetDefaultAgent()
	}

	gateway, err := agentgateway.NewAgentGateway(agentType)
	if err != nil {
		return fmt.Errorf("failed to create agent gateway: %w", err)
	}
	c.agentGateway = gateway

	// 4. Initialize Storage Gateway (Mock for Phase 4)
	c.storageGateway = storagegateway.NewMockStorageGateway()

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
	c.epicWorkflowUseCase = c.workflowUseCase.(input.EPICWorkflowUseCase)
	c.pbiWorkflowUseCase = c.workflowUseCase.(input.PBIWorkflowUseCase)
	c.sbiWorkflowUseCase = c.workflowUseCase.(input.SBIWorkflowUseCase)

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
