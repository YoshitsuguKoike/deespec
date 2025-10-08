# Domain Layer

This layer contains the core business logic and domain models.

## Structure

```
domain/
├── model/          # Domain entities and value objects
│   ├── task/      # Task interface (EPIC/PBI/SBI abstraction)
│   ├── epic/      # EPIC aggregate
│   ├── pbi/       # PBI aggregate
│   ├── sbi/       # SBI aggregate
│   ├── execution/ # Execution aggregate
│   ├── workflow/  # Workflow aggregate
│   ├── state/     # State aggregate
│   ├── agent/     # Agent aggregate
│   └── journal/   # Journal entity
├── service/       # Domain services
│   └── strategy/  # Implementation strategies
└── repository/    # Repository interfaces (ports)
```

## Principles

1. **No dependencies on outer layers**: Domain layer should not depend on Application, Adapter, or Infrastructure layers
2. **Rich domain models**: Entities and value objects contain business logic
3. **Interfaces, not implementations**: Repository interfaces are defined here, but implemented in Infrastructure layer
4. **Invariants protection**: Domain models enforce business rules and invariants

## Key Concepts

### Task Abstraction
EPIC, PBI, and SBI all implement the `Task` interface, allowing polymorphic handling.

### Aggregates
- **EPIC**: Large feature groups, can contain PBIs
- **PBI**: Medium-sized tasks, can contain SBIs
- **SBI**: Small tasks (implementation units)
- **Execution**: Execution state and history
- **Workflow**: Workflow configuration

### Value Objects
All IDs, statuses, and domain-specific types are implemented as value objects to ensure type safety and encapsulation.

## Testing

Mock implementations are provided in `*_test.go` files for testing purposes.
