package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// RegisterSpec represents the input specification for registration
type RegisterSpec struct {
	ID     string   `yaml:"id" json:"id"`
	Title  string   `yaml:"title" json:"title"`
	Labels []string `yaml:"labels" json:"labels"`
}

// RegisterResult represents the JSON output for registration
type RegisterResult struct {
	OK       bool     `json:"ok"`
	ID       string   `json:"id"`
	SpecPath string   `json:"spec_path"`
	Warnings []string `json:"warnings"`
}

// NewRegisterCommand creates the register command
func NewRegisterCommand() *cobra.Command {
	var stdinFlag bool
	var fileFlag string

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register a new SBI specification",
		Long:  "Register a new SBI specification from stdin or file input",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRegisterWithFlags(cmd, args, stdinFlag, fileFlag)
		},
	}

	cmd.Flags().BoolVar(&stdinFlag, "stdin", false, "Read input from stdin")
	cmd.Flags().StringVar(&fileFlag, "file", "", "Read input from file")

	return cmd
}

var exitFunc = os.Exit

func runRegisterWithFlags(cmd *cobra.Command, args []string, stdinFlag bool, fileFlag string) error {
	// Check exclusive flags
	if stdinFlag && fileFlag != "" {
		result := RegisterResult{
			OK:       false,
			ID:       "",
			SpecPath: "",
			Warnings: []string{"cannot specify both --stdin and --file"},
		}
		printJSONLine(result)
		exitFunc(1)
		return nil
	}

	if !stdinFlag && fileFlag == "" {
		result := RegisterResult{
			OK:       false,
			ID:       "",
			SpecPath: "",
			Warnings: []string{"must specify either --stdin or --file"},
		}
		printJSONLine(result)
		exitFunc(1)
		return nil
	}

	// Read input
	input, err := readInput(stdinFlag, fileFlag)
	if err != nil {
		result := RegisterResult{
			OK:       false,
			ID:       "",
			SpecPath: "",
			Warnings: []string{fmt.Sprintf("failed to read input: %v", err)},
		}
		printJSONLine(result)
		exitFunc(1)
		return nil
	}

	// Decode input
	var spec RegisterSpec
	if err := decodeStrict(input, &spec, fileFlag); err != nil {
		result := RegisterResult{
			OK:       false,
			ID:       "",
			SpecPath: "",
			Warnings: []string{fmt.Sprintf("invalid input: %v", err)},
		}
		printJSONLine(result)
		exitFunc(1)
		return nil
	}

	// Validate specification
	if err := validateSpec(&spec); err != nil {
		result := RegisterResult{
			OK:       false,
			ID:       spec.ID,
			SpecPath: "",
			Warnings: []string{fmt.Sprintf("validation failed: %v", err)},
		}
		printJSONLine(result)
		exitFunc(1)
		return nil
	}

	// Calculate spec path
	specPath := calculateSpecPath(spec.ID, spec.Title)

	// Success result
	result := RegisterResult{
		OK:       true,
		ID:       spec.ID,
		SpecPath: specPath,
		Warnings: []string{},
	}
	printJSONLine(result)

	return nil
}

// isTestMode allows overriding path validation for tests
var isTestMode = false

func readInput(useStdin bool, filePath string) ([]byte, error) {
	if useStdin {
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, err
		}
		if len(input) == 0 {
			return nil, fmt.Errorf("empty input")
		}
		return input, nil
	}

	// Check for dangerous paths (skip in test mode)
	if !isTestMode && (strings.Contains(filePath, "..") || (filepath.IsAbs(filePath) && !strings.Contains(filePath, "/tmp/"))) {
		return nil, fmt.Errorf("invalid file path: paths with '..' or absolute paths outside /tmp are not allowed")
	}

	input, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	if len(input) == 0 {
		return nil, fmt.Errorf("empty file")
	}
	return input, nil
}

func decodeStrict(input []byte, spec *RegisterSpec, filePath string) error {
	// Determine format from file extension or try both
	isJSON := false
	if filePath != "" {
		ext := strings.ToLower(filepath.Ext(filePath))
		isJSON = ext == ".json"
	} else {
		// Try to detect format by checking if it starts with {
		trimmed := bytes.TrimSpace(input)
		if len(trimmed) > 0 && trimmed[0] == '{' {
			isJSON = true
		}
	}

	if isJSON {
		// JSON decode with strict mode
		decoder := json.NewDecoder(bytes.NewReader(input))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(spec); err != nil {
			return err
		}
	} else {
		// YAML decode with strict mode
		decoder := yaml.NewDecoder(bytes.NewReader(input))
		decoder.KnownFields(true)
		if err := decoder.Decode(spec); err != nil {
			return err
		}
	}

	return nil
}

func validateSpec(spec *RegisterSpec) error {
	// Validate ID format
	idPattern := regexp.MustCompile(`^SBI-[A-Z]+-\d{3,}$`)
	if !idPattern.MatchString(spec.ID) {
		return fmt.Errorf("invalid ID format: must match ^SBI-[A-Z]+-\\d{3,}$")
	}

	// Validate Title
	if spec.Title == "" {
		return fmt.Errorf("title cannot be empty")
	}

	// Validate Labels
	if len(spec.Labels) == 0 {
		return fmt.Errorf("labels cannot be empty")
	}

	return nil
}

func calculateSpecPath(id, title string) string {
	// Slugify title: lowercase and replace spaces with hyphens
	slug := strings.ToLower(title)
	slug = strings.ReplaceAll(slug, " ", "-")
	// Remove non-alphanumeric characters except hyphens
	slug = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(slug, "")
	// Remove leading/trailing hyphens and collapse multiple hyphens
	slug = regexp.MustCompile(`^-+|-+$`).ReplaceAllString(slug, "")
	slug = regexp.MustCompile(`-+`).ReplaceAllString(slug, "-")

	return fmt.Sprintf(".deespec/specs/sbi/%s_%s", id, slug)
}

func printJSONLine(result RegisterResult) {
	output, _ := json.Marshal(result)
	os.Stdout.Write(output)
	os.Stdout.Write([]byte("\n"))
}