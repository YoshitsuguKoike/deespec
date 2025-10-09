package dto

// RegisterSBIInput represents input for SBI registration
type RegisterSBIInput struct {
	// Input source
	UseStdin  bool   // Read from stdin
	FilePath  string // Read from file (mutually exclusive with UseStdin)
	InputData []byte // Pre-loaded input data (for testing)

	// Configuration
	OnCollision string // Collision handling mode: "error", "suffix", "replace"
	StderrLevel string // Log level: "silent", "error", "warn", "info", "debug"

	// Dry-run mode (no actual registration)
	DryRun bool
}

// RegisterSBIOutput represents the result of SBI registration
type RegisterSBIOutput struct {
	OK       bool     `json:"ok"`
	ID       string   `json:"id"`
	SpecPath string   `json:"spec_path"`
	Warnings []string `json:"warnings"`
	Error    string   `json:"error,omitempty"`
	Turn     int      `json:"turn,omitempty"` // Journal turn number
}

// RegisterSpec represents the specification to register
type RegisterSpec struct {
	ID     string   `yaml:"id" json:"id"`
	Title  string   `yaml:"title" json:"title"`
	Labels []string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

// ValidationResult holds validation warnings and errors
type ValidationResult struct {
	Warnings []string
	Err      error
}

// CollisionResolutionResult represents the result of collision resolution
type CollisionResolutionResult struct {
	FinalPath string
	Warning   string
	Error     error
}
