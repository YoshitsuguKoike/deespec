package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/YoshitsuguKoike/deespec/internal/validator/common"
	"github.com/YoshitsuguKoike/deespec/internal/validator/state"
)

func newStateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "state",
		Short: "State validation commands",
		RunE: func(c *cobra.Command, _ []string) error { return c.Help() },
	}
	cmd.AddCommand(newStateVerifyCmd())
	return cmd
}

func newStateVerifyCmd() *cobra.Command {
	var filePath string
	var format string

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify state.json schema",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStateVerify(filePath, format)
		},
	}

	cmd.Flags().StringVar(&filePath, "path", ".deespec/var/state.json", "Path to state.json file")
	cmd.Flags().StringVar(&format, "format", "", "Output format (json for CI integration)")
	return cmd
}

func runStateVerify(filePath, format string) error {
	// Validate the state file
	result, err := state.ValidateStateFile(filePath)
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
		printStateTextResult(result)
	}

	// Exit with appropriate code
	if result.Summary.Error > 0 {
		os.Exit(1)
	}
	return nil
}

func printStateTextResult(result *common.ValidationResult) {
	for _, fileResult := range result.Files {
		if len(fileResult.Issues) == 0 {
			fmt.Printf("OK: %s valid\n", getFileName(fileResult.File))
		} else {
			for _, issue := range fileResult.Issues {
				switch issue.Type {
				case "error":
					if issue.Field != "" {
						fmt.Fprintf(os.Stderr, "ERROR: %s %s: %s\n",
							getFileName(fileResult.File), issue.Field, issue.Message)
					} else {
						fmt.Fprintf(os.Stderr, "ERROR: %s %s\n",
							getFileName(fileResult.File), issue.Message)
					}
				case "warn":
					if issue.Field != "" {
						fmt.Printf("WARN: %s %s: %s\n",
							getFileName(fileResult.File), issue.Field, issue.Message)
					} else {
						fmt.Printf("WARN: %s %s\n",
							getFileName(fileResult.File), issue.Message)
					}
				case "ok":
					fmt.Printf("OK: %s %s\n", getFileName(fileResult.File), issue.Message)
				}
			}
		}
	}

	// Print summary
	fmt.Printf("SUMMARY: files=%d ok=%d warn=%d error=%d\n",
		result.Summary.Files, result.Summary.OK, result.Summary.Warn, result.Summary.Error)
}

// getFileName extracts the filename from a path for cleaner output
func getFileName(filePath string) string {
	if filePath == ".deespec/var/state.json" {
		return "state.json"
	}
	if filePath == ".deespec/var/health.json" {
		return "health.json"
	}
	return filePath
}