# Clean Architecture Design for DeeSpec

## Overview

This document outlines the migration plan to transform DeeSpec into a Clean Architecture system, improving maintainability, testability, and separation of concerns.

## Current Architecture Analysis

### Existing Structure
```
internal/
├── app/          # Mixed application services and config
├── domain/       # Partial domain entities (execution, sbi)
├── infra/        # Infrastructure implementations
├── interface/    # CLI controllers and external interfaces
├── usecase/      # Some use cases (sbi registration)
├── validator/    # Validation logic (misplaced)
└── workflow/     # Workflow logic (misplaced)
```

### Issues with Current Architecture
1. **Mixed Responsibilities**: `app/` contains both configuration and business logic
2. **Scattered Domain Logic**: Validators and workflow logic are outside domain layer
3. **Direct Infrastructure Dependencies**: CLI commands directly access file system
4. **Tight Coupling**: State management mixed with CLI implementation
5. **Missing Abstractions**: No clear ports/adapters pattern

## Proposed Clean Architecture

### Layer Structure

```
internal/
├── domain/           # Enterprise Business Rules
│   ├── entity/       # Core business entities
│   ├── value/        # Value objects
│   └── service/      # Domain services
├── application/      # Application Business Rules
│   ├── usecase/      # Use case interactors
│   ├── port/         # Interface definitions (ports)
│   └── dto/          # Data transfer objects
├── adapter/          # Interface Adapters
│   ├── controller/   # Input adapters (CLI, API)
│   ├── presenter/    # Output adapters
│   └── gateway/      # External service adapters
└── infrastructure/   # Frameworks & Drivers
    ├── persistence/  # Database, file system
    ├── config/       # Configuration loading
    └── external/     # External tools (claude, etc.)
```

## Domain Layer Design

### Core Entities

```go
// domain/entity/sbi.go
type SBI struct {
    ID          SBIID
    TaskID      TaskID
    Label       Label
    Status      Status
    Priority    Priority
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

// domain/entity/execution.go
type Execution struct {
    ID          ExecutionID
    SBIID       SBIID
    Turn        Turn
    Step        Step
    Status      ExecutionStatus
    Decision    Decision
    Attempt     Attempt
    StartedAt   time.Time
    CompletedAt *time.Time
}

// domain/entity/workflow.go
type Workflow struct {
    ID          WorkflowID
    Name        string
    Steps       []WorkflowStep
    MaxTurns    int
    MaxAttempts int
}
```

### Value Objects

```go
// domain/value/sbi_id.go
type SBIID string

func NewSBIID(prefix string) (SBIID, error) {
    // Validation and generation logic
}

// domain/value/turn.go
type Turn struct {
    value int
}

func NewTurn(v int) (Turn, error) {
    if v < 1 || v > 100 {
        return Turn{}, ErrInvalidTurn
    }
    return Turn{value: v}, nil
}

// domain/value/attempt.go
type Attempt struct {
    value int
    max   int
}
```

### Domain Services

```go
// domain/service/execution_service.go
type ExecutionService interface {
    CanTransition(from Step, to Step) bool
    DetermineNextStep(current Step, decision Decision) Step
    ShouldForceTerminate(turn Turn, attempt Attempt) bool
}

// domain/service/lock_service.go
type LockService interface {
    AcquireLock(sbiID SBIID, ttl time.Duration) (Lock, error)
    ReleaseLock(lock Lock) error
    IsLockExpired(lock Lock) bool
}
```

## Application Layer Design

### Use Cases

```go
// application/usecase/run_sbi.go
type RunSBIUseCase struct {
    sbiRepo      port.SBIRepository
    execRepo     port.ExecutionRepository
    lockService  port.LockService
    agentGateway port.AgentGateway
    presenter    port.ExecutionPresenter
}

func (uc *RunSBIUseCase) Execute(input RunSBIInput) error {
    // 1. Acquire lock
    // 2. Load SBI and current execution
    // 3. Check turn/attempt limits
    // 4. Execute agent
    // 5. Update execution state
    // 6. Present results
}
```

### Ports (Interfaces)

```go
// application/port/repository.go
type SBIRepository interface {
    Find(id SBIID) (*domain.SBI, error)
    Save(sbi *domain.SBI) error
    List(filter SBIFilter) ([]*domain.SBI, error)
}

type ExecutionRepository interface {
    FindCurrent(sbiID SBIID) (*domain.Execution, error)
    Save(exec *domain.Execution) error
    ListHistory(sbiID SBIID) ([]*domain.Execution, error)
}

// application/port/gateway.go
type AgentGateway interface {
    Execute(ctx context.Context, prompt string, timeout time.Duration) (AgentResponse, error)
}

// application/port/presenter.go
type ExecutionPresenter interface {
    PresentStatus(exec *domain.Execution)
    PresentProgress(progress Progress)
    PresentError(err error)
}
```

