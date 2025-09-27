package journal

// JournalEntry represents a single journal NDJSON line with exactly 7 required keys
type JournalEntry struct {
	Timestamp string        `json:"ts"`         // RFC3339Nano UTC (must end with Z)
	Turn      int           `json:"turn"`       // Turn number, >= 0
	Step      string        `json:"step"`       // plan|implement|test|review|done
	Decision  string        `json:"decision"`   // OK|NEEDS_CHANGES|PENDING
	ElapsedMS int           `json:"elapsed_ms"` // Elapsed time in milliseconds, >= 0
	Error     string        `json:"error"`      // Error message (can be empty string)
	Artifacts []interface{} `json:"artifacts"`  // Array of strings or objects
}

// ValidationIssue represents a single validation issue
type ValidationIssue struct {
	Type    string `json:"type"` // "ok", "warn", "error"
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// LineResult represents validation result for a single line
type LineResult struct {
	Line   int               `json:"line"`
	Issues []ValidationIssue `json:"issues"`
}

// ValidationResult represents the complete validation result
type ValidationResult struct {
	Version     int          `json:"version"`
	GeneratedAt string       `json:"generated_at"`
	File        string       `json:"file"`
	Lines       []LineResult `json:"lines"`
	Summary     Summary      `json:"summary"`
}

// Summary contains validation statistics
type Summary struct {
	Lines int `json:"lines"`
	OK    int `json:"ok"`
	Warn  int `json:"warn"`
	Error int `json:"error"`
}

// Validator contains validation configuration and state
type Validator struct {
	filePath     string
	previousTurn int
}

// ValidSteps defines the allowed step values
var ValidSteps = map[string]bool{
	"plan":      true,
	"implement": true,
	"test":      true,
	"review":    true,
	"done":      true,
}

// ValidDecisions defines the allowed decision values
var ValidDecisions = map[string]bool{
	"OK":            true,
	"NEEDS_CHANGES": true,
	"PENDING":       true,
}

// NewValidator creates a new journal validator
func NewValidator(filePath string) *Validator {
	return &Validator{
		filePath:     filePath,
		previousTurn: -1, // Initialize to -1 to allow first turn to be 0
	}
}
