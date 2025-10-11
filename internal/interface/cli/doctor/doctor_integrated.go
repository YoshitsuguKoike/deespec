package doctor

import (
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
)

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/YoshitsuguKoike/deespec/internal/app"
	"github.com/YoshitsuguKoike/deespec/internal/validator/integrated"
	"github.com/spf13/cobra"
)

// NewCommand creates the doctor command
func NewCommand() *cobra.Command {
	var format string
	var jsonOutput bool
	var repairJournal bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check environment & configuration (integrated validation)",
		Long:  "Performs comprehensive validation of all deespec components",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Handle repair-journal flag first
			if repairJournal {
				return runRepairJournal()
			}

			// Support legacy --json flag for config info
			if jsonOutput {
				return runDoctorJSON()
			}

			paths := app.GetPathsWithConfig(common.GetGlobalConfig())

			// Configure validation paths
			config := &integrated.DoctorConfig{
				BasePath:    paths.Home,
				StatePath:   paths.State,
				HealthPath:  paths.Health,
				JournalPath: paths.Journal,
			}

			// Run integrated validation
			report, err := integrated.RunIntegratedValidation(config)
			if err != nil && report == nil {
				return fmt.Errorf("validation failed: %w", err)
			}

			// Validate summary consistency
			if err := integrated.ValidateSummaryConsistency(report); err != nil {
				common.Warn("Summary consistency check failed: %v", err)
			}

			// Output results
			if format == "json" {
				// JSON output mode
				encoder := json.NewEncoder(os.Stdout)
				encoder.SetIndent("", "  ")
				if err := encoder.Encode(report); err != nil {
					return fmt.Errorf("failed to encode JSON: %w", err)
				}
			} else {
				// Text output mode
				outputTextReport(report, &paths)
			}

			// Set exit code based on errors
			if report.Summary.Error > 0 {
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", "", "Output format (json for CI integration)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output config info in JSON format (legacy)")
	cmd.Flags().BoolVar(&repairJournal, "repair-journal", false, "Repair corrupted journal.ndjson file")

	return cmd
}

func outputTextReport(report *integrated.IntegratedReport, paths *app.Paths) {
	// Output individual component results
	for name, component := range report.Components {
		if component == nil {
			continue
		}

		fmt.Printf("\n=== %s ===\n", name)

		for _, file := range component.Files {
			hasError := false
			hasWarn := false

			for _, issue := range file.Issues {
				switch issue.Type {
				case "error":
					hasError = true
					if issue.Field != "" {
						common.Error("%s%s %s\n", file.File, issue.Field, issue.Message)
					} else {
						common.Error("%s %s\n", file.File, issue.Message)
					}
				case "warn":
					hasWarn = true
					if issue.Field != "" {
						fmt.Printf("WARN: %s%s %s\n", file.File, issue.Field, issue.Message)
					} else {
						fmt.Printf("WARN: %s %s\n", file.File, issue.Message)
					}
				case "ok":
					// Only show OK if no errors or warnings
					if !hasError && !hasWarn {
						fmt.Printf("OK: %s %s\n", file.File, issue.Message)
					}
				}
			}

			// If file has no issues, show as OK
			if len(file.Issues) == 0 {
				fmt.Printf("OK: %s valid\n", file.File)
			}
		}
	}

	// Output integrated summary
	status := integrated.GetComponentStatus(report)
	fmt.Printf("\n=== INTEGRATED SUMMARY ===\n")
	fmt.Printf("SUMMARY: state=%s health=%s journal=%s total_error=%d\n",
		status.State,
		status.Health,
		status.Journal,
		report.Summary.Error)

	// Additional details
	fmt.Printf("Details: components=%d ok=%d warn=%d error=%d\n",
		report.Summary.Components,
		report.Summary.OK,
		report.Summary.Warn,
		report.Summary.Error)
}