## Adapter Layer Design

### Controllers

```go
// adapter/controller/cli/run_controller.go
type RunController struct {
    runUseCase *usecase.RunSBIUseCase
}

func (c *RunController) Handle(cmd *cobra.Command, args []string) error {
    // 1. Parse CLI arguments
    // 2. Create use case input
    // 3. Execute use case
    // 4. Handle errors
}
```

### Presenters

```go
// adapter/presenter/cli/execution_presenter.go
type CLIExecutionPresenter struct {
    writer io.Writer
}

func (p *CLIExecutionPresenter) PresentStatus(exec *domain.Execution) {
    // Format and display execution status
}
```

### Gateways

```go
// adapter/gateway/claude_gateway.go
type ClaudeGateway struct {
    binary  string
    timeout time.Duration
}

func (g *ClaudeGateway) Execute(ctx context.Context, prompt string) (AgentResponse, error) {
    // Execute claude CLI and parse response
}
```

## Infrastructure Layer Design

### Persistence

```go
// infrastructure/persistence/file/sbi_repository.go
type FileSBIRepository struct {
    basePath string
    fs       FileSystem
}

func (r *FileSBIRepository) Find(id SBIID) (*domain.SBI, error) {
    // Read from file system
}

// infrastructure/persistence/sqlite/execution_repository.go
type SQLiteExecutionRepository struct {
    db *sql.DB
}

func (r *SQLiteExecutionRepository) Save(exec *domain.Execution) error {
    // Save to SQLite database
}
```

### Configuration

```go
// infrastructure/config/loader.go
type ConfigLoader struct {
    paths []string
}

func (l *ConfigLoader) Load() (*AppConfig, error) {
    // Load from setting.json, with fallback to defaults
}
```

## Migration Plan

### Phase 1: Domain Layer Extraction (Week 1-2)
1. Extract entities from existing code
2. Create value objects for IDs, status, etc.
3. Define domain services interfaces
4. Move validation logic to domain

### Phase 2: Use Case Layer (Week 2-3)
1. Extract use cases from CLI commands
2. Define repository interfaces
3. Create DTOs for input/output
4. Implement use case tests

### Phase 3: Adapter Layer (Week 3-4)
1. Refactor CLI commands to controllers
2. Implement presenters for output
3. Create gateways for external services
4. Add adapter tests

### Phase 4: Infrastructure Refactoring (Week 4-5)
1. Move file operations to repositories
2. Extract configuration loading
3. Prepare SQLite repositories
4. Add infrastructure tests

### Phase 5: Integration & Testing (Week 5-6)
1. Wire up dependency injection
2. Integration testing
3. Performance testing
4. Documentation update

## Benefits of Migration

1. **Testability**: Each layer can be tested independently
2. **Maintainability**: Clear separation of concerns
3. **Flexibility**: Easy to swap implementations (file → SQLite)
4. **Scalability**: Clean boundaries for feature additions
5. **Documentation**: Code structure documents the business

## Backward Compatibility

- Keep existing CLI commands unchanged
- Maintain file format compatibility
- Gradual migration with feature flags
- Parallel implementation during transition

## Future Enhancements

### SQLite Integration
- Transaction support with proper ACID guarantees
- Better querying capabilities
- Performance improvements for large datasets

### API Support
- REST/gRPC API alongside CLI
- WebSocket for real-time updates
- Multi-user support

### Plugin Architecture
- Custom validators
- External agent integrations
- Workflow extensions

## Testing Strategy

### Unit Tests
- Domain entities and value objects
- Use case business logic
- Individual adapters

### Integration Tests
- Repository implementations
- Gateway integrations
- End-to-end workflows

### Contract Tests
- Interface compliance
- Data format compatibility
- API contracts

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Breaking existing functionality | Comprehensive test suite before migration |
| Performance regression | Benchmark critical paths |
| Complex dependency injection | Use simple DI container |
| Team learning curve | Incremental migration with documentation |

## Success Metrics

- Test coverage > 80%
- No performance regression
- Zero breaking changes
- Improved developer productivity
- Easier feature additions

## Conclusion

This Clean Architecture migration will transform DeeSpec into a more maintainable, testable, and extensible system while preserving all existing functionality and preparing for future enhancements like SQLite integration.