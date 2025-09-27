package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/buildinfo"
	"gopkg.in/yaml.v3"
)

// EffectiveConfig represents the final applied configuration for serialization
type EffectiveConfig struct {
	Meta       EffectiveConfigMeta       `json:"meta" yaml:"meta"`
	ID         EffectiveConfigID         `json:"id" yaml:"id"`
	Title      EffectiveConfigTitle      `json:"title" yaml:"title"`
	Labels     EffectiveConfigLabels     `json:"labels" yaml:"labels"`
	Input      EffectiveConfigInput      `json:"input" yaml:"input"`
	Slug       EffectiveConfigSlug       `json:"slug" yaml:"slug"`
	Path       EffectiveConfigPath       `json:"path" yaml:"path"`
	Collision  EffectiveConfigCollision  `json:"collision" yaml:"collision"`
	Journal    EffectiveConfigJournal    `json:"journal" yaml:"journal"`
	Logging    EffectiveConfigLogging    `json:"logging" yaml:"logging"`
	PathInputs EffectiveConfigPathInputs `json:"path_inputs" yaml:"path_inputs"`
}

// EffectiveConfigMeta contains metadata about the configuration
type EffectiveConfigMeta struct {
	PolicyFileFound bool     `json:"policy_file_found" yaml:"policy_file_found"`
	PolicyPath      string   `json:"policy_path" yaml:"policy_path"`
	SourcePriority  []string `json:"source_priority" yaml:"source_priority"`
	Version         string   `json:"version" yaml:"version"`
	TsUTC           string   `json:"ts_utc" yaml:"ts_utc"`
}

// EffectiveConfigID represents ID validation configuration
type EffectiveConfigID struct {
	Pattern string `json:"pattern" yaml:"pattern"`
	MaxLen  int    `json:"max_len" yaml:"max_len"`
}

// EffectiveConfigTitle represents title validation configuration
type EffectiveConfigTitle struct {
	MaxLen    int  `json:"max_len" yaml:"max_len"`
	DenyEmpty bool `json:"deny_empty" yaml:"deny_empty"`
}

// EffectiveConfigLabels represents labels validation configuration
type EffectiveConfigLabels struct {
	Pattern          string `json:"pattern" yaml:"pattern"`
	MaxCount         int    `json:"max_count" yaml:"max_count"`
	WarnOnDuplicates bool   `json:"warn_on_duplicates" yaml:"warn_on_duplicates"`
}

// EffectiveConfigInput represents input size configuration
type EffectiveConfigInput struct {
	MaxKB int `json:"max_kb" yaml:"max_kb"`
}

// EffectiveConfigSlug represents slug generation configuration
type EffectiveConfigSlug struct {
	NFKC                  bool   `json:"nfkc" yaml:"nfkc"`
	Lowercase             bool   `json:"lowercase" yaml:"lowercase"`
	Allow                 string `json:"allow" yaml:"allow"`
	MaxRunes              int    `json:"max_runes" yaml:"max_runes"`
	Fallback              string `json:"fallback" yaml:"fallback"`
	WindowsReservedSuffix string `json:"windows_reserved_suffix" yaml:"windows_reserved_suffix"`
	TrimTrailingDotSpace  bool   `json:"trim_trailing_dot_space" yaml:"trim_trailing_dot_space"`
}

// EffectiveConfigPath represents path generation configuration
type EffectiveConfigPath struct {
	BaseDir               string `json:"base_dir" yaml:"base_dir"`
	MaxBytes              int    `json:"max_bytes" yaml:"max_bytes"`
	DenySymlinkComponents bool   `json:"deny_symlink_components" yaml:"deny_symlink_components"`
	EnforceContainment    bool   `json:"enforce_containment" yaml:"enforce_containment"`
}

// EffectiveConfigCollision represents collision handling configuration
type EffectiveConfigCollision struct {
	DefaultMode string `json:"default_mode" yaml:"default_mode"`
	SuffixLimit int    `json:"suffix_limit" yaml:"suffix_limit"`
}

// EffectiveConfigJournal represents journal configuration
type EffectiveConfigJournal struct {
	RecordSource     bool `json:"record_source" yaml:"record_source"`
	RecordInputBytes bool `json:"record_input_bytes" yaml:"record_input_bytes"`
}

// EffectiveConfigLogging represents logging configuration
type EffectiveConfigLogging struct {
	StderrLevelDefault string `json:"stderr_level_default" yaml:"stderr_level_default"`
}

// EffectiveConfigPathInputs represents path input validation configuration
type EffectiveConfigPathInputs struct {
	Enabled             bool     `json:"enabled" yaml:"enabled"`
	AllowedBases        []string `json:"allowed_bases" yaml:"allowed_bases"`
	DenyAbsolute        bool     `json:"deny_absolute" yaml:"deny_absolute"`
	DenyUNC             bool     `json:"deny_unc" yaml:"deny_unc"`
	DenyDriveLetter     bool     `json:"deny_drive_letter" yaml:"deny_drive_letter"`
	DenyDotDot          bool     `json:"deny_dotdot" yaml:"deny_dotdot"`
	DenyMidSymlink      bool     `json:"deny_mid_symlink" yaml:"deny_mid_symlink"`
	RequireEvalSymlinks bool     `json:"require_evalsymlinks" yaml:"require_evalsymlinks"`
	MaxBytes            int      `json:"max_bytes" yaml:"max_bytes"`
}

