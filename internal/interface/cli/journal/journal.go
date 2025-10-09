package journal

import (
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
)

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/YoshitsuguKoike/deespec/internal/validator/journal"
	"github.com/spf13/cobra"
)

// NewCommand creates the journal command
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "journal",
		Short: "Journal validation commands",
		RunE:  func(c *cobra.Command, _ []string) error { return c.Help() },
	}
	cmd.AddCommand(newJournalVerifyCmd())
	return cmd
}

func newJournalVerifyCmd() *cobra.Command {
	var filePath string
	var format string

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify journal NDJSON schema",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJournalVerify(filePath, format)
		},
	}

	cmd.Flags().StringVar(&filePath, "path", ".deespec/var/journal.ndjson", "Path to journal NDJSON file")
	cmd.Flags().StringVar(&format, "format", "", "Output format (json for CI integration)")
	return cmd
}

func runJournalVerify(filePath, format string) error {
	// Check if file exists
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			if format == "json" {
				// Return empty result for JSON format
				result := &journal.ValidationResult{
					Version:     1,
					GeneratedAt: "",
					File:        filePath,
					Lines:       []journal.LineResult{},
					Summary:     journal.Summary{Lines: 0, OK: 0, Warn: 0, Error: 0},
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			} else {
				fmt.Printf("WARN: journal file not found: %s (skipping validation)\n", filePath)
				return nil
			}
		}
		return fmt.Errorf("error opening journal file: %w", err)
	}
	defer file.Close()

	// Create validator and validate
	validator := journal.NewValidator(filePath)
	result, err := validator.ValidateFile(file)
	if err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	if format == "json" {
		// JSON output
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			return err
		}
	} else {
		// Text output
		printTextResult(result)
	}

	// Exit with appropriate code
	if result.Summary.Error > 0 {
		os.Exit(1)
	}
	return nil
}

func printTextResult(result *journal.ValidationResult) {
	for _, lineResult := range result.Lines {
		for _, issue := range lineResult.Issues {
			switch issue.Type {
			case "error":
				if issue.Field != "" {
					common.Error("journal line=%d field=%s %s\n",
						lineResult.Line, issue.Field, issue.Message)
				} else {
					common.Error("journal line=%d %s\n",
						lineResult.Line, issue.Message)
				}
			case "warn":
				if issue.Field != "" {
					fmt.Printf("WARN: journal line=%d field=%s %s\n",
						lineResult.Line, issue.Field, issue.Message)
				} else {
					fmt.Printf("WARN: journal line=%d %s\n",
						lineResult.Line, issue.Message)
				}
			case "ok":
				fmt.Printf("OK: journal line=%d %s\n", lineResult.Line, issue.Message)
			}
		}

		// If no issues, it's OK
		if len(lineResult.Issues) == 0 {
			fmt.Printf("OK: journal line=%d valid\n", lineResult.Line)
		}
	}

	// Print summary
	fmt.Printf("SUMMARY: lines=%d ok=%d warn=%d error=%d\n",
		result.Summary.Lines, result.Summary.OK, result.Summary.Warn, result.Summary.Error)
}
