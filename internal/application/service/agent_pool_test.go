package service

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAgentPool(t *testing.T) {
	pool := NewAgentPool()
	require.NotNil(t, pool)

	// Verify default limits
	assert.Equal(t, 2, pool.GetMax("claude-code"))
	assert.Equal(t, 1, pool.GetMax("gemini-cli"))
	assert.Equal(t, 1, pool.GetMax("codex"))

	// Verify initial current counts are 0
	assert.Equal(t, 0, pool.GetCurrent("claude-code"))
	assert.Equal(t, 0, pool.GetCurrent("gemini-cli"))
}

func TestNewAgentPoolWithConfig(t *testing.T) {
	config := AgentPoolConfig{
		MaxPerAgent: map[string]int{
			"claude-code": 5,
			"custom-agent": 3,
		},
	}

	pool := NewAgentPoolWithConfig(config)
	require.NotNil(t, pool)

	assert.Equal(t, 5, pool.GetMax("claude-code"))
	assert.Equal(t, 3, pool.GetMax("custom-agent"))
	assert.Equal(t, 1, pool.GetMax("unknown-agent")) // Default
}

func TestAgentPool_TryAcquire_Success(t *testing.T) {
	pool := NewAgentPool()

	// First acquire should succeed
	ok := pool.TryAcquire("claude-code")
	assert.True(t, ok, "First acquire should succeed")
	assert.Equal(t, 1, pool.GetCurrent("claude-code"))

	// Second acquire should succeed (max is 2)
	ok = pool.TryAcquire("claude-code")
	assert.True(t, ok, "Second acquire should succeed")
	assert.Equal(t, 2, pool.GetCurrent("claude-code"))
}

func TestAgentPool_TryAcquire_Failure(t *testing.T) {
	pool := NewAgentPool()

	// Acquire up to limit
	ok := pool.TryAcquire("claude-code")
	assert.True(t, ok)
	ok = pool.TryAcquire("claude-code")
	assert.True(t, ok)

	// Third acquire should fail (max is 2)
	ok = pool.TryAcquire("claude-code")
	assert.False(t, ok, "Third acquire should fail when limit is 2")
	assert.Equal(t, 2, pool.GetCurrent("claude-code"))
}

func TestAgentPool_Release(t *testing.T) {
	pool := NewAgentPool()

	// Acquire then release
	pool.TryAcquire("claude-code")
	assert.Equal(t, 1, pool.GetCurrent("claude-code"))

	pool.Release("claude-code")
	assert.Equal(t, 0, pool.GetCurrent("claude-code"))
}

func TestAgentPool_Release_WhenZero(t *testing.T) {
	pool := NewAgentPool()

	// Release when count is already 0 should not go negative
	pool.Release("claude-code")
	assert.Equal(t, 0, pool.GetCurrent("claude-code"))
}

func TestAgentPool_UnknownAgent_DefaultLimit(t *testing.T) {
	pool := NewAgentPool()

	// Unknown agent should use default limit of 1
	ok := pool.TryAcquire("unknown-agent")
	assert.True(t, ok, "First acquire should succeed with default limit")

	ok = pool.TryAcquire("unknown-agent")
	assert.False(t, ok, "Second acquire should fail with default limit of 1")

	assert.Equal(t, 1, pool.GetMax("unknown-agent"))
}

func TestAgentPool_SetLimit(t *testing.T) {
	pool := NewAgentPool()

	// Update limit
	err := pool.SetLimit("claude-code", 5)
	require.NoError(t, err)
	assert.Equal(t, 5, pool.GetMax("claude-code"))

	// Try acquiring up to new limit
	for i := 0; i < 5; i++ {
		ok := pool.TryAcquire("claude-code")
		assert.True(t, ok, "Acquire %d should succeed", i+1)
	}

	// 6th acquire should fail
	ok := pool.TryAcquire("claude-code")
	assert.False(t, ok, "Acquire beyond new limit should fail")
}

