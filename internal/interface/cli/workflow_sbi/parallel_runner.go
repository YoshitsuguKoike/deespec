package workflow_sbi

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/service"
	"github.com/YoshitsuguKoike/deespec/internal/application/workflow"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/lock"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/di"
)

// ExecuteTurnFunc is a function type for executing a single SBI turn
// It takes a context, container, SBI ID, and autoFB flag
type ExecuteTurnFunc func(ctx context.Context, container *di.Container, sbiID string, autoFB bool) error

// ParallelSBIWorkflowRunner executes multiple SBIs concurrently
// It implements the WorkflowRunner interface for parallel SBI processing
type ParallelSBIWorkflowRunner struct {
	enabled     bool
	maxParallel int                // Maximum number of concurrent SBI executions
	container   *di.Container      // Shared DI container
	executeTurn ExecuteTurnFunc    // Function to execute a single SBI turn
	agentPool   *service.AgentPool // Optional agent pool for per-agent concurrency control
	mu          sync.RWMutex       // Protects enabled flag
}

// NewParallelSBIWorkflowRunner creates a new parallel SBI workflow runner
func NewParallelSBIWorkflowRunner(container *di.Container, maxParallel int, executeTurn ExecuteTurnFunc) *ParallelSBIWorkflowRunner {
	if maxParallel < 1 {
		maxParallel = 1 // Default to sequential execution
	}
	if maxParallel > 10 {
		maxParallel = 10 // Cap at 10 for SQLite performance
	}

	return &ParallelSBIWorkflowRunner{
		enabled:     true,
		maxParallel: maxParallel,
		container:   container,
		executeTurn: executeTurn,
		agentPool:   nil, // No agent pool by default
	}
}

// NewParallelSBIWorkflowRunnerWithAgentPool creates a new parallel runner with agent pool
func NewParallelSBIWorkflowRunnerWithAgentPool(
	container *di.Container,
	maxParallel int,
	executeTurn ExecuteTurnFunc,
	agentPool *service.AgentPool,
) *ParallelSBIWorkflowRunner {
	runner := NewParallelSBIWorkflowRunner(container, maxParallel, executeTurn)
	runner.agentPool = agentPool
	return runner
}

// Name returns the workflow name
func (r *ParallelSBIWorkflowRunner) Name() string {
	return "sbi-parallel"
}

// Description returns a human-readable description
func (r *ParallelSBIWorkflowRunner) Description() string {
	return fmt.Sprintf("Parallel SBI workflow (max: %d concurrent tasks)", r.maxParallel)
}

// IsEnabled checks if the workflow should be executed
func (r *ParallelSBIWorkflowRunner) IsEnabled() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.enabled
}

// SetEnabled sets the enabled state
func (r *ParallelSBIWorkflowRunner) SetEnabled(enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enabled = enabled
}

