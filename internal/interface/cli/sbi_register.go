package cli

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	infraSbi "github.com/YoshitsuguKoike/deespec/internal/infra/repository/sbi"
	usecaseSbi "github.com/YoshitsuguKoike/deespec/internal/usecase/sbi"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// sbiRegisterFlags holds the flags for sbi register command
type sbiRegisterFlags struct {
	title   string
	body    string
	jsonOut bool
	dryRun  bool
	quiet   bool
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
	cmd.Flags().BoolVar(&flags.jsonOut, "json", false, "Output result in JSON format")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false, "Simulate registration without creating files")
	cmd.Flags().BoolVar(&flags.quiet, "quiet", false, "Suppress non-error output")

	// Mark required flags
	cmd.MarkFlagRequired("title")

	return cmd
}

// runSBIRegister executes the sbi register command
func runSBIRegister(ctx context.Context, flags *sbiRegisterFlags) error {
	// Validate title
	if flags.title == "" {
		return fmt.Errorf("title is required")
	}

	// Get body content
	body := flags.body
	if body == "" && !isInputFromTerminal() {
		// Read from stdin if body not provided and stdin is available
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
		body = string(data)
	}

	// Create the use case input
	input := usecaseSbi.RegisterSBIInput{
		Title: flags.title,
		Body:  body,
	}

	// For dry-run, we need to simulate the ID and path
	if flags.dryRun {
		return handleDryRun(input, flags)
	}

	// Set up dependencies
	fs := afero.NewOsFs()
	repo := infraSbi.NewFileSBIRepository(fs)
	useCase := &usecaseSbi.RegisterSBIUseCase{
		Repo: repo,
		Now:  time.Now,
		Rand: rand.Reader,
	}

	// Execute the use case
	output, err := useCase.Execute(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to register SBI: %w", err)
	}

	// Output the result
	if flags.jsonOut {
		return outputJSON(output, true)
	}

	if !flags.quiet {
		fmt.Printf("Successfully registered SBI\n")
		fmt.Printf("ID: %s\n", output.ID)
		fmt.Printf("Spec path: %s\n", output.SpecPath)
	}

	return nil
}

// handleDryRun simulates the registration without creating files
func handleDryRun(input usecaseSbi.RegisterSBIInput, flags *sbiRegisterFlags) error {
	// Create a mock use case with in-memory filesystem
	fs := afero.NewMemMapFs()
	repo := infraSbi.NewFileSBIRepository(fs)
	useCase := &usecaseSbi.RegisterSBIUseCase{
		Repo: repo,
		Now:  time.Now,
		Rand: rand.Reader,
	}

	// Execute with in-memory filesystem
	output, err := useCase.Execute(context.Background(), input)
	if err != nil {
		return fmt.Errorf("dry-run failed: %w", err)
	}

	// Output the result
	if flags.jsonOut {
		return outputJSON(output, false)
	}

	if !flags.quiet {
		fmt.Printf("[DRY RUN] Would register SBI\n")
		fmt.Printf("ID: %s\n", output.ID)
		fmt.Printf("Spec path: %s\n", output.SpecPath)
	}

	return nil
}

// outputJSON outputs the result in JSON format
func outputJSON(output *usecaseSbi.RegisterSBIOutput, created bool) error {
	result := map[string]interface{}{
		"ok":        true,
		"id":        output.ID,
		"spec_path": output.SpecPath,
		"created":   created,
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
