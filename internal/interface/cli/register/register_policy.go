package register

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// RegisterPolicy represents the configuration policy for the register command
type RegisterPolicy struct {
	ID struct {
		Pattern string `yaml:"pattern"`
		MaxLen  int    `yaml:"max_len"`
	} `yaml:"id"`
	Title struct {
		MaxLen    int  `yaml:"max_len"`
		DenyEmpty bool `yaml:"deny_empty"`
	} `yaml:"title"`
	Labels struct {
		Pattern          string `yaml:"pattern"`
		MaxCount         int    `yaml:"max_count"`
		WarnOnDuplicates bool   `yaml:"warn_on_duplicates"`
	} `yaml:"labels"`
	Input struct {
		MaxKB int `yaml:"max_kb"`
	} `yaml:"input"`
	Slug struct {
		NFKC                  bool   `yaml:"nfkc"`
		Lowercase             bool   `yaml:"lowercase"`
		Allow                 string `yaml:"allow"`
		MaxRunes              int    `yaml:"max_runes"`
		Fallback              string `yaml:"fallback"`
		WindowsReservedSuffix string `yaml:"windows_reserved_suffix"`
		TrimTrailingDotSpace  bool   `yaml:"trim_trailing_dot_space"`
	} `yaml:"slug"`
	Path struct {
		BaseDir               string `yaml:"base_dir"`
		MaxBytes              int    `yaml:"max_bytes"`
		DenySymlinkComponents bool   `yaml:"deny_symlink_components"`
		EnforceContainment    bool   `yaml:"enforce_containment"`
	} `yaml:"path"`
	Collision struct {
		DefaultMode string `yaml:"default_mode"` // error|suffix|replace
		SuffixLimit int    `yaml:"suffix_limit"`
	} `yaml:"collision"`
	Journal struct {
		RecordSource     bool `yaml:"record_source"`
		RecordInputBytes bool `yaml:"record_input_bytes"`
	} `yaml:"journal"`
	Logging struct {
		StderrLevelDefault string `yaml:"stderr_level_default"` // info|warn|error
	} `yaml:"logging"`
}

// LoadRegisterPolicy loads the register policy from the specified path
func LoadRegisterPolicy(path string) (*RegisterPolicy, error) {
	// Check if file exists
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Policy file is optional
		}
		return nil, fmt.Errorf("failed to open policy file: %w", err)
	}
	defer file.Close()

	// Parse YAML with strict mode
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true) // Reject unknown fields

	var policy RegisterPolicy
	if err := decoder.Decode(&policy); err != nil {
		return nil, fmt.Errorf("failed to parse policy file: %w", err)
	}

	// Validate policy values
	if err := validatePolicy(&policy); err != nil {
		return nil, fmt.Errorf("invalid policy: %w", err)
	}

	return &policy, nil
}

// validatePolicy checks if policy values are within safe limits
func validatePolicy(p *RegisterPolicy) error {
	// Validate ID pattern if specified
	if p.ID.Pattern != "" {
		if _, err := regexp.Compile(p.ID.Pattern); err != nil {
			return fmt.Errorf("invalid ID pattern: %w", err)
		}
	}

	// Validate max lengths
	if p.ID.MaxLen > 0 && p.ID.MaxLen > 256 {
		return fmt.Errorf("ID max_len exceeds safe limit (256)")
	}
	if p.Title.MaxLen > 0 && p.Title.MaxLen > 1024 {
		return fmt.Errorf("Title max_len exceeds safe limit (1024)")
	}

	// Validate labels pattern if specified
	if p.Labels.Pattern != "" {
		if _, err := regexp.Compile(p.Labels.Pattern); err != nil {
			return fmt.Errorf("invalid labels pattern: %w", err)
		}
	}

	// Validate input size
	if p.Input.MaxKB > 0 && p.Input.MaxKB > 1024 { // 1MB max
		return fmt.Errorf("Input max_kb exceeds safe limit (1024)")
	}

	// Validate slug settings
	if p.Slug.MaxRunes > 0 && p.Slug.MaxRunes > 255 {
		return fmt.Errorf("Slug max_runes exceeds safe limit (255)")
	}

	// Validate path settings
	if p.Path.MaxBytes > 0 && p.Path.MaxBytes > 4096 {
		return fmt.Errorf("Path max_bytes exceeds safe limit (4096)")
	}

	// Validate collision mode
	if p.Collision.DefaultMode != "" {
		validModes := map[string]bool{"error": true, "suffix": true, "replace": true}
		if !validModes[p.Collision.DefaultMode] {
			return fmt.Errorf("invalid collision default_mode: %s (must be error|suffix|replace)", p.Collision.DefaultMode)
		}
	}

	// Validate suffix limit
	if p.Collision.SuffixLimit > 0 && p.Collision.SuffixLimit > 999 {
		return fmt.Errorf("collision suffix_limit exceeds safe limit (999)")
	}

	// Validate logging level
	if p.Logging.StderrLevelDefault != "" {
		validLevels := map[string]bool{"info": true, "warn": true, "error": true}
		if !validLevels[strings.ToLower(p.Logging.StderrLevelDefault)] {
			return fmt.Errorf("invalid stderr_level_default: %s (must be info|warn|error)", p.Logging.StderrLevelDefault)
		}
	}

	return nil
}

