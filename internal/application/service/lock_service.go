package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/lock"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// LockService manages lock lifecycle, heartbeats, and cleanup
type LockService interface {
	// RunLock operations
	AcquireRunLock(ctx context.Context, lockID lock.LockID, ttl time.Duration) (*lock.RunLock, error)
	ReleaseRunLock(ctx context.Context, lockID lock.LockID) error
	ExtendRunLock(ctx context.Context, lockID lock.LockID, duration time.Duration) error
	FindRunLock(ctx context.Context, lockID lock.LockID) (*lock.RunLock, error)
	ListRunLocks(ctx context.Context) ([]*lock.RunLock, error)

	// StateLock operations
	AcquireStateLock(ctx context.Context, lockID lock.LockID, lockType lock.LockType, ttl time.Duration) (*lock.StateLock, error)
	ReleaseStateLock(ctx context.Context, lockID lock.LockID) error
	ExtendStateLock(ctx context.Context, lockID lock.LockID, duration time.Duration) error
	FindStateLock(ctx context.Context, lockID lock.LockID) (*lock.StateLock, error)
	ListStateLocks(ctx context.Context) ([]*lock.StateLock, error)

	// Lifecycle management
	Start(ctx context.Context) error
	Stop() error
}

// LockServiceConfig holds configuration for lock service
type LockServiceConfig struct {
	HeartbeatInterval time.Duration // How often to send heartbeats
	CleanupInterval   time.Duration // How often to cleanup expired locks
}

// DefaultLockServiceConfig returns default configuration
func DefaultLockServiceConfig() LockServiceConfig {
	return LockServiceConfig{
		HeartbeatInterval: 30 * time.Second,
		CleanupInterval:   60 * time.Second,
	}
}

// LockServiceImpl implements LockService
type LockServiceImpl struct {
	runLockRepo   repository.RunLockRepository
	stateLockRepo repository.StateLockRepository
	config        LockServiceConfig

	// Heartbeat management
	mu              sync.RWMutex
	runHeartbeats   map[string]context.CancelFunc // lockID -> cancel function
	stateHeartbeats map[string]context.CancelFunc // lockID -> cancel function
	cleanupCancel   context.CancelFunc
	stopOnce        sync.Once
}

// NewLockService creates a new lock service
func NewLockService(
	runLockRepo repository.RunLockRepository,
	stateLockRepo repository.StateLockRepository,
	config LockServiceConfig,
) LockService {
	return &LockServiceImpl{
		runLockRepo:     runLockRepo,
		stateLockRepo:   stateLockRepo,
		config:          config,
		runHeartbeats:   make(map[string]context.CancelFunc),
		stateHeartbeats: make(map[string]context.CancelFunc),
	}
}

// Start starts background tasks (heartbeat monitoring, cleanup)
func (s *LockServiceImpl) Start(ctx context.Context) error {
	// Start cleanup scheduler
	cleanupCtx, cleanupCancel := context.WithCancel(ctx)
	s.cleanupCancel = cleanupCancel

	go s.cleanupScheduler(cleanupCtx)

	return nil
}

// Stop stops all background tasks
func (s *LockServiceImpl) Stop() error {
	var stopErr error
	s.stopOnce.Do(func() {
		// Stop cleanup scheduler
		if s.cleanupCancel != nil {
			s.cleanupCancel()
		}

		// Stop all heartbeats
		s.mu.Lock()
		defer s.mu.Unlock()

		for _, cancel := range s.runHeartbeats {
			cancel()
		}
		for _, cancel := range s.stateHeartbeats {
			cancel()
		}

		s.runHeartbeats = make(map[string]context.CancelFunc)
		s.stateHeartbeats = make(map[string]context.CancelFunc)
	})

	return stopErr
}

// AcquireRunLock acquires a run lock and starts heartbeat
func (s *LockServiceImpl) AcquireRunLock(ctx context.Context, lockID lock.LockID, ttl time.Duration) (*lock.RunLock, error) {
	runLock, err := s.runLockRepo.Acquire(ctx, lockID, ttl)
	if err != nil {
		return nil, fmt.Errorf("acquire run lock: %w", err)
	}

	if runLock == nil {
		return nil, nil // Lock already held
	}

	// Start heartbeat goroutine
	s.startRunLockHeartbeat(lockID)

	return runLock, nil
}

// ReleaseRunLock releases a run lock and stops heartbeat
func (s *LockServiceImpl) ReleaseRunLock(ctx context.Context, lockID lock.LockID) error {
	// Stop heartbeat first
	s.stopRunLockHeartbeat(lockID)

	// Release lock
	if err := s.runLockRepo.Release(ctx, lockID); err != nil {
		return fmt.Errorf("release run lock: %w", err)
	}

	return nil
}

// ExtendRunLock extends the TTL of a run lock
func (s *LockServiceImpl) ExtendRunLock(ctx context.Context, lockID lock.LockID, duration time.Duration) error {
	if err := s.runLockRepo.Extend(ctx, lockID, duration); err != nil {
		return fmt.Errorf("extend run lock: %w", err)
	}
	return nil
}