// Run executes multiple SBIs in parallel
func (r *ParallelSBIWorkflowRunner) Run(ctx context.Context, config workflow.WorkflowConfig) error {
	// Check for cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Get services from container
	sbiRepo := r.container.GetSBIRepository()
	lockService := r.container.GetLockService()

	// Extract AutoFB from config
	autoFB := config.AutoFB

	// Fetch executable SBIs (up to maxParallel)
	// Use alternative implementation that directly queries repository
	// without changing SBI state
	sbis, err := r.fetchExecutableSBIsAlt(ctx, sbiRepo, r.maxParallel)
	if err != nil {
		return fmt.Errorf("failed to fetch executable SBIs: %w", err)
	}

	if len(sbis) == 0 {
		// No tasks to execute
		return nil
	}

	// Create conflict detector for this execution batch
	conflictDetector := service.NewConflictDetector()

	// Execute SBIs in parallel with semaphore control
	var wg sync.WaitGroup
	sem := make(chan struct{}, r.maxParallel) // Semaphore for concurrency control
	errChan := make(chan error, len(sbis))    // Buffered channel for error collection

	for _, currentSBI := range sbis {
		// Check for cancellation before starting each task
		select {
		case <-ctx.Done():
			// Context cancelled, stop starting new tasks
			break
		default:
		}

		// Skip if file conflict detected
		if conflictDetector.HasConflict(currentSBI) {
			// Skip this SBI to avoid concurrent file modifications
			continue
		}

		// Check agent pool availability (if enabled)
		agent := currentSBI.Metadata().AssignedAgent
		if r.agentPool != nil {
			if !r.agentPool.TryAcquire(agent) {
				// Agent pool full for this agent, skip
				continue
			}
		}

		// Register file paths IMMEDIATELY after all checks
		// This prevents race conditions where multiple SBIs pass the checks
		// before any of them register their files
		conflictDetector.Register(currentSBI)

		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore

		go func(s *sbi.SBI, agentName string) {
			defer wg.Done()
			defer func() { <-sem }()             // Release semaphore
			defer conflictDetector.Unregister(s) // Unregister on goroutine exit

			// Release agent pool slot on exit
			if r.agentPool != nil {
				defer r.agentPool.Release(agentName)
			}

			// Acquire SBI-specific lock
			lockID, err := lock.NewLockID(fmt.Sprintf("sbi-%s", s.ID()))
			if err != nil {
				errChan <- fmt.Errorf("SBI %s: failed to create lock ID: %w", s.ID(), err)
				return
			}

			sbiLock, err := lockService.AcquireStateLock(ctx, lockID, lock.LockTypeWrite, 10*time.Minute)
			if err != nil {
				errChan <- fmt.Errorf("SBI %s: failed to acquire lock: %w", s.ID(), err)
				return
			}

			if sbiLock == nil {
				// Another worker is processing this SBI, skip
				return
			}

			defer func() {
				if err := lockService.ReleaseStateLock(ctx, lockID); err != nil {
					// Log error but don't fail
				}
			}()

			// Execute turn for this SBI
			if err := r.executeTurn(ctx, r.container, s.ID().String(), autoFB); err != nil {
				errChan <- fmt.Errorf("SBI %s: %w", s.ID(), err)
			}
		}(currentSBI, agent)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Collect errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		// Return first error (could be enhanced to return all errors)
		return fmt.Errorf("parallel execution errors: %v", errors[0])
	}

	return nil
}

// Validate checks if the workflow can be executed
func (r *ParallelSBIWorkflowRunner) Validate() error {
	if r.maxParallel < 1 {
		return fmt.Errorf("maxParallel must be >= 1, got: %d", r.maxParallel)
	}
	if r.container == nil {
		return fmt.Errorf("container is nil")
	}
	if r.executeTurn == nil {
		return fmt.Errorf("executeTurn function is nil")
	}
	return nil
}

// fetchExecutableSBIs retrieves SBIs ready for execution
// Returns up to 'limit' SBIs that are in PENDING, PICKED, or IMPLEMENTING status
func (r *ParallelSBIWorkflowRunner) fetchExecutableSBIs(
	ctx context.Context,
	sbiRepo repository.SBIRepository,
	limit int,
) ([]*sbi.SBI, error) {
	// Use SBIExecutionService to pick multiple SBIs
	// For now, we'll pick one at a time in a loop (can be optimized later)
	var sbis []*sbi.SBI

	sbiExecService := service.NewSBIExecutionService(sbiRepo, r.container.GetLockService())

	for i := 0; i < limit; i++ {
		nextSBI, err := sbiExecService.PickNextSBI(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to pick SBI: %w", err)
		}

		if nextSBI == nil {
			// No more SBIs available
			break
		}

		sbis = append(sbis, nextSBI)
	}

	return sbis, nil
}

// Alternative implementation using repository directly
func (r *ParallelSBIWorkflowRunner) fetchExecutableSBIsAlt(
	ctx context.Context,
	sbiRepo repository.SBIRepository,
	limit int,
) ([]*sbi.SBI, error) {
	filter := repository.SBIFilter{
		Statuses: []model.Status{
			model.StatusPending,
			model.StatusPicked,
			model.StatusImplementing,
		},
		Limit:  limit,
		Offset: 0,
	}

	return sbiRepo.List(ctx, filter)
}