// GetDefaultPolicy returns the default policy configuration
func GetDefaultPolicy() *RegisterPolicy {
	policy := &RegisterPolicy{}

	// ID defaults
	policy.ID.Pattern = "^[A-Z0-9-]{1,64}$"
	policy.ID.MaxLen = 64

	// Title defaults
	policy.Title.MaxLen = 200
	policy.Title.DenyEmpty = true

	// Labels defaults
	policy.Labels.Pattern = "^[a-z0-9-]+$"
	policy.Labels.MaxCount = 32
	policy.Labels.WarnOnDuplicates = true

	// Input defaults
	policy.Input.MaxKB = 64

	// Slug defaults
	policy.Slug.NFKC = true
	policy.Slug.Lowercase = true
	policy.Slug.Allow = "a-z0-9-"
	policy.Slug.MaxRunes = 60
	policy.Slug.Fallback = "spec"
	policy.Slug.WindowsReservedSuffix = "-x"
	policy.Slug.TrimTrailingDotSpace = true

	// Path defaults
	policy.Path.BaseDir = ".deespec/specs/sbi"
	policy.Path.MaxBytes = 240
	policy.Path.DenySymlinkComponents = true
	policy.Path.EnforceContainment = true

	// Collision defaults
	policy.Collision.DefaultMode = "error"
	policy.Collision.SuffixLimit = 99

	// Journal defaults
	policy.Journal.RecordSource = false
	policy.Journal.RecordInputBytes = false

	// Logging defaults
	policy.Logging.StderrLevelDefault = "warn"

	return policy
}

// ResolvedConfig represents the final configuration after applying precedence
type ResolvedConfig struct {
	// ID validation
	IDPattern *regexp.Regexp
	IDMaxLen  int

	// Title validation
	TitleMaxLen    int
	TitleDenyEmpty bool

	// Labels validation
	LabelsPattern          *regexp.Regexp
	LabelsMaxCount         int
	LabelsWarnOnDuplicates bool

	// Input limits
	InputMaxBytes int

	// Slug settings
	SlugNFKC                  bool
	SlugLowercase             bool
	SlugAllow                 string
	SlugMaxRunes              int
	SlugFallback              string
	SlugWindowsReservedSuffix string
	SlugTrimTrailingDotSpace  bool

	// Path settings
	PathBaseDir               string
	PathMaxBytes              int
	PathDenySymlinkComponents bool
	PathEnforceContainment    bool

	// Collision handling
	CollisionMode string
	SuffixLimit   int

	// Journal settings
	JournalRecordSource     bool
	JournalRecordInputBytes bool

	// Logging settings
	StderrLevel string

	// Track source for debugging
	InputSource string // "stdin" or "file"
	InputBytes  int
}

// ResolveRegisterConfig applies precedence: CLI > Policy > Default
func ResolveRegisterConfig(cliCollisionMode string, policy *RegisterPolicy) (*ResolvedConfig, error) {
	// Start with defaults if no policy
	if policy == nil {
		policy = GetDefaultPolicy()
	}

	config := &ResolvedConfig{}

	// ID configuration
	if policy.ID.Pattern != "" {
		pattern, err := regexp.Compile(policy.ID.Pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid ID pattern in policy: %w", err)
		}
		config.IDPattern = pattern
	}
	config.IDMaxLen = policy.ID.MaxLen

	// Title configuration
	config.TitleMaxLen = policy.Title.MaxLen
	config.TitleDenyEmpty = policy.Title.DenyEmpty

	// Labels configuration
	if policy.Labels.Pattern != "" {
		pattern, err := regexp.Compile(policy.Labels.Pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid labels pattern in policy: %w", err)
		}
		config.LabelsPattern = pattern
	}
	config.LabelsMaxCount = policy.Labels.MaxCount
	config.LabelsWarnOnDuplicates = policy.Labels.WarnOnDuplicates

	// Input configuration
	config.InputMaxBytes = policy.Input.MaxKB * 1024

	// Slug configuration
	config.SlugNFKC = policy.Slug.NFKC
	config.SlugLowercase = policy.Slug.Lowercase
	config.SlugAllow = policy.Slug.Allow
	config.SlugMaxRunes = policy.Slug.MaxRunes
	config.SlugFallback = policy.Slug.Fallback
	config.SlugWindowsReservedSuffix = policy.Slug.WindowsReservedSuffix
	config.SlugTrimTrailingDotSpace = policy.Slug.TrimTrailingDotSpace

	// Path configuration
	config.PathBaseDir = policy.Path.BaseDir
	config.PathMaxBytes = policy.Path.MaxBytes
	config.PathDenySymlinkComponents = policy.Path.DenySymlinkComponents
	config.PathEnforceContainment = policy.Path.EnforceContainment

	// Collision configuration - CLI overrides policy
	if cliCollisionMode != "" {
		config.CollisionMode = cliCollisionMode
	} else {
		config.CollisionMode = policy.Collision.DefaultMode
	}
	config.SuffixLimit = policy.Collision.SuffixLimit

	// Journal configuration
	config.JournalRecordSource = policy.Journal.RecordSource
	config.JournalRecordInputBytes = policy.Journal.RecordInputBytes

	// Logging configuration
	config.StderrLevel = strings.ToLower(policy.Logging.StderrLevelDefault)

	// Ensure base directory is always set (critical for path resolution)
	if strings.TrimSpace(config.PathBaseDir) == "" {
		config.PathBaseDir = ".deespec/specs/sbi"
	}

	return config, nil
}

// ShouldLog determines if a message should be logged based on level
func (c *ResolvedConfig) ShouldLog(level string) bool {
	levelMap := map[string]int{
		"info":  0,
		"warn":  1,
		"error": 2,
	}

	configLevel, ok := levelMap[c.StderrLevel]
	if !ok {
		configLevel = 0 // Default to info
	}

	messageLevel, ok := levelMap[strings.ToLower(level)]
	if !ok {
		return true // Unknown level, log it
	}

	return messageLevel >= configLevel
}

// GetPolicyPath returns the default policy file path
var GetPolicyPath = func() string {
	return filepath.Join(".deespec", "etc", "policies", "register_policy.yaml")
}