// FindRunLock finds a run lock by ID
func (s *LockServiceImpl) FindRunLock(ctx context.Context, lockID lock.LockID) (*lock.RunLock, error) {
	return s.runLockRepo.Find(ctx, lockID)
}

// ListRunLocks lists all active run locks
func (s *LockServiceImpl) ListRunLocks(ctx context.Context) ([]*lock.RunLock, error) {
	return s.runLockRepo.List(ctx)
}

// AcquireStateLock acquires a state lock and starts heartbeat
func (s *LockServiceImpl) AcquireStateLock(ctx context.Context, lockID lock.LockID, lockType lock.LockType, ttl time.Duration) (*lock.StateLock, error) {
	stateLock, err := s.stateLockRepo.Acquire(ctx, lockID, lockType, ttl)
	if err != nil {
		return nil, fmt.Errorf("acquire state lock: %w", err)
	}

	if stateLock == nil {
		return nil, nil // Lock already held
	}

	// Start heartbeat goroutine
	s.startStateLockHeartbeat(lockID)

	return stateLock, nil
}

// ReleaseStateLock releases a state lock and stops heartbeat
func (s *LockServiceImpl) ReleaseStateLock(ctx context.Context, lockID lock.LockID) error {
	// Stop heartbeat first
	s.stopStateLockHeartbeat(lockID)

	// Release lock
	if err := s.stateLockRepo.Release(ctx, lockID); err != nil {
		return fmt.Errorf("release state lock: %w", err)
	}

	return nil
}

// ExtendStateLock extends the TTL of a state lock
func (s *LockServiceImpl) ExtendStateLock(ctx context.Context, lockID lock.LockID, duration time.Duration) error {
	if err := s.stateLockRepo.Extend(ctx, lockID, duration); err != nil {
		return fmt.Errorf("extend state lock: %w", err)
	}
	return nil
}

// FindStateLock finds a state lock by ID
func (s *LockServiceImpl) FindStateLock(ctx context.Context, lockID lock.LockID) (*lock.StateLock, error) {
	return s.stateLockRepo.Find(ctx, lockID)
}

// ListStateLocks lists all active state locks
func (s *LockServiceImpl) ListStateLocks(ctx context.Context) ([]*lock.StateLock, error) {
	return s.stateLockRepo.List(ctx)
}

// startRunLockHeartbeat starts a heartbeat goroutine for a run lock
func (s *LockServiceImpl) startRunLockHeartbeat(lockID lock.LockID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop existing heartbeat if any
	if cancel, exists := s.runHeartbeats[lockID.String()]; exists {
		cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.runHeartbeats[lockID.String()] = cancel

	go func() {
		ticker := time.NewTicker(s.config.HeartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.runLockRepo.UpdateHeartbeat(context.Background(), lockID); err != nil {
					// Lock might be released or expired, stop heartbeat
					s.stopRunLockHeartbeat(lockID)
					return
				}
			}
		}
	}()
}

// stopRunLockHeartbeat stops the heartbeat goroutine for a run lock
func (s *LockServiceImpl) stopRunLockHeartbeat(lockID lock.LockID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cancel, exists := s.runHeartbeats[lockID.String()]; exists {
		cancel()
		delete(s.runHeartbeats, lockID.String())
	}
}

// startStateLockHeartbeat starts a heartbeat goroutine for a state lock
func (s *LockServiceImpl) startStateLockHeartbeat(lockID lock.LockID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop existing heartbeat if any
	if cancel, exists := s.stateHeartbeats[lockID.String()]; exists {
		cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.stateHeartbeats[lockID.String()] = cancel

	go func() {
		ticker := time.NewTicker(s.config.HeartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.stateLockRepo.UpdateHeartbeat(context.Background(), lockID); err != nil {
					// Lock might be released or expired, stop heartbeat
					s.stopStateLockHeartbeat(lockID)
					return
				}
			}
		}
	}()
}

// stopStateLockHeartbeat stops the heartbeat goroutine for a state lock
func (s *LockServiceImpl) stopStateLockHeartbeat(lockID lock.LockID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cancel, exists := s.stateHeartbeats[lockID.String()]; exists {
		cancel()
		delete(s.stateHeartbeats, lockID.String())
	}
}

// cleanupScheduler periodically cleans up expired locks
func (s *LockServiceImpl) cleanupScheduler(ctx context.Context) {
	ticker := time.NewTicker(s.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Cleanup expired run locks
			if count, err := s.runLockRepo.CleanupExpired(context.Background()); err == nil && count > 0 {
				// Successfully cleaned up expired run locks
			}

			// Cleanup expired state locks
			if count, err := s.stateLockRepo.CleanupExpired(context.Background()); err == nil && count > 0 {
				// Successfully cleaned up expired state locks
			}
		}
	}
}