// runPrintEffectiveConfig prints the effective configuration and exits
func runPrintEffectiveConfig(cliCollisionMode, format string, compact, redactSecrets bool) error {
	// Load policy
	policyPath := GetPolicyPath()
	policy, err := LoadRegisterPolicy(policyPath)

	// Track if policy file was found
	policyFileFound := err == nil && policy != nil
	if err != nil && !os.IsNotExist(err) {
		// Policy file exists but has errors
		fmt.Fprintf(os.Stderr, "ERROR: failed to load policy: %v\n", err)
		return err
	}

	// Log policy load status
	if policyFileFound {
		fmt.Fprintf(os.Stderr, "INFO: policy loaded: %s\n", policyPath)
	} else {
		fmt.Fprintf(os.Stderr, "INFO: no policy file found, using defaults\n")
	}

	// Resolve configuration with precedence
	config, err := ResolveRegisterConfig(cliCollisionMode, policy)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: failed to resolve config: %v\n", err)
		return err
	}

	// Build effective config for output
	effective := buildEffectiveConfig(config, policyFileFound, policyPath)

	// Apply redaction if needed (currently no secrets to redact)
	if redactSecrets {
		// Future: redact sensitive values
	}

	// Format and output
	var output []byte
	switch format {
	case "yaml":
		output, err = yaml.Marshal(effective)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: failed to marshal to YAML: %v\n", err)
			return err
		}
	case "json":
		if compact {
			output, err = json.Marshal(effective)
		} else {
			output, err = json.MarshalIndent(effective, "", "  ")
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: failed to marshal to JSON: %v\n", err)
			return err
		}
		// Ensure newline at end
		if !bytes.HasSuffix(output, []byte("\n")) {
			output = append(output, '\n')
		}
	default:
		fmt.Fprintf(os.Stderr, "ERROR: unsupported format: %s\n", format)
		return fmt.Errorf("unsupported format: %s", format)
	}

	// Write to stdout
	os.Stdout.Write(output)
	return nil
}

// buildEffectiveConfig converts ResolvedConfig to EffectiveConfig for output
func buildEffectiveConfig(config *ResolvedConfig, policyFileFound bool, policyPath string) *EffectiveConfig {
	// Build metadata
	meta := EffectiveConfigMeta{
		PolicyFileFound: policyFileFound,
		PolicyPath:      "",
		SourcePriority:  []string{"cli", "policy", "defaults"},
		Version:         buildinfo.GetVersion(),
		TsUTC:           time.Now().UTC().Format(time.RFC3339Nano),
	}

	// Only include policy path if file was found
	if policyFileFound {
		// Clean the path for consistent output
		cleanPath, _ := filepath.Abs(policyPath)
		meta.PolicyPath = cleanPath
	}

	// Extract pattern strings
	idPattern := ""
	if config.IDPattern != nil {
		idPattern = config.IDPattern.String()
	}

	labelsPattern := ""
	if config.LabelsPattern != nil {
		labelsPattern = config.LabelsPattern.String()
	}

	// Convert bytes back to KB for output
	inputMaxKB := 0
	if config.InputMaxBytes > 0 {
		inputMaxKB = config.InputMaxBytes / 1024
	}

	// Build path_inputs configuration
	// Note: These are placeholder values for now as path_inputs isn't fully implemented
	// The spec mentions this is from REG-004 supplement
	pathInputs := EffectiveConfigPathInputs{
		Enabled:             false, // Default disabled
		AllowedBases:        []string{},
		DenyAbsolute:        true,
		DenyUNC:             true,
		DenyDriveLetter:     true,
		DenyDotDot:          true,
		DenyMidSymlink:      true,
		RequireEvalSymlinks: false,
		MaxBytes:            config.PathMaxBytes,
	}

	return &EffectiveConfig{
		Meta: meta,
		ID: EffectiveConfigID{
			Pattern: idPattern,
			MaxLen:  config.IDMaxLen,
		},
		Title: EffectiveConfigTitle{
			MaxLen:    config.TitleMaxLen,
			DenyEmpty: config.TitleDenyEmpty,
		},
		Labels: EffectiveConfigLabels{
			Pattern:          labelsPattern,
			MaxCount:         config.LabelsMaxCount,
			WarnOnDuplicates: config.LabelsWarnOnDuplicates,
		},
		Input: EffectiveConfigInput{
			MaxKB: inputMaxKB,
		},
		Slug: EffectiveConfigSlug{
			NFKC:                  config.SlugNFKC,
			Lowercase:             config.SlugLowercase,
			Allow:                 config.SlugAllow,
			MaxRunes:              config.SlugMaxRunes,
			Fallback:              config.SlugFallback,
			WindowsReservedSuffix: config.SlugWindowsReservedSuffix,
			TrimTrailingDotSpace:  config.SlugTrimTrailingDotSpace,
		},
		Path: EffectiveConfigPath{
			BaseDir:               config.PathBaseDir,
			MaxBytes:              config.PathMaxBytes,
			DenySymlinkComponents: config.PathDenySymlinkComponents,
			EnforceContainment:    config.PathEnforceContainment,
		},
		Collision: EffectiveConfigCollision{
			DefaultMode: config.CollisionMode,
			SuffixLimit: config.SuffixLimit,
		},
		Journal: EffectiveConfigJournal{
			RecordSource:     config.JournalRecordSource,
			RecordInputBytes: config.JournalRecordInputBytes,
		},
		Logging: EffectiveConfigLogging{
			StderrLevelDefault: config.StderrLevel,
		},
		PathInputs: pathInputs,
	}
}
