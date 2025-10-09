package usecase

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"golang.org/x/text/unicode/norm"
	"gopkg.in/yaml.v3"
)

// Constants
const (
	MaxIDLength    = 64
	MaxTitleLength = 200
	MaxLabelCount  = 32
	MaxInputSize   = 64 * 1024
	MaxPathBytes   = 240
	MaxSlugLength  = 60
)

// Windows reserved names
var windowsReservedNames = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true,
	"com5": true, "com6": true, "com7": true, "com8": true,
	"com9": true, "lpt1": true, "lpt2": true, "lpt3": true,
	"lpt4": true, "lpt5": true, "lpt6": true, "lpt7": true,
	"lpt8": true, "lpt9": true,
}

// loadPolicy loads the registration policy from file
func (u *RegisterSBIUseCase) loadPolicy() (*RegisterPolicy, error) {
	policyPath := u.getPolicyPath()
	file, err := os.Open(policyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Policy file is optional
		}
		return nil, fmt.Errorf("failed to open policy file: %w", err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)

	var policy RegisterPolicy
	if err := decoder.Decode(&policy); err != nil {
		return nil, fmt.Errorf("failed to parse policy file: %w", err)
	}

	if err := u.validatePolicy(&policy); err != nil {
		return nil, fmt.Errorf("invalid policy: %w", err)
	}

	return &policy, nil
}

// getPolicyPath returns the path to the policy file
func (u *RegisterSBIUseCase) getPolicyPath() string {
	return filepath.Join(".deespec", "register_policy.yml")
}

// validatePolicy validates policy configuration
func (u *RegisterSBIUseCase) validatePolicy(p *RegisterPolicy) error {
	if p.ID.Pattern != "" {
		if _, err := regexp.Compile(p.ID.Pattern); err != nil {
			return fmt.Errorf("invalid ID pattern: %w", err)
		}
	}
	if p.ID.MaxLen > 0 && p.ID.MaxLen > 256 {
		return fmt.Errorf("ID max_len exceeds safe limit (256)")
	}
	if p.Title.MaxLen > 0 && p.Title.MaxLen > 1024 {
		return fmt.Errorf("title max_len exceeds safe limit (1024)")
	}
	if p.Labels.MaxCount > 0 && p.Labels.MaxCount > 128 {
		return fmt.Errorf("labels max_count exceeds safe limit (128)")
	}
	if p.Input.MaxKB > 0 && p.Input.MaxKB > 1024 {
		return fmt.Errorf("input max_kb exceeds safe limit (1024)")
	}
	if p.Path.MaxBytes > 0 && p.Path.MaxBytes > 512 {
		return fmt.Errorf("path max_bytes exceeds safe limit (512)")
	}
	if p.Collision.SuffixLimit > 0 && p.Collision.SuffixLimit > 1000 {
		return fmt.Errorf("collision suffix_limit exceeds safe limit (1000)")
	}
	return nil
}