func TestAgentPool_SetLimit_InvalidValue(t *testing.T) {
	pool := NewAgentPool()

	err := pool.SetLimit("claude-code", 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max must be >= 1")

	err = pool.SetLimit("claude-code", -1)
	assert.Error(t, err)
}

func TestAgentPool_GetStats(t *testing.T) {
	pool := NewAgentPool()

	// Acquire some slots
	pool.TryAcquire("claude-code")
	pool.TryAcquire("claude-code")
	pool.TryAcquire("gemini-cli")

	stats := pool.GetStats()

	// Check claude-code stats
	claudeStats, exists := stats["claude-code"]
	require.True(t, exists)
	assert.Equal(t, "claude-code", claudeStats.Agent)
	assert.Equal(t, 2, claudeStats.Current)
	assert.Equal(t, 2, claudeStats.Max)
	assert.False(t, claudeStats.IsAvailable())
	assert.Equal(t, 100.0, claudeStats.UtilizationPercent())

	// Check gemini-cli stats
	geminiStats, exists := stats["gemini-cli"]
	require.True(t, exists)
	assert.Equal(t, "gemini-cli", geminiStats.Agent)
	assert.Equal(t, 1, geminiStats.Current)
	assert.Equal(t, 1, geminiStats.Max)
	assert.False(t, geminiStats.IsAvailable())

	// Check codex stats (not acquired)
	codexStats, exists := stats["codex"]
	require.True(t, exists)
	assert.Equal(t, 0, codexStats.Current)
	assert.Equal(t, 1, codexStats.Max)
	assert.True(t, codexStats.IsAvailable())
	assert.Equal(t, 0.0, codexStats.UtilizationPercent())
}

func TestAgentPool_Reset(t *testing.T) {
	pool := NewAgentPool()

	// Acquire some slots
	pool.TryAcquire("claude-code")
	pool.TryAcquire("claude-code")
	pool.TryAcquire("gemini-cli")

	assert.Equal(t, 2, pool.GetCurrent("claude-code"))
	assert.Equal(t, 1, pool.GetCurrent("gemini-cli"))

	// Reset
	pool.Reset()

	assert.Equal(t, 0, pool.GetCurrent("claude-code"))
	assert.Equal(t, 0, pool.GetCurrent("gemini-cli"))

	// Max should remain unchanged
	assert.Equal(t, 2, pool.GetMax("claude-code"))
	assert.Equal(t, 1, pool.GetMax("gemini-cli"))
}

func TestAgentPool_ConcurrentAccess(t *testing.T) {
	pool := NewAgentPool()
	pool.SetLimit("claude-code", 10) // Higher limit for concurrent test

	var wg sync.WaitGroup
	var startWg sync.WaitGroup
	numGoroutines := 20
	var maxConcurrent int
	var mu sync.Mutex

	// Start all goroutines simultaneously
	startWg.Add(1)

	// Try to acquire concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Wait for all goroutines to be ready
			startWg.Wait()

			if pool.TryAcquire("claude-code") {
				// Track max concurrent
				mu.Lock()
				current := pool.GetCurrent("claude-code")
				if current > maxConcurrent {
					maxConcurrent = current
				}
				mu.Unlock()

				// Hold the slot briefly
				// time.Sleep(1 * time.Millisecond)

				pool.Release("claude-code")
			}
		}()
	}

	// Start all goroutines at once
	startWg.Done()
	wg.Wait()

	// Max concurrent should not exceed limit of 10
	assert.LessOrEqual(t, maxConcurrent, 10, "Max concurrent should not exceed limit")
	assert.Greater(t, maxConcurrent, 0, "At least one acquisition should succeed")

	// After all releases, current should be 0
	assert.Equal(t, 0, pool.GetCurrent("claude-code"))

	t.Logf("Max concurrent: %d out of %d goroutines", maxConcurrent, numGoroutines)
}

