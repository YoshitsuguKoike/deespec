package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/buildinfo"
	"gopkg.in/yaml.v3"
)

// DryRunReport represents the complete dry-run output
type DryRunReport struct {
	Meta           DryRunMeta       `json:"meta" yaml:"meta"`
	Input          DryRunInput      `json:"input" yaml:"input"`
	Validation     DryRunValidation `json:"validation" yaml:"validation"`
	Resolution     DryRunResolution `json:"resolution" yaml:"resolution"`
	JournalPreview DryRunJournal    `json:"journal_preview" yaml:"journal_preview"`
}

// DryRunMeta contains metadata about the dry-run execution
type DryRunMeta struct {
	SchemaVersion   int      `json:"schema_version" yaml:"schema_version"`
	TsUTC           string   `json:"ts_utc" yaml:"ts_utc"`
	Version         string   `json:"version" yaml:"version"`
	PolicyFileFound bool     `json:"policy_file_found" yaml:"policy_file_found"`
	PolicyPath      string   `json:"policy_path" yaml:"policy_path"`
	PolicySHA256    string   `json:"policy_sha256,omitempty" yaml:"policy_sha256,omitempty"`
	SourcePriority  []string `json:"source_priority" yaml:"source_priority"`
	DryRun          bool     `json:"dry_run" yaml:"dry_run"`
}

// DryRunInput describes the input source
type DryRunInput struct {
	Source string `json:"source" yaml:"source"`
	Bytes  int    `json:"bytes" yaml:"bytes"`
}

// DryRunValidation contains validation results
type DryRunValidation struct {
	OK       bool     `json:"ok" yaml:"ok"`
	Errors   []string `json:"errors" yaml:"errors"`
	Warnings []string `json:"warnings" yaml:"warnings"`
}

// DryRunResolution shows path resolution details
type DryRunResolution struct {
	ID                   string `json:"id" yaml:"id"`
	Title                string `json:"title" yaml:"title"`
	Slug                 string `json:"slug" yaml:"slug"`
	BaseDir              string `json:"base_dir" yaml:"base_dir"`
	SpecPath             string `json:"spec_path" yaml:"spec_path"`
	CollisionMode        string `json:"collision_mode" yaml:"collision_mode"`
	CollisionWouldHappen bool   `json:"collision_would_happen" yaml:"collision_would_happen"`
	CollisionResolution  string `json:"collision_resolution" yaml:"collision_resolution"`
	FinalPath            string `json:"final_path" yaml:"final_path"`
	PathSafe             bool   `json:"path_safe" yaml:"path_safe"`
	SymlinkSafe          bool   `json:"symlink_safe" yaml:"symlink_safe"`
	ContainedInBase      bool   `json:"contained_in_base" yaml:"contained_in_base"`
}

// DryRunJournal represents the journal entry that would be written
type DryRunJournal struct {
	Ts        string                   `json:"ts" yaml:"ts"`
	Turn      int                      `json:"turn" yaml:"turn"`
	Step      string                   `json:"step" yaml:"step"`
	Decision  string                   `json:"decision" yaml:"decision"`
	ElapsedMs int64                    `json:"elapsed_ms" yaml:"elapsed_ms"`
	Error     string                   `json:"error" yaml:"error"`
	Artifacts []map[string]interface{} `json:"artifacts" yaml:"artifacts"`
}

