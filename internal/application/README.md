# Application Layer

This layer contains use cases and application business rules.

## Structure

```
application/
├── usecase/    # Use case implementations
│   ├── task/   # Unified task workflow
│   ├── epic/   # EPIC-specific use cases
│   ├── pbi/    # PBI-specific use cases
│   ├── sbi/    # SBI-specific use cases
│   ├── execution/ # Execution use cases
│   ├── workflow/  # Workflow use cases
│   └── health/    # Health check use cases
├── dto/        # Data Transfer Objects
├── port/       # Port interfaces
│   ├── input/  # Input ports (use case interfaces)
│   └── output/ # Output ports (gateways, presenters, etc.)
└── service/    # Application services
```

## Principles

1. **Use case per operation**: Each use case handles one specific operation
2. **Orchestration, not business logic**: Use cases orchestrate domain objects, but don't contain business rules
3. **Dependency Inversion**: Use cases depend on interfaces (ports), not concrete implementations
4. **Transaction boundaries**: Use cases define transaction boundaries

## Port Interfaces

### Output Ports
- **AgentGateway**: Interface for AI agent execution
- **StorageGateway**: Interface for external storage (S3, local)
- **TransactionManager**: Interface for transaction management
- **Presenter**: Interface for presenting results to users

### Input Ports
Use case interfaces for controllers to depend on.

## Testing

Use cases can be tested independently by providing mock implementations of ports.
