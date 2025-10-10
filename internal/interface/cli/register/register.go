package register

import (
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
)

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/application/service"
	"github.com/YoshitsuguKoike/deespec/internal/application/usecase"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/transaction"
	"github.com/spf13/cobra"
)

// RegisterResult represents the JSON output for registration (CLI layer)
type RegisterResult struct {
	OK       bool     `json:"ok"`
	ID       string   `json:"id"`
	SpecPath string   `json:"spec_path"`
	Warnings []string `json:"warnings"`
	Error    string   `json:"error,omitempty"`
}

// NewRegisterCommand creates the register command
func NewRegisterCommand() *cobra.Command {
	var stdinFlag bool
	var fileFlag string
	var onCollision string
	var printEffectiveConfig bool
	var format string
	var compact bool
	var redactSecrets bool
	var dryRun bool
	var stderrLevel string

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register a new SBI specification",
		Long:  "Register a new SBI specification from stdin or file input",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Apply stderr level immediately if specified
			if stderrLevel != "" {
				os.Setenv("DEESPEC_STDERR_LEVEL", stderrLevel)
			}

			// Handle print-effective-config first (no side effects)
			if printEffectiveConfig {
				return runPrintEffectiveConfig(onCollision, format, compact, redactSecrets)
			}

			// Handle dry-run mode
			if dryRun {
				return runDryRun(stdinFlag, fileFlag, onCollision, format, compact)
			}

			return runRegisterWithFlags(cmd, args, stdinFlag, fileFlag, onCollision, stderrLevel)
		},
	}

	cmd.Flags().BoolVar(&stdinFlag, "stdin", false, "Read input from stdin")
	cmd.Flags().StringVar(&fileFlag, "file", "", "Read input from file")
	cmd.Flags().StringVar(&onCollision, "on-collision", "error", "How to handle path collisions (error|suffix|replace)")
	cmd.Flags().BoolVar(&printEffectiveConfig, "print-effective-config", false, "Print the effective configuration and exit")
	cmd.Flags().StringVar(&format, "format", "json", "Output format for effective config (json|yaml)")
	cmd.Flags().BoolVar(&compact, "compact", false, "Use compact format (single line JSON)")
	cmd.Flags().BoolVar(&redactSecrets, "redact-secrets", true, "Redact sensitive values in output")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Simulate registration without side effects")
	cmd.Flags().StringVar(&stderrLevel, "stderr-level", "", "Set log level (off|error|warn|info|debug)")

	return cmd
}

var exitFunc = os.Exit

func runRegisterWithFlags(cmd *cobra.Command, args []string, stdinFlag bool, fileFlag string, onCollision string, stderrLevel string) error {
	// Initialize stderr logger
	stderrLog := log.New(os.Stderr, "", 0)

	// Check exclusive flags
	if stdinFlag && fileFlag != "" {
		result := RegisterResult{
			OK:       false,
			Warnings: []string{},
			Error:    "cannot specify both --stdin and --file",
		}
		stderrLog.Println("ERROR: cannot specify both --stdin and --file")
		printJSONLine(result)
		exitFunc(1)
		return nil
	}

	if !stdinFlag && fileFlag == "" {
		result := RegisterResult{
			OK:       false,
			Warnings: []string{},
			Error:    "must specify either --stdin or --file",
		}
		stderrLog.Println("ERROR: must specify either --stdin or --file")
		printJSONLine(result)
		exitFunc(1)
		return nil
	}

	// Initialize DI container to get DB connection
	container, err := common.InitializeContainer()
	if err != nil {
		result := RegisterResult{
			OK:       false,
			Warnings: []string{},
			Error:    fmt.Sprintf("failed to initialize container: %v", err),
		}
		stderrLog.Printf("ERROR: %v\n", err)
		printJSONLine(result)
		exitFunc(1)
		return nil
	}
	defer container.Close()

	// Get DB connection from container
	db := container.GetDB()

	// Create use case
	validationService := service.NewRegisterValidationService()
	transactionService := transaction.NewRegisterTransactionService("", "", db, common.Warn)
	journalPath := filepath.Join(".deespec", "journal.jsonl")
	registerUseCase := usecase.NewRegisterSBIUseCase(
		validationService,
		transactionService,
		journalPath,
		common.Warn,
	)

	// Enable test mode if needed
	if isTestMode {
		registerUseCase.SetTestMode(true)
	}

	// Build input DTO
	input := &dto.RegisterSBIInput{
		UseStdin:    stdinFlag,
		FilePath:    fileFlag,
		OnCollision: onCollision,
		StderrLevel: stderrLevel,
		DryRun:      false,
	}

	// Execute use case
	ctx := context.Background()
	output, err := registerUseCase.Execute(ctx, input)
	if err != nil {
		result := RegisterResult{
			OK:       false,
			Warnings: []string{},
			Error:    fmt.Sprintf("execution failed: %v", err),
		}
		stderrLog.Printf("ERROR: %v\n", err)
		printJSONLine(result)
		exitFunc(1)
		return nil
	}

	// Convert output DTO to CLI result
	warnings := output.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	result := RegisterResult{
		OK:       output.OK,
		ID:       output.ID,
		SpecPath: output.SpecPath,
		Warnings: warnings,
		Error:    output.Error,
	}

	// Log based on result
	if !result.OK {
		if stderrLevel == "" || stderrLevel != "off" {
			stderrLog.Printf("ERROR: %s\n", result.Error)
		}
		printJSONLine(result)
		exitFunc(1)
		return nil
	}

	// Log warnings
	for _, warning := range result.Warnings {
		if stderrLevel == "" || (stderrLevel != "off" && stderrLevel != "error") {
			stderrLog.Printf("WARN: %s\n", warning)
		}
	}

	// Log success
	if stderrLevel == "" || (stderrLevel != "off" && stderrLevel != "error" && stderrLevel != "warn") {
		stderrLog.Printf("INFO: spec_path resolved: %s\n", result.SpecPath)
		stderrLog.Printf("INFO: registration completed with transaction\n")
	}

	printJSONLine(result)
	return nil
}

func printJSONLine(result RegisterResult) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.Encode(result)
}