// runDryRun executes a dry-run of the registration process
func runDryRun(stdinFlag bool, fileFlag string, cliCollisionMode string, format string, compact bool) error {
	startTime := time.Now()

	// Initialize log buffer for early messages
	logBuf := NewLogBuffer()
	SetDefaultStderr(os.Stderr)

	// Load policy
	policyPath := GetPolicyPath()
	policy, policyErr := LoadRegisterPolicy(policyPath)

	var policySHA256 string
	policyFileFound := policyErr == nil && policy != nil

	if policyFileFound {
		// Calculate SHA256 of policy file
		if policyBytes, err := os.ReadFile(policyPath); err == nil {
			hash := sha256.Sum256(policyBytes)
			policySHA256 = hex.EncodeToString(hash[:])
		}
		logBuf.Info("policy loaded: %s", policyPath)
	} else if !os.IsNotExist(policyErr) && policyErr != nil {
		// Policy file exists but has errors - always output errors immediately
		logBuf.Error("failed to load policy: %v", policyErr)
		return policyErr
	} else {
		logBuf.Info("no policy file found, using defaults")
	}

	// Resolve configuration with precedence
	config, err := ResolveRegisterConfig(cliCollisionMode, policy)
	if err != nil {
		logBuf.Error("failed to resolve config: %v", err)
		return err
	}

	// Flush buffered logs based on resolved log level
	logLevel := ParseLogLevel(config.StderrLevel)
	logBuf.Flush(logLevel, os.Stderr)

	// Read input
	input, err := readInputWithConfig(stdinFlag, fileFlag, config)
	if err != nil {
		report := buildErrorReport(err, "failed to read input", config, policyFileFound, policyPath, policySHA256, startTime)
		outputReport(report, format, compact)
		return err
	}

	// Decode input
	var spec RegisterSpec
	if err := decodeStrict(input, &spec, fileFlag); err != nil {
		report := buildErrorReport(err, fmt.Sprintf("invalid input: %v", err), config, policyFileFound, policyPath, policySHA256, startTime)
		outputReport(report, format, compact)
		return err
	}

	// Validate specification
	validationResult := validateSpecWithConfig(&spec, config)

	// Build spec path
	specPath, pathErr := buildSafeSpecPathWithConfig(spec.ID, spec.Title, config)
	if pathErr != nil && validationResult.Err == nil {
		validationResult.Err = pathErr
	}

	// Check for collisions and resolve
	var finalPath string
	var collisionWouldHappen bool
	var collisionResolution string
	var pathSafe, symlinkSafe, containedInBase bool

	if specPath != "" && validationResult.Err == nil {
		// Check if path exists (collision detection)
		if _, err := os.Stat(specPath); err == nil {
			collisionWouldHappen = true
		}

		// Simulate collision resolution
		if collisionWouldHappen {
			switch config.CollisionMode {
			case CollisionError:
				validationResult.Err = fmt.Errorf("path already exists: %s", specPath)
				collisionResolution = "error:would-fail"
				finalPath = specPath
			case CollisionSuffix:
				// Find available suffix
				for i := 2; i <= config.SuffixLimit; i++ {
					testPath := fmt.Sprintf("%s_%d", specPath, i)
					if _, err := os.Stat(testPath); os.IsNotExist(err) {
						finalPath = testPath
						collisionResolution = fmt.Sprintf("suffix:_%d", i)
						break
					}
				}
				if finalPath == "" {
					validationResult.Err = fmt.Errorf("unable to resolve collision: all suffixes exhausted")
					collisionResolution = "suffix:exhausted"
					finalPath = specPath
				}
			case CollisionReplace:
				finalPath = specPath
				collisionResolution = "replace:would-delete"
				if config.ShouldLog("warn") {
					fmt.Fprintf(os.Stderr, "WARN: would replace existing directory: %s\n", specPath)
				}
			}
		} else {
			finalPath = specPath
			collisionResolution = ""
		}

		// Safety checks
		if finalPath != "" {
			pathSafe = isPathSafe(config.PathBaseDir, finalPath)

			// Check for symlinks only if path exists
			if err := checkForSymlinks(filepath.Dir(finalPath)); err == nil {
				symlinkSafe = true
			}

			containedInBase = pathSafe // For now, same as path safe
		}
	}

	// Build dry-run report
	report := buildDryRunReport(
		spec,
		config,
		validationResult,
		specPath,
		finalPath,
		collisionWouldHappen,
		collisionResolution,
		pathSafe,
		symlinkSafe,
		containedInBase,
		policyFileFound,
		policyPath,
		policySHA256,
		startTime,
	)

	// Output report
	outputReport(report, format, compact)

	// Return error if validation failed
	if validationResult.Err != nil {
		if config.ShouldLog("error") {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", validationResult.Err)
		}
		return validationResult.Err
	}

	// Log warnings
	for _, warning := range validationResult.Warnings {
		if config.ShouldLog("warn") {
			fmt.Fprintf(os.Stderr, "WARN: %s\n", warning)
		}
	}

	// Log success info
	if config.ShouldLog("info") {
		fmt.Fprintf(os.Stderr, "INFO: dry-run completed successfully\n")
	}

	return nil
}

