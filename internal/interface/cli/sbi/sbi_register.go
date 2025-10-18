package sbi

import (
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
)

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/spf13/cobra"
)

// sbiRegisterFlags holds the flags for sbi register command
type sbiRegisterFlags struct {
	title         string
	body          string
	fromFile      string   // File path to read body content from
	parentPBI     string   // Parent PBI ID to link this SBI to
	labels        string   // Comma-separated labels
	labelArray    []string // Multiple --label flags
	dependsOn     []string // SBI IDs that this SBI depends on
	onlyImplement bool     // If true, skip review cycle (implementation-only)
	jsonOut       bool
	dryRun        bool
	quiet         bool
}

// NewSBIRegisterCommand creates the sbi register command
func NewSBIRegisterCommand() *cobra.Command {
	flags := &sbiRegisterFlags{}

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register a new SBI specification",
		Long: `Register a new SBI (Specification for Business Implementation) document.

The command generates a unique SBI-ID using ULID and creates a spec.md file
with guidelines and the provided content.

Examples:
  # Register with title and body from command line
  deespec sbi register --title "User Authentication" --body "Implementation details..."

  # Register and link to a parent PBI
  deespec sbi register --title "Auth API Implementation" --parent-pbi PBI-001 --body "Details..."

  # Register with title and body from file
  deespec sbi register --title "New Feature" --from-file spec.md --parent-pbi PBI-001

  # Short form with -f flag
  deespec sbi register --title "New Feature" -f spec.md --parent-pbi PBI-001 --only-implement

  # Register with title and body from stdin
  echo "Implementation details..." | deespec sbi register --title "User Authentication"

  # Dry run to see what would be created
  deespec sbi register --title "Test Spec" --body "Content" --dry-run --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSBIRegister(cmd.Context(), flags)
		},
	}

	// Define flags
	cmd.Flags().StringVar(&flags.title, "title", "", "Title of the specification (required)")
	cmd.Flags().StringVar(&flags.body, "body", "", "Body content of the specification (reads from stdin if not provided)")
	cmd.Flags().StringVarP(&flags.fromFile, "from-file", "f", "", "File path to read body content from")
	cmd.Flags().StringVar(&flags.parentPBI, "parent-pbi", "", "Parent PBI ID to link this SBI to")
	cmd.Flags().StringVar(&flags.labels, "labels", "", "Comma-separated list of labels")
	cmd.Flags().StringSliceVar(&flags.labelArray, "label", []string{}, "Label for the specification (can be specified multiple times)")
	cmd.Flags().StringSliceVar(&flags.dependsOn, "depends-on", []string{}, "SBI IDs that must be completed before this SBI (can be specified multiple times)")
	cmd.Flags().BoolVar(&flags.onlyImplement, "only-implement", false, "Skip review cycle and go directly to DONE after implementation")
	cmd.Flags().BoolVar(&flags.jsonOut, "json", false, "Output result in JSON format")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false, "Simulate registration without creating files")
	cmd.Flags().BoolVar(&flags.quiet, "quiet", false, "Suppress non-error output")

	// Mark required flags
	cmd.MarkFlagRequired("title")

	return cmd
}

// buildSpecMarkdown constructs the full markdown content for a specification
func buildSpecMarkdown(title, body string) string {
	var sb strings.Builder

	// Fixed guideline block (preamble)
	sb.WriteString(`## ガイドライン

このドキュメントは、チームで共有される仕様書です。以下のガイドラインに従って記述してください。

### 記述ルール

1. **明確性**: 曖昧な表現を避け、具体的に記述する
2. **完全性**: 必要な情報をすべて含める
3. **一貫性**: 用語や形式を統一する
4. **追跡可能性**: 変更履歴を明確にする

### セクション構成

- 概要: 機能の目的と背景
- 詳細仕様: 具体的な要求事項
- 制約事項: 技術的・業務的制約
- 受け入れ条件: 完了の定義

---

`)

	// Title as H1
	sb.WriteString(fmt.Sprintf("# %s\n\n", title))

	// Body content
	if body != "" {
		sb.WriteString(body)
		// Ensure trailing newline
		if !strings.HasSuffix(body, "\n") {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// runSBIRegister executes the sbi register command
func runSBIRegister(ctx context.Context, flags *sbiRegisterFlags) error {
	// Validate title
	if flags.title == "" {
		return fmt.Errorf("title is required")
	}

	// Get body content (priority: --body > --from-file > stdin)
	body := flags.body

	// If --from-file is specified, read from file
	if body == "" && flags.fromFile != "" {
		data, err := os.ReadFile(flags.fromFile)
		if err != nil {
			return fmt.Errorf("failed to read from file '%s': %w", flags.fromFile, err)
		}
		body = string(data)
	}

	// If still empty and stdin is available, read from stdin
	if body == "" && !isInputFromTerminal() {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
		body = string(data)
	}

	// Process labels
	labels := processLabels(flags.labelArray, flags.labels)

	// For dry-run, simulate without creating actual SBI
	if flags.dryRun {
		sbiDTO := &dto.SBIDTO{
			TaskDTO: dto.TaskDTO{
				ID:    "SBI-DRYRUN-EXAMPLE",
				Title: flags.title,
			},
			Labels: labels,
		}
		specPath := filepath.Join(".deespec", "specs", "sbi", sbiDTO.ID, "spec.md")

		if flags.jsonOut {
			return outputJSONNew(sbiDTO, specPath, false)
		}

		if !flags.quiet {
			fmt.Printf("[DRY RUN] Would register SBI\n")
			fmt.Printf("ID: %s\n", sbiDTO.ID)
			fmt.Printf("Spec path: %s\n", specPath)
			if len(labels) > 0 {
				fmt.Printf("Labels: %v\n", labels)
			}
		}
		return nil
	}

	// Initialize DI container
	container, err := common.InitializeContainer()
	if err != nil {
		return fmt.Errorf("failed to initialize container: %w", err)
	}
	defer container.Close()

	// Get Task UseCase
	taskUseCase := container.GetTaskUseCase()

	// Prepare parent PBI ID if provided
	var parentPBIID *string
	if flags.parentPBI != "" {
		parentPBIID = &flags.parentPBI
	}

	// Create SBI request
	req := dto.CreateSBIRequest{
		Title:         flags.title,
		Description:   body,
		ParentPBIID:   parentPBIID,
		Labels:        labels,
		DependsOn:     flags.dependsOn,
		OnlyImplement: flags.onlyImplement,
	}

	// Execute the use case
	sbiDTO, err := taskUseCase.CreateSBI(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to register SBI: %w", err)
	}

	// Build spec markdown content
	specContent := buildSpecMarkdown(flags.title, body)

	// Save spec.md to .deespec/specs/sbi/<ID>/spec.md (for backward compatibility)
	specDir := filepath.Join(".deespec", "specs", "sbi", sbiDTO.ID)
	specPath := filepath.Join(specDir, "spec.md")

	// Create directory
	if err := os.MkdirAll(specDir, 0755); err != nil {
		return fmt.Errorf("failed to create spec directory: %w", err)
	}

	// Write spec.md
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		return fmt.Errorf("failed to write spec.md: %w", err)
	}

	// Output the result
	if flags.jsonOut {
		return outputJSONNew(sbiDTO, specPath, true)
	}

	if !flags.quiet {
		fmt.Printf("Successfully registered SBI\n")
		fmt.Printf("ID: %s\n", sbiDTO.ID)
		fmt.Printf("Spec path: %s\n", specPath)
		if parentPBIID != nil {
			fmt.Printf("Parent PBI: %s\n", *parentPBIID)
		}
		if len(labels) > 0 {
			fmt.Printf("Labels: %v\n", labels)
		}
	}

	return nil
}

// outputJSONNew outputs the result in JSON format using new implementation
func outputJSONNew(sbiDTO *dto.SBIDTO, specPath string, created bool) error {
	result := map[string]interface{}{
		"ok":        true,
		"id":        sbiDTO.ID,
		"spec_path": specPath,
		"created":   created,
	}

	// Add labels if present
	if len(sbiDTO.Labels) > 0 {
		result["labels"] = sbiDTO.Labels
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

// isInputFromTerminal checks if stdin is from terminal
func isInputFromTerminal() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return true
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// processLabels merges and deduplicates labels from both flag sources
func processLabels(labelArray []string, labelsStr string) []string {
	labelMap := make(map[string]bool)
	var result []string

	// Process --label flags (multiple instances)
	for _, label := range labelArray {
		label = trimSpace(label)
		if label != "" && !labelMap[label] {
			labelMap[label] = true
			result = append(result, label)
		}
	}

	// Process --labels flag (comma-separated)
	if labelsStr != "" {
		labels := splitLabels(labelsStr)
		for _, label := range labels {
			label = trimSpace(label)
			if label != "" && !labelMap[label] {
				labelMap[label] = true
				result = append(result, label)
			}
		}
	}

	return result
}

// splitLabels splits a comma-separated string into labels
func splitLabels(s string) []string {
	if s == "" {
		return []string{}
	}

	// Split by comma
	var labels []string
	for _, label := range stringsSplit(s, ",") {
		if label != "" {
			labels = append(labels, label)
		}
	}
	return labels
}

// stringsSplit is a simple string split by delimiter
func stringsSplit(s, sep string) []string {
	if s == "" {
		return []string{}
	}

	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	// Add the last segment
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

// trimSpace removes leading and trailing whitespace
func trimSpace(s string) string {
	start := 0
	end := len(s)

	// Find first non-space character
	for start < end && isSpace(s[start]) {
		start++
	}

	// Find last non-space character
	for end > start && isSpace(s[end-1]) {
		end--
	}

	return s[start:end]
}

// isSpace checks if a byte is a whitespace character
func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}