func TestAgentPool_MultipleAgents(t *testing.T) {
	pool := NewAgentPool()

	// Acquire for different agents
	ok := pool.TryAcquire("claude-code")
	assert.True(t, ok)
	ok = pool.TryAcquire("claude-code")
	assert.True(t, ok)

	ok = pool.TryAcquire("gemini-cli")
	assert.True(t, ok)

	ok = pool.TryAcquire("codex")
	assert.True(t, ok)

	// Verify counts
	assert.Equal(t, 2, pool.GetCurrent("claude-code"))
	assert.Equal(t, 1, pool.GetCurrent("gemini-cli"))
	assert.Equal(t, 1, pool.GetCurrent("codex"))

	// claude-code should be full
	ok = pool.TryAcquire("claude-code")
	assert.False(t, ok)

	// gemini-cli should be full
	ok = pool.TryAcquire("gemini-cli")
	assert.False(t, ok)

	// Release claude-code
	pool.Release("claude-code")
	assert.Equal(t, 1, pool.GetCurrent("claude-code"))

	// Now claude-code should have space
	ok = pool.TryAcquire("claude-code")
	assert.True(t, ok)
	assert.Equal(t, 2, pool.GetCurrent("claude-code"))
}

func TestAgentStats_IsAvailable(t *testing.T) {
	tests := []struct {
		name      string
		current   int
		max       int
		available bool
	}{
		{"Empty", 0, 2, true},
		{"Partial", 1, 2, true},
		{"Full", 2, 2, false},
		{"Single slot available", 0, 1, true},
		{"Single slot full", 1, 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := AgentStats{
				Agent:   "test",
				Current: tt.current,
				Max:     tt.max,
			}

			assert.Equal(t, tt.available, stats.IsAvailable())
		})
	}
}

func TestAgentStats_UtilizationPercent(t *testing.T) {
	tests := []struct {
		name     string
		current  int
		max      int
		expected float64
	}{
		{"Empty", 0, 2, 0.0},
		{"Half", 1, 2, 50.0},
		{"Full", 2, 2, 100.0},
		{"Three quarters", 3, 4, 75.0},
		{"Zero max", 0, 0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := AgentStats{
				Agent:   "test",
				Current: tt.current,
				Max:     tt.max,
			}

			assert.Equal(t, tt.expected, stats.UtilizationPercent())
		})
	}
}

func TestAgentPool_ConcurrentAcquireRelease(t *testing.T) {
	pool := NewAgentPool()
	pool.SetLimit("test-agent", 5)

	var wg sync.WaitGroup
	numGoroutines := 100
	operationsPerGoroutine := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				if pool.TryAcquire("test-agent") {
					// Simulate work
					current := pool.GetCurrent("test-agent")
					_ = current

					pool.Release("test-agent")
				}
			}
		}(i)
	}

	wg.Wait()

	// After all goroutines finish, current should be 0
	assert.Equal(t, 0, pool.GetCurrent("test-agent"))

	// Max should remain unchanged
	assert.Equal(t, 5, pool.GetMax("test-agent"))
}

func TestAgentPool_StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	pool := NewAgentPool()
	pool.SetLimit("stress-agent", 10)

	var wg sync.WaitGroup
	numGoroutines := 1000
	var maxConcurrent int32
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			if pool.TryAcquire("stress-agent") {
				defer pool.Release("stress-agent")

				// Track max concurrent
				mu.Lock()
				current := pool.GetCurrent("stress-agent")
				if int32(current) > maxConcurrent {
					maxConcurrent = int32(current)
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Verify no goroutines leaked
	assert.Equal(t, 0, pool.GetCurrent("stress-agent"))

	// Max concurrent should not exceed limit
	assert.LessOrEqual(t, maxConcurrent, int32(10))

	t.Logf("Stress test: %d goroutines, max concurrent: %d", numGoroutines, maxConcurrent)
}

// Benchmark tests
func BenchmarkAgentPool_TryAcquire(b *testing.B) {
	pool := NewAgentPool()
	pool.Reset()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.TryAcquire("claude-code")
		pool.Release("claude-code")
	}
}

func BenchmarkAgentPool_Concurrent(b *testing.B) {
	pool := NewAgentPool()
	pool.SetLimit("bench-agent", 10)
	pool.Reset()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if pool.TryAcquire("bench-agent") {
				pool.Release("bench-agent")
			}
		}
	})
}

// Example usage
func ExampleAgentPool() {
	pool := NewAgentPool()

	// Acquire slot for claude-code
	if pool.TryAcquire("claude-code") {
		fmt.Println("Acquired slot for claude-code")
		// Do work...
		pool.Release("claude-code")
		fmt.Println("Released slot for claude-code")
	}

	// Output:
	// Acquired slot for claude-code
	// Released slot for claude-code
}
