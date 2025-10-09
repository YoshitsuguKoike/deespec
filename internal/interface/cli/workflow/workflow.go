package workflow

import (
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
)

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/YoshitsuguKoike/deespec/internal/validator/workflow"
	"github.com/spf13/cobra"
)

// Deprecated: Workflow commands are not currently used in the main execution path.
// These commands are kept for compatibility but may be removed in future versions.
var workflowCmd = &cobra.Command{
	Use:   "workflow",
	Short: "Workflow-related commands (deprecated)",
	Long:  "Commands for managing and validating workflow configurations (deprecated - not used in main execution)",
}

// NewCommand creates the workflow command
func NewCommand() *cobra.Command {
	return workflowCmd
}

// Deprecated: Workflow verification is not currently used in the main execution path.
var workflowVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify workflow.yaml structure (deprecated)",
	Long:  "Validate the structure and schema of workflow.yaml files (deprecated - not used in main execution)",
	RunE:  runWorkflowVerify,
}

var (
	workflowPath   string
	workflowFormat string
)

func init() {
	workflowVerifyCmd.Flags().StringVar(&workflowPath, "path", ".deespec/etc/workflow.yaml", "Path to workflow.yaml file")
	workflowVerifyCmd.Flags().StringVar(&workflowFormat, "format", "text", "Output format (text or json)")
	workflowCmd.AddCommand(workflowVerifyCmd)
}

func runWorkflowVerify(cmd *cobra.Command, args []string) error {
	basePath := ".deespec"
	validator := workflow.NewValidator(basePath)
	result, err := validator.Validate(workflowPath)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if workflowFormat == "json" {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			return fmt.Errorf("failed to encode JSON: %w", err)
		}
	} else {
		for _, file := range result.Files {
			if len(file.Issues) == 0 {
				fmt.Printf("OK: %s valid\n", file.File)
			} else {
				for _, issue := range file.Issues {
					if issue.Type == "error" {
						common.Error("%s%s %s\n", file.File, issue.Field, issue.Message)
					} else if issue.Type == "warning" {
						fmt.Printf("WARN: %s%s %s\n", file.File, issue.Field, issue.Message)
					}
				}
			}
		}

		fmt.Printf("SUMMARY: files=%d ok=%d warn=%d error=%d\n",
			result.Summary.Files, result.Summary.OK, result.Summary.Warn, result.Summary.Error)
	}

	if result.Summary.Error > 0 {
		os.Exit(1)
	}
	return nil
}
