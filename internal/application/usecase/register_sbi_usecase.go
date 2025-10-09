package usecase

import (
	"context"
	"fmt"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/application/service"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/transaction"
)

// RegisterSBIUseCase handles the registration of SBI specifications
type RegisterSBIUseCase struct {
	validationService  *service.RegisterValidationService
	transactionService *transaction.RegisterTransactionService
	warnLog            func(format string, args ...interface{})
	isTestMode         bool
	journalPath        string
}

// NewRegisterSBIUseCase creates a new register SBI use case
func NewRegisterSBIUseCase(
	validationService *service.RegisterValidationService,
	transactionService *transaction.RegisterTransactionService,
	journalPath string,
	warnLog func(format string, args ...interface{}),
) *RegisterSBIUseCase {
	if warnLog == nil {
		warnLog = func(format string, args ...interface{}) {}
	}
	return &RegisterSBIUseCase{
		validationService:  validationService,
		transactionService: transactionService,
		journalPath:        journalPath,
		warnLog:            warnLog,
		isTestMode:         false,
	}
}

// RegisterPolicy represents the configuration policy for registration
type RegisterPolicy struct {
	ID struct {
		Pattern string `yaml:"pattern"`
		MaxLen  int    `yaml:"max_len"`
	} `yaml:"id"`
	Title struct {
		MaxLen    int  `yaml:"max_len"`
		DenyEmpty bool `yaml:"deny_empty"`
	} `yaml:"title"`
	Labels struct {
		Pattern          string `yaml:"pattern"`
		MaxCount         int    `yaml:"max_count"`
		WarnOnDuplicates bool   `yaml:"warn_on_duplicates"`
	} `yaml:"labels"`
	Input struct {
		MaxKB int `yaml:"max_kb"`
	} `yaml:"input"`
	Slug struct {
		NFKC                  bool   `yaml:"nfkc"`
		Lowercase             bool   `yaml:"lowercase"`
		Allow                 string `yaml:"allow"`
		MaxRunes              int    `yaml:"max_runes"`
		Fallback              string `yaml:"fallback"`
		WindowsReservedSuffix string `yaml:"windows_reserved_suffix"`
		TrimTrailingDotSpace  bool   `yaml:"trim_trailing_dot_space"`
	} `yaml:"slug"`
	Path struct {
		BaseDir               string `yaml:"base_dir"`
		MaxBytes              int    `yaml:"max_bytes"`
		DenySymlinkComponents bool   `yaml:"deny_symlink_components"`
		EnforceContainment    bool   `yaml:"enforce_containment"`
	} `yaml:"path"`
	Collision struct {
		DefaultMode string `yaml:"default_mode"`
		SuffixLimit int    `yaml:"suffix_limit"`
	} `yaml:"collision"`
	Journal struct {
		RecordSource     bool `yaml:"record_source"`
		RecordInputBytes bool `yaml:"record_input_bytes"`
	} `yaml:"journal"`
	Logging struct {
		StderrLevelDefault string `yaml:"stderr_level_default"`
	} `yaml:"logging"`
}

// ResolvedConfig represents the final resolved configuration
type ResolvedConfig struct {
	// Collision handling
	CollisionMode string

	// Input tracking
	InputSource   string
	InputBytes    int
	InputMaxBytes int

	// Validation limits
	IDMaxLen      int
	TitleMaxLen   int
	LabelMaxCount int

	// Path configuration
	PathMaxBytes       int
	PathBaseDir        string
	DenySymlinks       bool
	EnforceContainment bool

	// Slug configuration
	SlugNFKC                  bool
	SlugLowercase             bool
	SlugAllow                 string
	SlugMaxRunes              int
	SlugFallback              string
	SlugWindowsReservedSuffix string
	SlugTrimTrailingDotSpace  bool

	// Collision limits
	CollisionSuffixLimit int

	// Logging
	StderrLevel string

	// Journal options
	JournalRecordSource     bool
	JournalRecordInputBytes bool
}

// ShouldLog determines if a message at the given level should be logged
func (c *ResolvedConfig) ShouldLog(level string) bool {
	levels := map[string]int{
		"off":   0,
		"error": 1,
		"warn":  2,
		"info":  3,
		"debug": 4,
	}

	currentLevel := levels[c.StderrLevel]
	messageLevel := levels[level]

	return messageLevel > 0 && messageLevel <= currentLevel
}

// Execute performs the SBI registration
func (u *RegisterSBIUseCase) Execute(ctx context.Context, input *dto.RegisterSBIInput) (*dto.RegisterSBIOutput, error) {
	// Load policy
	policy, err := u.loadPolicy()
	if err != nil {
		return &dto.RegisterSBIOutput{
			OK:    false,
			Error: fmt.Sprintf("failed to load policy: %v", err),
		}, nil
	}

	// Resolve configuration
	config, err := u.resolveConfig(input.OnCollision, input.StderrLevel, policy)
	if err != nil {
		return &dto.RegisterSBIOutput{
			OK:    false,
			Error: fmt.Sprintf("failed to resolve config: %v", err),
		}, nil
	}

	// Read input
	inputData, err := u.readInput(input, config)
	if err != nil {
		return &dto.RegisterSBIOutput{
			OK:    false,
			Error: fmt.Sprintf("failed to read input: %v", err),
		}, nil
	}

	// Decode spec
	spec, err := u.decodeSpec(inputData, input.FilePath)
	if err != nil {
		return &dto.RegisterSBIOutput{
			OK:    false,
			Error: fmt.Sprintf("invalid input: %v", err),
		}, nil
	}

	// Validate spec
	validationResult := u.validateSpec(spec, config)
	if validationResult.Err != nil {
		return &dto.RegisterSBIOutput{
			OK:    false,
			ID:    spec.ID,
			Error: validationResult.Err.Error(),
		}, nil
	}

	// Build spec path
	specPath, err := u.buildSpecPath(spec.ID, spec.Title, config)
	if err != nil {
		return &dto.RegisterSBIOutput{
			OK:    false,
			ID:    spec.ID,
			Error: fmt.Sprintf("failed to build spec path: %v", err),
		}, nil
	}

	// Resolve collision
	finalPath, collisionWarning, err := u.resolveCollision(specPath, config)
	if err != nil {
		return &dto.RegisterSBIOutput{
			OK:    false,
			ID:    spec.ID,
			Error: err.Error(),
		}, nil
	}

	// Add collision warning if any
	warnings := validationResult.Warnings
	if collisionWarning != "" {
		warnings = append(warnings, collisionWarning)
	}

	// Get next turn number
	turn := u.getNextTurnNumber()

	// Build output
	output := &dto.RegisterSBIOutput{
		OK:       true,
		ID:       spec.ID,
		SpecPath: finalPath,
		Warnings: warnings,
		Turn:     turn,
	}

	// Build journal entry
	journalEntry := u.buildJournalEntry(spec, output, config, turn)

	// Execute transaction
	if err := u.transactionService.ExecuteRegisterTransaction(ctx, spec, output.SpecPath, journalEntry); err != nil {
		return &dto.RegisterSBIOutput{
			OK:       false,
			ID:       spec.ID,
			SpecPath: "",
			Error:    fmt.Sprintf("transaction failed: %v", err),
		}, nil
	}

	return output, nil
}

// SetTestMode enables test mode for path validation
func (u *RegisterSBIUseCase) SetTestMode(enabled bool) {
	u.isTestMode = enabled
}