// resolveConfig resolves the final configuration from CLI flags and policy
func (u *RegisterSBIUseCase) resolveConfig(onCollision, stderrLevel string, policy *RegisterPolicy) (*ResolvedConfig, error) {
	config := &ResolvedConfig{
		// Defaults
		CollisionMode:             "error",
		IDMaxLen:                  MaxIDLength,
		TitleMaxLen:               MaxTitleLength,
		LabelMaxCount:             MaxLabelCount,
		InputMaxBytes:             MaxInputSize,
		PathMaxBytes:              MaxPathBytes,
		PathBaseDir:               ".deespec/specs/sbi",
		DenySymlinks:              true,
		EnforceContainment:        true,
		SlugNFKC:                  true,
		SlugLowercase:             true,
		SlugAllow:                 "a-z0-9-",
		SlugMaxRunes:              MaxSlugLength,
		SlugFallback:              "untitled",
		SlugWindowsReservedSuffix: "_spec",
		SlugTrimTrailingDotSpace:  true,
		CollisionSuffixLimit:      100,
		StderrLevel:               "info",
		JournalRecordSource:       true,
		JournalRecordInputBytes:   true,
	}

	// Apply policy
	if policy != nil {
		if policy.ID.MaxLen > 0 {
			config.IDMaxLen = policy.ID.MaxLen
		}
		if policy.Title.MaxLen > 0 {
			config.TitleMaxLen = policy.Title.MaxLen
		}
		if policy.Labels.MaxCount > 0 {
			config.LabelMaxCount = policy.Labels.MaxCount
		}
		if policy.Input.MaxKB > 0 {
			config.InputMaxBytes = policy.Input.MaxKB * 1024
		}
		if policy.Path.BaseDir != "" {
			config.PathBaseDir = policy.Path.BaseDir
		}
		if policy.Path.MaxBytes > 0 {
			config.PathMaxBytes = policy.Path.MaxBytes
		}
		config.DenySymlinks = policy.Path.DenySymlinkComponents
		config.EnforceContainment = policy.Path.EnforceContainment

		// Slug configuration
		if policy.Slug.Allow != "" {
			config.SlugAllow = policy.Slug.Allow
		}
		if policy.Slug.MaxRunes > 0 {
			config.SlugMaxRunes = policy.Slug.MaxRunes
		}
		if policy.Slug.Fallback != "" {
			config.SlugFallback = policy.Slug.Fallback
		}
		if policy.Slug.WindowsReservedSuffix != "" {
			config.SlugWindowsReservedSuffix = policy.Slug.WindowsReservedSuffix
		}
		config.SlugNFKC = policy.Slug.NFKC
		config.SlugLowercase = policy.Slug.Lowercase
		config.SlugTrimTrailingDotSpace = policy.Slug.TrimTrailingDotSpace

		// Collision configuration
		if policy.Collision.DefaultMode != "" {
			config.CollisionMode = policy.Collision.DefaultMode
		}
		if policy.Collision.SuffixLimit > 0 {
			config.CollisionSuffixLimit = policy.Collision.SuffixLimit
		}

		// Logging configuration
		if policy.Logging.StderrLevelDefault != "" {
			config.StderrLevel = policy.Logging.StderrLevelDefault
		}

		// Journal configuration
		config.JournalRecordSource = policy.Journal.RecordSource
		config.JournalRecordInputBytes = policy.Journal.RecordInputBytes
	}

	// CLI flags override policy
	if onCollision != "" {
		if onCollision != "error" && onCollision != "suffix" && onCollision != "replace" {
			return nil, fmt.Errorf("invalid collision mode: %s", onCollision)
		}
		config.CollisionMode = onCollision
	}
	if stderrLevel != "" {
		validLevels := map[string]bool{"off": true, "error": true, "warn": true, "info": true, "debug": true}
		if !validLevels[stderrLevel] {
			return nil, fmt.Errorf("invalid stderr level: %s", stderrLevel)
		}
		config.StderrLevel = stderrLevel
	}

	return config, nil
}

// readInput reads input from stdin or file
func (u *RegisterSBIUseCase) readInput(input *dto.RegisterSBIInput, config *ResolvedConfig) ([]byte, error) {
	// If InputData is pre-loaded (for testing), use it
	if len(input.InputData) > 0 {
		config.InputBytes = len(input.InputData)
		return input.InputData, nil
	}

	var data []byte
	var err error

	if input.UseStdin {
		config.InputSource = "stdin"
		maxSize := config.InputMaxBytes
		data, err = io.ReadAll(io.LimitReader(os.Stdin, int64(maxSize)+1))
	} else {
		config.InputSource = "file"
		// Check for dangerous paths (skip in test mode)
		if !u.isTestMode && (strings.Contains(input.FilePath, "..") ||
			(filepath.IsAbs(input.FilePath) && !strings.Contains(input.FilePath, "/tmp/"))) {
			return nil, fmt.Errorf("invalid file path: paths with '..' or absolute paths outside /tmp are not allowed")
		}

		file, err := os.Open(input.FilePath)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		maxSize := config.InputMaxBytes
		data, err = io.ReadAll(io.LimitReader(file, int64(maxSize)+1))
	}

	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("empty input")
	}

	config.InputBytes = len(data)

	if len(data) > config.InputMaxBytes {
		return nil, fmt.Errorf("input size exceeds limit of %d bytes", config.InputMaxBytes)
	}

	return data, nil
}

