package service

import (
	"fmt"
	"sync"
)

// AgentPool manages per-agent concurrency limits
// It tracks how many concurrent executions each agent type can handle
type AgentPool struct {
	maxPerAgent map[string]int // agent -> max concurrent executions allowed
	current     map[string]int // agent -> current number of active executions
	mu          sync.Mutex
}

// AgentPoolConfig holds configuration for agent concurrency limits
type AgentPoolConfig struct {
	MaxPerAgent map[string]int
}

// NewAgentPool creates a new agent pool with default limits
func NewAgentPool() *AgentPool {
	return &AgentPool{
		maxPerAgent: map[string]int{
			"claude-code": 2, // Claude Code can handle 2 concurrent tasks
			"gemini-cli":  1, // Gemini CLI: 1 task at a time
			"codex":       1, // Codex: 1 task at a time
		},
		current: make(map[string]int),
	}
}

// NewAgentPoolWithConfig creates an agent pool with custom configuration
func NewAgentPoolWithConfig(config AgentPoolConfig) *AgentPool {
	pool := &AgentPool{
		maxPerAgent: make(map[string]int),
		current:     make(map[string]int),
	}

	// Copy config to avoid external modifications
	for agent, max := range config.MaxPerAgent {
		pool.maxPerAgent[agent] = max
	}

	return pool
}

// TryAcquire attempts to acquire a slot for the specified agent
// Returns true if successful, false if the pool is full
func (p *AgentPool) TryAcquire(agent string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	max, exists := p.maxPerAgent[agent]
	if !exists {
		max = 1 // Default to 1 concurrent execution for unknown agents
	}

	if p.current[agent] >= max {
		return false // Pool full for this agent
	}

	p.current[agent]++
	return true
}

// Release releases a slot for the specified agent
func (p *AgentPool) Release(agent string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.current[agent] > 0 {
		p.current[agent]--
	}
}

// GetCurrent returns the current number of active executions for an agent
func (p *AgentPool) GetCurrent(agent string) int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.current[agent]
}

// GetMax returns the maximum allowed concurrent executions for an agent
func (p *AgentPool) GetMax(agent string) int {
	p.mu.Lock()
	defer p.mu.Unlock()

	max, exists := p.maxPerAgent[agent]
	if !exists {
		return 1 // Default
	}
	return max
}

// GetStats returns current usage statistics for all agents
func (p *AgentPool) GetStats() map[string]AgentStats {
	p.mu.Lock()
	defer p.mu.Unlock()

	stats := make(map[string]AgentStats)
	for agent, max := range p.maxPerAgent {
		stats[agent] = AgentStats{
			Agent:   agent,
			Current: p.current[agent],
			Max:     max,
		}
	}

	return stats
}

// SetLimit updates the maximum concurrent executions for an agent
func (p *AgentPool) SetLimit(agent string, max int) error {
	if max < 1 {
		return fmt.Errorf("max must be >= 1, got: %d", max)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.maxPerAgent[agent] = max
	return nil
}

// Reset clears all current counts (useful for testing)
func (p *AgentPool) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.current = make(map[string]int)
}

// AgentStats represents usage statistics for a single agent
type AgentStats struct {
	Agent   string
	Current int
	Max     int
}

// IsAvailable checks if the agent has available slots
func (s AgentStats) IsAvailable() bool {
	return s.Current < s.Max
}

// UtilizationPercent returns the utilization percentage (0-100)
func (s AgentStats) UtilizationPercent() float64 {
	if s.Max == 0 {
		return 0
	}
	return float64(s.Current) / float64(s.Max) * 100
}