// buildDryRunReport creates the complete dry-run report
func buildDryRunReport(
	spec RegisterSpec,
	config *ResolvedConfig,
	validation ValidationResult,
	specPath string,
	finalPath string,
	collisionWouldHappen bool,
	collisionResolution string,
	pathSafe bool,
	symlinkSafe bool,
	containedInBase bool,
	policyFileFound bool,
	policyPath string,
	policySHA256 string,
	startTime time.Time,
) *DryRunReport {
	now := time.Now().UTC()

	// Build meta
	meta := DryRunMeta{
		SchemaVersion:   1,
		TsUTC:           now.Format(time.RFC3339Nano),
		Version:         buildinfo.GetVersion(),
		PolicyFileFound: policyFileFound,
		PolicyPath:      "",
		PolicySHA256:    policySHA256,
		SourcePriority:  []string{"cli", "policy", "defaults"},
		DryRun:          true,
	}

	if policyFileFound {
		cleanPath, _ := filepath.Abs(policyPath)
		meta.PolicyPath = cleanPath
	}

	// Build input info
	input := DryRunInput{
		Source: config.InputSource,
		Bytes:  config.InputBytes,
	}

	// Build validation info
	validationOK := validation.Err == nil
	var errors []string
	if validation.Err != nil {
		errors = []string{validation.Err.Error()}
	}
	validationInfo := DryRunValidation{
		OK:       validationOK,
		Errors:   errors,
		Warnings: validation.Warnings,
	}

	// Build resolution info
	resolution := DryRunResolution{
		ID:                   spec.ID,
		Title:                spec.Title,
		Slug:                 slugifyTitleWithConfig(spec.Title, config),
		BaseDir:              config.PathBaseDir,
		SpecPath:             specPath,
		CollisionMode:        config.CollisionMode,
		CollisionWouldHappen: collisionWouldHappen,
		CollisionResolution:  collisionResolution,
		FinalPath:            finalPath,
		PathSafe:             pathSafe,
		SymlinkSafe:          symlinkSafe,
		ContainedInBase:      containedInBase,
	}

	// Build journal preview
	journalArtifact := map[string]interface{}{
		"type":      "register",
		"id":        spec.ID,
		"ok":        validationOK,
		"spec_path": finalPath,
	}

	// Add optional journal fields based on policy
	if config.JournalRecordSource {
		journalArtifact["source"] = config.InputSource
	}
	if config.JournalRecordInputBytes {
		journalArtifact["input_bytes"] = config.InputBytes
	}

	journalPreview := DryRunJournal{
		Ts:        now.Format(time.RFC3339Nano),
		Turn:      0, // Dry-run doesn't increment turn
		Step:      "plan",
		Decision:  "PENDING",
		ElapsedMs: time.Since(startTime).Milliseconds(),
		Error:     "",
		Artifacts: []map[string]interface{}{journalArtifact},
	}

	if validation.Err != nil {
		journalPreview.Error = validation.Err.Error()
	}

	return &DryRunReport{
		Meta:           meta,
		Input:          input,
		Validation:     validationInfo,
		Resolution:     resolution,
		JournalPreview: journalPreview,
	}
}

// buildErrorReport creates a report for early errors
func buildErrorReport(err error, message string, config *ResolvedConfig, policyFileFound bool, policyPath string, policySHA256 string, startTime time.Time) *DryRunReport {
	now := time.Now().UTC()

	meta := DryRunMeta{
		SchemaVersion:   1,
		TsUTC:           now.Format(time.RFC3339Nano),
		Version:         buildinfo.GetVersion(),
		PolicyFileFound: policyFileFound,
		PolicyPath:      "",
		PolicySHA256:    policySHA256,
		SourcePriority:  []string{"cli", "policy", "defaults"},
		DryRun:          true,
	}

	if policyFileFound {
		cleanPath, _ := filepath.Abs(policyPath)
		meta.PolicyPath = cleanPath
	}

	return &DryRunReport{
		Meta: meta,
		Input: DryRunInput{
			Source: config.InputSource,
			Bytes:  config.InputBytes,
		},
		Validation: DryRunValidation{
			OK:     false,
			Errors: []string{message},
		},
		Resolution: DryRunResolution{},
		JournalPreview: DryRunJournal{
			Ts:        now.Format(time.RFC3339Nano),
			Turn:      0,
			Step:      "plan",
			Decision:  "PENDING",
			ElapsedMs: time.Since(startTime).Milliseconds(),
			Error:     message,
			Artifacts: []map[string]interface{}{},
		},
	}
}

// outputReport formats and outputs the dry-run report
func outputReport(report *DryRunReport, format string, compact bool) error {
	var output []byte
	var err error

	switch format {
	case "yaml":
		output, err = yaml.Marshal(report)
	case "json":
		if compact {
			output, err = json.Marshal(report)
		} else {
			output, err = json.MarshalIndent(report, "", "  ")
		}
		if err == nil && !bytes.HasSuffix(output, []byte("\n")) {
			output = append(output, '\n')
		}
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	_, err = os.Stdout.Write(output)
	return err
}
