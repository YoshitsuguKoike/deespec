package config

import "time"

// Config provides read-only access to application configuration.
// This interface abstracts the configuration source (JSON, ENV, defaults)
// and ensures the app layer doesn't depend on infrastructure details.
type Config interface {
	// Core settings
	Home() string           // Base directory for DeeSpec (DEE_HOME)
	AgentBin() string       // Agent binary path (DEE_AGENT_BIN)
	TimeoutSec() int        // Execution timeout in seconds (DEE_TIMEOUT_SEC)
	Timeout() time.Duration // Execution timeout as Duration

	// Workflow variables
	ProjectName() string // Project name (DEE_PROJECT_NAME)
	Language() string    // Language setting (DEE_LANGUAGE)
	Turn() string        // Current turn number (DEE_TURN)
	TaskID() string      // Task ID (DEE_TASK_ID)

	// Feature flags
	Validate() bool    // Enable journal validation (DEE_VALIDATE)
	AutoFB() bool      // Enable auto feedback (DEE_AUTO_FB)
	StrictFsync() bool // Treat fsync failures as errors (DEE_STRICT_FSYNC)

	// Execution limits
	MaxAttempts() int // Maximum attempts before force termination
	MaxTurns() int    // Maximum turns allowed for execution

	// Transaction settings
	TxDestRoot() string    // Transaction destination root (DEESPEC_TX_DEST_ROOT)
	DisableRecovery() bool // Disable startup recovery (DEESPEC_DISABLE_RECOVERY)

	// Metrics and audit
	DisableMetricsRotation() bool // Disable metrics rotation (DEESPEC_DISABLE_METRICS_ROTATION)
	FsyncAudit() bool             // Enable fsync audit logging (DEESPEC_FSYNC_AUDIT)

	// Test and debug
	TestMode() bool  // Test mode (DEESPEC_TEST_MODE)
	TestQuiet() bool // Suppress test logs (DEESPEC_TEST_QUIET)

	// Paths and logging
	Workflow() string    // Workflow definition path (DEESPEC_WORKFLOW)
	PolicyPath() string  // Policy file path (DEESPEC_POLICY_PATH)
	StderrLevel() string // Stderr log level (DEESPEC_STDERR_LEVEL)

	// Metadata
	ConfigSource() string // Source of configuration: "json", "env", or "default"
	SettingPath() string  // Path to setting.json if loaded from file
}

// AppConfig is the concrete implementation of Config interface.
// It holds all configuration values loaded from various sources.
type AppConfig struct {
	home       string
	agentBin   string
	timeoutSec int

	projectName string
	language    string
	turn        string
	taskID      string

	validate    bool
	autoFB      bool
	strictFsync bool

	maxAttempts int
	maxTurns    int

	txDestRoot      string
	disableRecovery bool

	disableMetricsRotation bool
	fsyncAudit             bool

	testMode  bool
	testQuiet bool

	workflow    string
	policyPath  string
	stderrLevel string

	configSource string
	settingPath  string
}

// Home returns the base directory for DeeSpec
func (c *AppConfig) Home() string {
	return c.home
}

// AgentBin returns the agent binary path
func (c *AppConfig) AgentBin() string {
	return c.agentBin
}

// TimeoutSec returns the timeout in seconds
func (c *AppConfig) TimeoutSec() int {
	return c.timeoutSec
}

// Timeout returns the timeout as a Duration
func (c *AppConfig) Timeout() time.Duration {
	return time.Duration(c.timeoutSec) * time.Second
}

// ProjectName returns the project name
func (c *AppConfig) ProjectName() string {
	return c.projectName
}

// Language returns the language setting
func (c *AppConfig) Language() string {
	return c.language
}

// Turn returns the current turn number
func (c *AppConfig) Turn() string {
	return c.turn
}

// TaskID returns the task ID
func (c *AppConfig) TaskID() string {
	return c.taskID
}

// Validate returns whether journal validation is enabled
func (c *AppConfig) Validate() bool {
	return c.validate
}

// AutoFB returns whether auto feedback is enabled
func (c *AppConfig) AutoFB() bool {
	return c.autoFB
}

// StrictFsync returns whether fsync failures should be treated as errors
func (c *AppConfig) StrictFsync() bool {
	return c.strictFsync
}

// MaxAttempts returns the maximum attempts before force termination
func (c *AppConfig) MaxAttempts() int {
	return c.maxAttempts
}

// MaxTurns returns the maximum turns allowed for execution
func (c *AppConfig) MaxTurns() int {
	return c.maxTurns
}

// TxDestRoot returns the transaction destination root
func (c *AppConfig) TxDestRoot() string {
	return c.txDestRoot
}

// DisableRecovery returns whether startup recovery is disabled
func (c *AppConfig) DisableRecovery() bool {
	return c.disableRecovery
}

// DisableMetricsRotation returns whether metrics rotation is disabled
func (c *AppConfig) DisableMetricsRotation() bool {
	return c.disableMetricsRotation
}

// FsyncAudit returns whether fsync audit logging is enabled
func (c *AppConfig) FsyncAudit() bool {
	return c.fsyncAudit
}

// TestMode returns whether test mode is enabled
func (c *AppConfig) TestMode() bool {
	return c.testMode
}

// TestQuiet returns whether test logs should be suppressed
func (c *AppConfig) TestQuiet() bool {
	return c.testQuiet
}

// Workflow returns the workflow definition path
func (c *AppConfig) Workflow() string {
	return c.workflow
}

// PolicyPath returns the policy file path
func (c *AppConfig) PolicyPath() string {
	return c.policyPath
}

// StderrLevel returns the stderr log level
func (c *AppConfig) StderrLevel() string {
	return c.stderrLevel
}

// ConfigSource returns the source of configuration
func (c *AppConfig) ConfigSource() string {
	return c.configSource
}

// SettingPath returns the path to setting.json if loaded from file
func (c *AppConfig) SettingPath() string {
	return c.settingPath
}

// NewAppConfig creates a new AppConfig with the given values.
// This is typically called by the infrastructure layer after loading and merging configurations.
func NewAppConfig(
	home, agentBin string, timeoutSec int,
	projectName, language, turn, taskID string,
	validate, autoFB, strictFsync bool,
	maxAttempts, maxTurns int,
	txDestRoot string, disableRecovery bool,
	disableMetricsRotation, fsyncAudit bool,
	testMode, testQuiet bool,
	workflow, policyPath, stderrLevel string,
	configSource, settingPath string,
) *AppConfig {
	return &AppConfig{
		home:                   home,
		agentBin:               agentBin,
		timeoutSec:             timeoutSec,
		projectName:            projectName,
		language:               language,
		turn:                   turn,
		taskID:                 taskID,
		validate:               validate,
		autoFB:                 autoFB,
		strictFsync:            strictFsync,
		maxAttempts:            maxAttempts,
		maxTurns:               maxTurns,
		txDestRoot:             txDestRoot,
		disableRecovery:        disableRecovery,
		disableMetricsRotation: disableMetricsRotation,
		fsyncAudit:             fsyncAudit,
		testMode:               testMode,
		testQuiet:              testQuiet,
		workflow:               workflow,
		policyPath:             policyPath,
		stderrLevel:            stderrLevel,
		configSource:           configSource,
		settingPath:            settingPath,
	}
}
