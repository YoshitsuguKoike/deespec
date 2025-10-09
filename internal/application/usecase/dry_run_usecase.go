package usecase

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/buildinfo"
)

// DryRunUseCase handles dry-run execution of registration
type DryRunUseCase struct {
	registerUseCase *RegisterSBIUseCase
}

// NewDryRunUseCase creates a new dry-run use case
func NewDryRunUseCase(registerUseCase *RegisterSBIUseCase) *DryRunUseCase {
	return &DryRunUseCase{
		registerUseCase: registerUseCase,
	}
}

// Execute performs a dry-run of the registration process
func (u *DryRunUseCase) Execute(ctx context.Context, input *dto.DryRunInput) (*dto.DryRunReport, error) {
	startTime := time.Now()

	// Load policy and calculate SHA256
	policyPath := filepath.Join(".deespec", "register_policy.yml")
	policyFileFound := false
	policySHA256 := ""

	if policyBytes, err := os.ReadFile(policyPath); err == nil {
		policyFileFound = true
		hash := sha256.Sum256(policyBytes)
		policySHA256 = hex.EncodeToString(hash[:])
	}

	// Prepare register input (with DryRun flag set)
	registerInput := &dto.RegisterSBIInput{
		UseStdin:    input.UseStdin,
		FilePath:    input.FilePath,
		OnCollision: input.OnCollision,
		StderrLevel: "off", // Suppress logs during dry-run
		DryRun:      true,
	}

	// Execute registration use case (it won't actually create files in dry-run mode)
	// Instead, we'll simulate the process step by step
	report, err := u.simulateRegistration(ctx, registerInput, policyFileFound, policyPath, policySHA256, startTime)
	if err != nil {
		return u.buildErrorReport(err, policyFileFound, policyPath, policySHA256, startTime), nil
	}

	return report, nil
}

// simulateRegistration simulates the registration process and builds a report
func (u *DryRunUseCase) simulateRegistration(
	ctx context.Context,
	input *dto.RegisterSBIInput,
	policyFileFound bool,
	policyPath string,
	policySHA256 string,
	startTime time.Time,
) (*dto.DryRunReport, error) {
	// Build a simulated report
	// In a complete implementation, RegisterSBIUseCase would have a DryRun flag
	// that prevents actual file writes

	// Build a placeholder report
	report := &dto.DryRunReport{
		Meta: dto.DryRunMeta{
			SchemaVersion:   1,
			TsUTC:           time.Now().UTC().Format(time.RFC3339Nano),
			Version:         buildinfo.Version,
			PolicyFileFound: policyFileFound,
			PolicyPath:      policyPath,
			PolicySHA256:    policySHA256,
			SourcePriority:  []string{"cli-flag", "policy-file", "default"},
			DryRun:          true,
		},
		Input: dto.DryRunInputInfo{
			Source: "file",
			Bytes:  0,
		},
		Validation: dto.DryRunValidation{
			OK:       true,
			Errors:   []string{},
			Warnings: []string{},
		},
		Resolution: dto.DryRunResolution{
			ID:                   "",
			Title:                "",
			Slug:                 "",
			BaseDir:              ".deespec/specs/sbi",
			SpecPath:             "",
			CollisionMode:        input.OnCollision,
			CollisionWouldHappen: false,
			CollisionResolution:  "",
			FinalPath:            "",
			PathSafe:             true,
			SymlinkSafe:          true,
			ContainedInBase:      true,
		},
		JournalPreview: dto.DryRunJournal{
			Ts:        time.Now().UTC().Format(time.RFC3339Nano),
			Turn:      1,
			Step:      "register",
			Decision:  "register_sbi",
			ElapsedMs: time.Since(startTime).Milliseconds(),
			Error:     "",
			Artifacts: []map[string]interface{}{},
		},
	}

	return report, nil
}

// buildErrorReport builds an error report for dry-run failures
func (u *DryRunUseCase) buildErrorReport(
	err error,
	policyFileFound bool,
	policyPath string,
	policySHA256 string,
	startTime time.Time,
) *dto.DryRunReport {
	return &dto.DryRunReport{
		Meta: dto.DryRunMeta{
			SchemaVersion:   1,
			TsUTC:           time.Now().UTC().Format(time.RFC3339Nano),
			Version:         buildinfo.Version,
			PolicyFileFound: policyFileFound,
			PolicyPath:      policyPath,
			PolicySHA256:    policySHA256,
			SourcePriority:  []string{"cli-flag", "policy-file", "default"},
			DryRun:          true,
		},
		Input: dto.DryRunInputInfo{
			Source: "",
			Bytes:  0,
		},
		Validation: dto.DryRunValidation{
			OK:       false,
			Errors:   []string{err.Error()},
			Warnings: []string{},
		},
		Resolution: dto.DryRunResolution{
			CollisionMode: "",
		},
		JournalPreview: dto.DryRunJournal{
			Ts:        time.Now().UTC().Format(time.RFC3339Nano),
			Turn:      0,
			Step:      "register",
			Decision:  "",
			ElapsedMs: time.Since(startTime).Milliseconds(),
			Error:     err.Error(),
			Artifacts: []map[string]interface{}{},
		},
	}
}

// FormatReport formats the report as JSON or YAML
func (u *DryRunUseCase) FormatReport(report *dto.DryRunReport, format string, compact bool) ([]byte, error) {
	switch format {
	case "json":
		return u.formatJSON(report, compact)
	case "yaml":
		return u.formatYAML(report)
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// formatJSON formats the report as JSON
func (u *DryRunUseCase) formatJSON(report *dto.DryRunReport, compact bool) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)

	if !compact {
		encoder.SetIndent("", "  ")
	}

	if err := encoder.Encode(report); err != nil {
		return nil, fmt.Errorf("failed to encode JSON: %w", err)
	}

	return buf.Bytes(), nil
}

// formatYAML formats the report as YAML
func (u *DryRunUseCase) formatYAML(report *dto.DryRunReport) ([]byte, error) {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)

	if err := encoder.Encode(report); err != nil {
		return nil, fmt.Errorf("failed to encode YAML: %w", err)
	}

	return buf.Bytes(), nil
}
