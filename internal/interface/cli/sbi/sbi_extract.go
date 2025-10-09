package sbi

import (
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
)

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// Improvement represents an extracted improvement suggestion
type Improvement struct {
	Title       string
	Description string
	Priority    int
	Labels      []string
	SourceSBI   string
}

// NewSBIExtractCommand creates the sbi extract command
func NewSBIExtractCommand() *cobra.Command {
	var dryRun bool
	var autoRegister bool

	cmd := &cobra.Command{
		Use:   "extract-improvements [SBI-ID]",
		Short: "Extract improvement suggestions from done files",
		Long: `Extract improvement suggestions from completed SBI done files
and optionally register them as new SBI tasks.

Examples:
  # Extract improvements from a specific SBI
  deespec sbi extract-improvements SBI-01K6P86GWPD8A78X1DEHQ7FAWH

  # Extract and automatically register as new SBIs
  deespec sbi extract-improvements SBI-01K6P86GWPD8A78X1DEHQ7FAWH --auto-register

  # Dry run to see what would be extracted
  deespec sbi extract-improvements SBI-01K6P86GWPD8A78X1DEHQ7FAWH --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sbiID := args[0]
			return extractImprovements(sbiID, dryRun, autoRegister)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be extracted without creating files")
	cmd.Flags().BoolVar(&autoRegister, "auto-register", false, "Automatically register extracted improvements as new SBIs")

	return cmd
}

func extractImprovements(sbiID string, dryRun, autoRegister bool) error {
	// Find done files in the SBI directory
	sbiPath := filepath.Join(".deespec", "specs", "sbi", sbiID)
	if _, err := os.Stat(sbiPath); os.IsNotExist(err) {
		return fmt.Errorf("SBI directory not found: %s", sbiPath)
	}

	// Look for done_*.md files
	doneFiles, err := filepath.Glob(filepath.Join(sbiPath, "done_*.md"))
	if err != nil {
		return fmt.Errorf("failed to find done files: %w", err)
	}

	if len(doneFiles) == 0 {
		fmt.Printf("No done files found for SBI %s\n", sbiID)
		return nil
	}

	var allImprovements []Improvement

	for _, doneFile := range doneFiles {
		improvements, err := parseImprovementsFromFile(doneFile, sbiID)
		if err != nil {
			common.Warn("Failed to parse %s: %v", doneFile, err)
			continue
		}
		allImprovements = append(allImprovements, improvements...)
	}

	if len(allImprovements) == 0 {
		fmt.Printf("No improvement suggestions found in SBI %s\n", sbiID)
		return nil
	}

	// Display found improvements
	fmt.Printf("Found %d improvement suggestion(s) in SBI %s:\n\n", len(allImprovements), sbiID)
	for i, imp := range allImprovements {
		fmt.Printf("%d. %s\n", i+1, imp.Title)
		if imp.Description != "" {
			fmt.Printf("   %s\n", strings.ReplaceAll(imp.Description, "\n", "\n   "))
		}
		fmt.Println()
	}

	if dryRun {
		fmt.Println("(Dry run - no files created)")
		return nil
	}

	if autoRegister {
		// Register each improvement as a new SBI
		for _, imp := range allImprovements {
			if err := registerImprovementAsSBI(imp); err != nil {
				common.Warn("Failed to register '%s': %v", imp.Title, err)
			} else {
				common.Info("Registered new SBI: %s", imp.Title)
			}
		}
	} else {
		// Save to a file for manual review
		outputPath := filepath.Join(sbiPath, "extracted_improvements.md")
		if err := saveImprovements(outputPath, allImprovements); err != nil {
			return fmt.Errorf("failed to save improvements: %w", err)
		}
		fmt.Printf("\nImprovements saved to: %s\n", outputPath)
		fmt.Println("Use --auto-register flag to automatically create new SBIs")
	}

	return nil
}

func parseImprovementsFromFile(filePath string, sbiID string) ([]Improvement, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var improvements []Improvement
	scanner := bufio.NewScanner(file)
	inRecommendations := false
	var currentLines []string

	// Patterns to identify recommendation sections
	recStartPattern := regexp.MustCompile(`(?i)(Recommendations?|改善推奨事項|推奨事項):`)
	bulletPattern := regexp.MustCompile(`^[-•*]\s+(.+)`)
	numberedPattern := regexp.MustCompile(`^\d+\.\s+(.+)`)

	for scanner.Scan() {
		line := scanner.Text()

		// Check if we're entering recommendations section
		if recStartPattern.MatchString(line) {
			inRecommendations = true
			continue
		}

		// Stop at next major section
		if inRecommendations && strings.HasPrefix(line, "##") {
			inRecommendations = false
		}

		if inRecommendations {
			// Extract bullet points or numbered items
			if match := bulletPattern.FindStringSubmatch(line); match != nil {
				currentLines = append(currentLines, match[1])
			} else if match := numberedPattern.FindStringSubmatch(line); match != nil {
				currentLines = append(currentLines, match[1])
			}
		}
	}

	// Convert collected lines to improvements
	for _, line := range currentLines {
		// Skip non-actionable items
		if strings.Contains(strings.ToLower(line), "非クリティカル") ||
			strings.Contains(strings.ToLower(line), "non-critical") {
			continue
		}

		imp := Improvement{
			Title:       extractTitle(line),
			Description: line,
			SourceSBI:   sbiID,
			Priority:    3, // Default priority
			Labels:      extractLabels(line),
		}
		improvements = append(improvements, imp)
	}

	return improvements, nil
}

func extractTitle(line string) string {
	// Remove common prefixes and clean up
	title := line
	title = regexp.MustCompile(`(?i)^(⚠️|❌|✅|TODO:|FIXME:)\s*`).ReplaceAllString(title, "")

	// Truncate to reasonable length
	if len(title) > 80 {
		title = title[:77] + "..."
	}

	return title
}

func extractLabels(line string) []string {
	labels := []string{"improvement"}

	lower := strings.ToLower(line)
	if strings.Contains(lower, "test") {
		labels = append(labels, "testing")
	}
	if strings.Contains(lower, "security") || strings.Contains(lower, "セキュリティ") {
		labels = append(labels, "security")
	}
	if strings.Contains(lower, "performance") || strings.Contains(lower, "パフォーマンス") {
		labels = append(labels, "performance")
	}

	return labels
}

func registerImprovementAsSBI(imp Improvement) error {
	// TODO: Implement actual SBI registration
	// This would call the existing register logic or use RegisterSBIInput
	fmt.Printf("[Mock] Would register: %s\n", imp.Title)
	return nil
}

func saveImprovements(outputPath string, improvements []Improvement) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintf(file, "# Extracted Improvement Suggestions\n\n")
	fmt.Fprintf(file, "Source SBI: %s\n", improvements[0].SourceSBI)
	fmt.Fprintf(file, "Total: %d suggestions\n\n", len(improvements))

	for i, imp := range improvements {
		fmt.Fprintf(file, "## %d. %s\n\n", i+1, imp.Title)
		if imp.Description != "" {
			fmt.Fprintf(file, "%s\n\n", imp.Description)
		}
		if len(imp.Labels) > 0 {
			fmt.Fprintf(file, "**Labels:** %s\n\n", strings.Join(imp.Labels, ", "))
		}
		fmt.Fprintf(file, "---\n\n")
	}

	return nil
}
