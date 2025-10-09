package health

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
	validatorCommon "github.com/YoshitsuguKoike/deespec/internal/validator/common"
	"github.com/YoshitsuguKoike/deespec/internal/validator/health"
	"github.com/spf13/cobra"
)

// NewCommand creates the health command
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Health validation commands",
		RunE:  func(c *cobra.Command, _ []string) error { return c.Help() },
	}
	cmd.AddCommand(newHealthVerifyCmd())
	return cmd
}

func newHealthVerifyCmd() *cobra.Command {
	var filePath string
	var format string

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify health.json schema",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHealthVerify(filePath, format)
		},
	}

	cmd.Flags().StringVar(&filePath, "path", ".deespec/var/health.json", "Path to health.json file")
	cmd.Flags().StringVar(&format, "format", "", "Output format (json for CI integration)")
	return cmd
}

func runHealthVerify(filePath, format string) error {
	// Validate the health file
	result, err := health.ValidateHealthFile(filePath)
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
		printHealthTextResult(result)
	}

	// Exit with appropriate code
	if result.Summary.Error > 0 {
		os.Exit(1)
	}
	return nil
}

func printHealthTextResult(result *validatorCommon.ValidationResult) {
	for _, fileResult := range result.Files {
		if len(fileResult.Issues) == 0 {
			fmt.Printf("OK: %s valid\n", common.GetFileName(fileResult.File))
		} else {
			for _, issue := range fileResult.Issues {
				switch issue.Type {
				case "error":
					if issue.Field != "" {
						common.Error("%s %s: %s\n",
							common.GetFileName(fileResult.File), issue.Field, issue.Message)
					} else {
						common.Error("%s %s\n",
							common.GetFileName(fileResult.File), issue.Message)
					}
				case "warn":
					if issue.Field != "" {
						fmt.Printf("WARN: %s %s: %s\n",
							common.GetFileName(fileResult.File), issue.Field, issue.Message)
					} else {
						fmt.Printf("WARN: %s %s\n",
							common.GetFileName(fileResult.File), issue.Message)
					}
				case "ok":
					fmt.Printf("OK: %s %s\n", common.GetFileName(fileResult.File), issue.Message)
				}
			}
		}
	}

	// Print summary
	fmt.Printf("SUMMARY: files=%d ok=%d warn=%d error=%d\n",
		result.Summary.Files, result.Summary.OK, result.Summary.Warn, result.Summary.Error)
}