// decodeSpec decodes the input into a RegisterSpec
func (u *RegisterSBIUseCase) decodeSpec(input []byte, filePath string) (*dto.RegisterSpec, error) {
	var spec dto.RegisterSpec

	// Determine format
	isJSON := false
	if filePath != "" {
		ext := strings.ToLower(filepath.Ext(filePath))
		isJSON = ext == ".json"
	} else {
		trimmed := bytes.TrimSpace(input)
		if len(trimmed) > 0 && trimmed[0] == '{' {
			isJSON = true
		}
	}

	if isJSON {
		decoder := json.NewDecoder(bytes.NewReader(input))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&spec); err != nil {
			return nil, err
		}
	} else {
		decoder := yaml.NewDecoder(bytes.NewReader(input))
		decoder.KnownFields(true)
		if err := decoder.Decode(&spec); err != nil {
			return nil, err
		}
	}

	return &spec, nil
}

// validateSpec validates the specification
func (u *RegisterSBIUseCase) validateSpec(spec *dto.RegisterSpec, config *ResolvedConfig) dto.ValidationResult {
	// Use the validation service
	// The validation service handles all validation rules including ID, title, and label checks
	return u.validationService.ValidateSpec(spec)
}

// buildSpecPath builds the specification path
func (u *RegisterSBIUseCase) buildSpecPath(id, title string, config *ResolvedConfig) (string, error) {
	// Slugify title
	slug := u.slugifyTitle(title, config)

	// Build path using specpath package
	baseName := id + "_" + slug
	path := filepath.Join(config.PathBaseDir, baseName)

	// Validate path length
	if len(path) > config.PathMaxBytes {
		return "", fmt.Errorf("spec path exceeds maximum length of %d bytes", config.PathMaxBytes)
	}

	// Check path safety
	if !u.isPathSafe(config.PathBaseDir, path) {
		return "", fmt.Errorf("spec path would escape base directory")
	}

	// Check for symlinks if configured
	if config.DenySymlinks {
		if err := u.checkForSymlinks(path); err != nil {
			return "", err
		}
	}

	return path, nil
}

// slugifyTitle converts a title to a safe slug
func (u *RegisterSBIUseCase) slugifyTitle(title string, config *ResolvedConfig) string {
	slug := title

	// Apply NFKC normalization
	if config.SlugNFKC {
		slug = norm.NFKC.String(slug)
	}

	// Apply lowercase
	if config.SlugLowercase {
		slug = strings.ToLower(slug)
	}

	// Replace non-allowed characters with hyphens
	allowPattern := fmt.Sprintf("[^%s]", config.SlugAllow)
	re := regexp.MustCompile(allowPattern)
	slug = re.ReplaceAllString(slug, "-")

	// Collapse multiple hyphens
	slug = regexp.MustCompile("-+").ReplaceAllString(slug, "-")

	// Trim hyphens
	slug = strings.Trim(slug, "-")

	// Trim trailing dots and spaces if configured
	if config.SlugTrimTrailingDotSpace {
		slug = strings.TrimRight(slug, ". ")
	}

	// Truncate to max runes
	if config.SlugMaxRunes > 0 {
		runes := []rune(slug)
		if len(runes) > config.SlugMaxRunes {
			slug = string(runes[:config.SlugMaxRunes])
		}
	}

	// Use fallback if empty
	if slug == "" {
		slug = config.SlugFallback
	}

	// Handle Windows reserved names
	if u.isWindowsReserved(slug) {
		slug = slug + config.SlugWindowsReservedSuffix
	}

	return slug
}

