package txn

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// ThresholdConfig defines CI/CD threshold monitoring configuration
type ThresholdConfig struct {
	// SuccessRateThreshold is the minimum acceptable success rate (percentage)
	SuccessRateThreshold float64 `json:"success_rate_threshold"`

	// MaxCASConflicts is the maximum acceptable CAS conflicts
	MaxCASConflicts int64 `json:"max_cas_conflicts"`

	// MaxRecoveryCount is the maximum acceptable recovery operations
	MaxRecoveryCount int64 `json:"max_recovery_count"`

	// MinTotalCommits is the minimum total commits required for threshold checking
	MinTotalCommits int64 `json:"min_total_commits"`

	// Enabled determines if threshold checking is active
	Enabled bool `json:"enabled"`
}

// DefaultThresholdConfig returns a sensible default configuration
func DefaultThresholdConfig() *ThresholdConfig {
	return &ThresholdConfig{
		SuccessRateThreshold: 95.0, // 95% success rate
		MaxCASConflicts:      100,  // Maximum 100 CAS conflicts
		MaxRecoveryCount:     10,   // Maximum 10 recovery operations
		MinTotalCommits:      5,    // Need at least 5 commits for meaningful threshold
		Enabled:              true,
	}
}

// ThresholdResult represents the result of threshold checking
type ThresholdResult struct {
	Passed       bool                   `json:"passed"`
	CheckedAt    string                 `json:"checked_at"`
	Metrics      *MetricsCollector      `json:"metrics"`
	Config       *ThresholdConfig       `json:"config"`
	FailedChecks []string               `json:"failed_checks,omitempty"`
	Warnings     []string               `json:"warnings,omitempty"`
	SuccessRate  float64                `json:"success_rate"`
	TotalCommits int64                  `json:"total_commits"`
	Details      map[string]interface{} `json:"details"`
}

// CheckThresholds validates metrics against configured thresholds
func (m *MetricsCollector) CheckThresholds(config *ThresholdConfig) *ThresholdResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := m.GetSnapshot()
	result := &ThresholdResult{
		Passed:       true,
		CheckedAt:    time.Now().UTC().Format(time.RFC3339),
		Metrics:      &snapshot,
		Config:       config,
		FailedChecks: []string{},
		Warnings:     []string{},
		Details:      make(map[string]interface{}),
	}

	// Calculate derived metrics
	result.SuccessRate = m.GetSuccessRate()
	result.TotalCommits = m.GetTotalCommits()

	// Skip threshold checking if not enabled
	if !config.Enabled {
		result.Warnings = append(result.Warnings, "Threshold checking is disabled")
		result.Details["enabled"] = false
		return result
	}

	// Check if we have enough data for meaningful threshold checking
	if result.TotalCommits < config.MinTotalCommits {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Insufficient commits for threshold checking (need %d, have %d)",
				config.MinTotalCommits, result.TotalCommits))
		result.Details["insufficient_data"] = true
		return result
	}

	// Check success rate threshold
	if result.SuccessRate < config.SuccessRateThreshold {
		result.Passed = false
		result.FailedChecks = append(result.FailedChecks,
			fmt.Sprintf("Success rate %.2f%% is below threshold %.2f%%",
				result.SuccessRate, config.SuccessRateThreshold))
		result.Details["success_rate_failed"] = true
	}

	// Check CAS conflicts threshold
	if m.CASConflicts > config.MaxCASConflicts {
		result.Passed = false
		result.FailedChecks = append(result.FailedChecks,
			fmt.Sprintf("CAS conflicts %d exceed threshold %d",
				m.CASConflicts, config.MaxCASConflicts))
		result.Details["cas_conflicts_failed"] = true
	}

	// Check recovery count threshold
	if m.RecoveryCount > config.MaxRecoveryCount {
		result.Passed = false
		result.FailedChecks = append(result.FailedChecks,
			fmt.Sprintf("Recovery count %d exceeds threshold %d",
				m.RecoveryCount, config.MaxRecoveryCount))
		result.Details["recovery_count_failed"] = true
	}

	// Add warning thresholds (80% of failure thresholds)
	warningSuccessRate := config.SuccessRateThreshold - 2.0
	if result.SuccessRate < config.SuccessRateThreshold && result.SuccessRate >= warningSuccessRate {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Success rate %.2f%% is approaching threshold %.2f%%",
				result.SuccessRate, config.SuccessRateThreshold))
	}

	if m.CASConflicts > config.MaxCASConflicts*8/10 && m.CASConflicts <= config.MaxCASConflicts {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("CAS conflicts %d approaching threshold %d",
				m.CASConflicts, config.MaxCASConflicts))
	}

	if m.RecoveryCount > config.MaxRecoveryCount*8/10 && m.RecoveryCount <= config.MaxRecoveryCount {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Recovery count %d approaching threshold %d",
				m.RecoveryCount, config.MaxRecoveryCount))
	}

	// Store additional details for CI/CD systems
	result.Details["success_rate_percentage"] = result.SuccessRate
	result.Details["threshold_check_enabled"] = true
	result.Details["total_failed_checks"] = len(result.FailedChecks)
	result.Details["total_warnings"] = len(result.Warnings)

	return result
}

// LoadThresholdConfig loads threshold configuration from file
func LoadThresholdConfig(configPath string) (*ThresholdConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if file doesn't exist
			return DefaultThresholdConfig(), nil
		}
		return nil, fmt.Errorf("read threshold config: %w", err)
	}

	var config ThresholdConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unmarshal threshold config: %w", err)
	}

	return &config, nil
}

// SaveThresholdConfig saves threshold configuration to file
func SaveThresholdConfig(config *ThresholdConfig, configPath string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal threshold config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("write threshold config: %w", err)
	}

	return nil
}

// CheckMetricsThresholds is a convenience function for CI/CD integration
func CheckMetricsThresholds(metricsPath, configPath string) (*ThresholdResult, error) {
	// Load metrics
	metrics, err := LoadMetrics(metricsPath)
	if err != nil {
		return nil, fmt.Errorf("load metrics: %w", err)
	}

	// Load threshold configuration
	config, err := LoadThresholdConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("load threshold config: %w", err)
	}

	// Perform threshold check
	result := metrics.CheckThresholds(config)

	// Log result for CI/CD visibility
	if result.Passed {
		fmt.Fprintf(os.Stderr, "INFO: Metrics threshold check passed success_rate=%.2f%% total_commits=%d\n",
			result.SuccessRate, result.TotalCommits)
	} else {
		fmt.Fprintf(os.Stderr, "ERROR: Metrics threshold check failed checks=%d warnings=%d\n",
			len(result.FailedChecks), len(result.Warnings))
		for _, check := range result.FailedChecks {
			fmt.Fprintf(os.Stderr, "ERROR: Threshold failure: %s\n", check)
		}
	}

	for _, warning := range result.Warnings {
		fmt.Fprintf(os.Stderr, "WARN: Threshold warning: %s\n", warning)
	}

	return result, nil
}

// ExitCodeFromThresholds returns appropriate exit code for CI/CD systems
func ExitCodeFromThresholds(result *ThresholdResult) int {
	if !result.Passed {
		return 1 // Failure exit code
	}
	if len(result.Warnings) > 0 {
		return 2 // Warning exit code
	}
	return 0 // Success exit code
}