// isWindowsReserved checks if a name is Windows reserved
func (u *RegisterSBIUseCase) isWindowsReserved(name string) bool {
	lower := strings.ToLower(name)
	base := strings.Split(lower, ".")[0]
	return windowsReservedNames[base]
}

// isPathSafe checks if a path is safe (within base directory)
func (u *RegisterSBIUseCase) isPathSafe(base, path string) bool {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absBase, absPath)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

// checkForSymlinks checks if any component in the path is a symlink
func (u *RegisterSBIUseCase) checkForSymlinks(path string) error {
	parts := strings.Split(filepath.Clean(path), string(os.PathSeparator))
	current := ""

	for _, part := range parts {
		if part == "" {
			continue
		}
		if current == "" {
			current = part
		} else {
			current = filepath.Join(current, part)
		}

		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("failed to check path component %s: %w", current, err)
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("path contains symlink at %s", current)
		}
	}

	return nil
}

// resolveCollision resolves path collisions
func (u *RegisterSBIUseCase) resolveCollision(path string, config *ResolvedConfig) (string, string, error) {
	// Check if path already exists
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return path, "", nil // No collision
	}
	if err != nil {
		return "", "", fmt.Errorf("failed to check path: %w", err)
	}

	// Handle collision based on mode
	switch config.CollisionMode {
	case "error":
		return "", "", fmt.Errorf("path already exists: %s", path)

	case "replace":
		warning := fmt.Sprintf("replacing existing spec at %s", path)
		return path, warning, nil

	case "suffix":
		// Find available suffix
		for i := 1; i <= config.CollisionSuffixLimit; i++ {
			ext := filepath.Ext(path)
			base := strings.TrimSuffix(path, ext)
			newPath := fmt.Sprintf("%s-%d%s", base, i, ext)

			_, err := os.Stat(newPath)
			if os.IsNotExist(err) {
				warning := fmt.Sprintf("path collision resolved with suffix: %s", newPath)
				return newPath, warning, nil
			}
		}
		return "", "", fmt.Errorf("could not resolve collision: exceeded suffix limit of %d", config.CollisionSuffixLimit)

	default:
		return "", "", fmt.Errorf("invalid collision mode: %s", config.CollisionMode)
	}
}

// getNextTurnNumber gets the next journal turn number
func (u *RegisterSBIUseCase) getNextTurnNumber() int {
	if u.journalPath == "" {
		u.journalPath = filepath.Join(".deespec", "journal.jsonl")
	}

	data, err := os.ReadFile(u.journalPath)
	if err != nil {
		return 1
	}

	lines := bytes.Split(data, []byte("\n"))
	maxTurn := 0

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if turn, ok := entry["turn"].(float64); ok {
			if int(turn) > maxTurn {
				maxTurn = int(turn)
			}
		}
	}

	return maxTurn + 1
}

// buildJournalEntry builds a journal entry for the registration
func (u *RegisterSBIUseCase) buildJournalEntry(
	spec *dto.RegisterSpec,
	output *dto.RegisterSBIOutput,
	config *ResolvedConfig,
	turn int,
) map[string]interface{} {
	entry := map[string]interface{}{
		"ts":       time.Now().UTC().Format(time.RFC3339Nano),
		"turn":     turn,
		"step":     "register",
		"decision": "register_sbi",
		"artifacts": []map[string]interface{}{
			{
				"type":      "sbi_registered",
				"id":        spec.ID,
				"title":     spec.Title,
				"spec_path": output.SpecPath,
			},
		},
	}

	if config.JournalRecordSource {
		entry["input_source"] = config.InputSource
	}
	if config.JournalRecordInputBytes {
		entry["input_bytes"] = config.InputBytes
	}

	return entry
}
