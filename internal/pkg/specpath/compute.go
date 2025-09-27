package specpath

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// ResolvedConfig represents the configuration for spec path computation
type ResolvedConfig struct {
	PathBaseDir string

	// Slug policy (complete propagation from register policy)
	SlugNFKC                  bool   // Apply NFKC normalization
	SlugLowercase             bool   // Convert to lowercase
	SlugAllowChars            string // Allowed characters (e.g., "a-z0-9-")
	SlugMaxRunes              int    // Maximum length in runes
	SlugFallback              string // Fallback when slug is empty (e.g., "spec")
	SlugWindowsReservedSuffix string // Suffix for Windows reserved names (e.g., "-x")
	SlugTrimTrailingDotSpace  bool   // Remove trailing dots and spaces

	// Collision handling
	CollisionMode string
	SuffixLimit   int
}

// ComputeSpecPath computes the final spec path based on ID and title
// This function reuses the same logic from REG-003 for slug generation
// and path construction, ensuring consistency between tests and implementation
func ComputeSpecPath(id, title string, cfg ResolvedConfig) (string, error) {
	// Validate inputs
	if id == "" {
		return "", fmt.Errorf("ID is required")
	}
	if title == "" {
		return "", fmt.Errorf("title is required")
	}

	// Generate slug from title
	slug := SlugifyTitle(title, cfg)

	// Construct path: base_dir/ID_slug
	specName := fmt.Sprintf("%s_%s", id, slug)

	// Apply length limit (240 bytes max for filesystem compatibility)
	if len(specName) > 240 {
		// Truncate slug to fit within limit
		maxSlugLen := 240 - len(id) - 1 // -1 for underscore
		if maxSlugLen <= 0 {
			return "", fmt.Errorf("ID too long: %d bytes", len(id))
		}

		// Truncate slug preserving UTF-8 boundaries
		truncated := truncateUTF8(slug, maxSlugLen)
		specName = fmt.Sprintf("%s_%s", id, truncated)
	}

	// Resolve base directory (handle symlinks)
	baseDir := cfg.PathBaseDir
	if baseDir == "" {
		baseDir = ".deespec/specs/sbi"
	}

	// Construct final path
	finalPath := filepath.Join(baseDir, specName)

	// Clean the path to remove any redundant elements
	finalPath = filepath.Clean(finalPath)

	return finalPath, nil
}

// SlugifyTitle converts a title to a filesystem-safe slug
// Following REG-003 rules: NFKC normalization, lowercase, allowed chars only
// This is the single source of truth for slug generation
func SlugifyTitle(title string, cfg ResolvedConfig) string {
	// Apply NFKC normalization if enabled
	if cfg.SlugNFKC {
		title = norm.NFKC.String(title)
	}

	// Convert to lowercase if enabled
	if cfg.SlugLowercase {
		title = strings.ToLower(title)
	}

	// Build allowed character set
	allowed := make(map[rune]bool)
	allowStr := cfg.SlugAllowChars
	if allowStr == "" {
		allowStr = "a-z0-9-"
	}

	// Parse character ranges like "a-z0-9-"
	for i := 0; i < len(allowStr); i++ {
		if i+2 < len(allowStr) && allowStr[i+1] == '-' && i+2 < len(allowStr) && allowStr[i+2] != '-' {
			// This is a range like a-z or 0-9
			start := allowStr[i]
			end := allowStr[i+2]
			for c := start; c <= end; c++ {
				allowed[rune(c)] = true
			}
			i += 2
		} else if allowStr[i] == '-' && (i == 0 || i == len(allowStr)-1) {
			// Literal hyphen at start or end
			allowed['-'] = true
		} else if allowStr[i] != '-' {
			// Regular character
			allowed[rune(allowStr[i])] = true
		}
	}

	// Filter characters
	var result strings.Builder
	for _, r := range title {
		if allowed[r] {
			result.WriteRune(r)
		} else if r > 127 {
			// Non-ASCII characters - replace with nothing for accented latin chars
			// that decompose to ASCII equivalents after normalization
			// For é (U+00E9), we want 'e' not a hyphen
			if isAccentedLatin(r) {
				// Try to get the base character
				base := getBaseCharacter(r)
				if base != 0 && allowed[base] {
					result.WriteRune(base)
				}
			} else if result.Len() > 0 && result.String()[result.Len()-1] != '-' {
				// Other non-ASCII becomes hyphen
				result.WriteRune('-')
			}
		} else {
			// Replace disallowed ASCII chars with hyphen
			if result.Len() > 0 && result.String()[result.Len()-1] != '-' {
				result.WriteRune('-')
			}
		}
	}

	slug := result.String()

	// Collapse consecutive hyphens
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}

	// Trim hyphens from start and end
	slug = strings.Trim(slug, "-")

	// Remove trailing dots and spaces if configured
	if cfg.SlugTrimTrailingDotSpace {
		slug = strings.TrimRight(slug, ". ")
	}

	// Apply length limit
	maxLen := cfg.SlugMaxRunes
	if maxLen == 0 {
		maxLen = 60 // Default
	}

	if len([]rune(slug)) > maxLen {
		runes := []rune(slug)
		slug = string(runes[:maxLen])
		// Trim any trailing hyphen from truncation
		slug = strings.TrimRight(slug, "-")
	}

	// Handle empty slug case - use fallback
	if slug == "" {
		slug = cfg.SlugFallback
		if slug == "" {
			slug = "spec" // Ultimate fallback
		}
	}

	// Check for Windows reserved names and add suffix if configured
	if cfg.SlugWindowsReservedSuffix != "" && isWindowsReserved(slug) {
		slug = slug + cfg.SlugWindowsReservedSuffix
	}

	return slug
}

// isWindowsReserved checks if a name is a Windows reserved filename
func isWindowsReserved(name string) bool {
	reserved := map[string]bool{
		"con": true, "prn": true, "aux": true, "nul": true,
		"com1": true, "com2": true, "com3": true, "com4": true,
		"com5": true, "com6": true, "com7": true, "com8": true, "com9": true,
		"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true,
		"lpt5": true, "lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
	}
	return reserved[strings.ToLower(name)]
}

// isAccentedLatin checks if a rune is an accented Latin character
func isAccentedLatin(r rune) bool {
	// Common Latin-1 Supplement accented characters
	return (r >= 0x00C0 && r <= 0x00FF) || // À-ÿ
		(r >= 0x0100 && r <= 0x017F) // Latin Extended-A
}

// getBaseCharacter returns the base ASCII character for common accented Latin characters
func getBaseCharacter(r rune) rune {
	// Map common accented characters to their base forms
	switch r {
	case 'à', 'á', 'â', 'ã', 'ä', 'å', 'À', 'Á', 'Â', 'Ã', 'Ä', 'Å':
		return 'a'
	case 'è', 'é', 'ê', 'ë', 'È', 'É', 'Ê', 'Ë':
		return 'e'
	case 'ì', 'í', 'î', 'ï', 'Ì', 'Í', 'Î', 'Ï':
		return 'i'
	case 'ò', 'ó', 'ô', 'õ', 'ö', 'Ò', 'Ó', 'Ô', 'Õ', 'Ö':
		return 'o'
	case 'ù', 'ú', 'û', 'ü', 'Ù', 'Ú', 'Û', 'Ü':
		return 'u'
	case 'ý', 'ÿ', 'Ý', 'Ÿ':
		return 'y'
	case 'ñ', 'Ñ':
		return 'n'
	case 'ç', 'Ç':
		return 'c'
	default:
		return 0
	}
}

// truncateUTF8 truncates a string to a maximum byte length while preserving UTF-8 boundaries
func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}

	// Find the last valid UTF-8 boundary within the limit
	for i := maxBytes; i > 0; i-- {
		if utf8Valid(s[:i]) {
			result := s[:i]
			// Trim any trailing hyphen
			return strings.TrimRight(result, "-")
		}
	}

	return ""
}

// utf8Valid checks if a byte slice forms valid UTF-8
func utf8Valid(b string) bool {
	for i := 0; i < len(b); {
		r, size := utf8DecodeRune([]byte(b[i:]))
		if r == unicode.ReplacementChar && size == 1 {
			return false
		}
		i += size
	}
	return true
}

// utf8DecodeRune is a simple UTF-8 decoder
func utf8DecodeRune(b []byte) (rune, int) {
	if len(b) == 0 {
		return unicode.ReplacementChar, 0
	}

	b0 := b[0]
	if b0 < 0x80 {
		// ASCII
		return rune(b0), 1
	}

	// Multi-byte sequence
	if b0 < 0xC0 {
		return unicode.ReplacementChar, 1
	} else if b0 < 0xE0 {
		if len(b) < 2 {
			return unicode.ReplacementChar, 1
		}
		return rune(b0&0x1F)<<6 | rune(b[1]&0x3F), 2
	} else if b0 < 0xF0 {
		if len(b) < 3 {
			return unicode.ReplacementChar, 1
		}
		return rune(b0&0x0F)<<12 | rune(b[1]&0x3F)<<6 | rune(b[2]&0x3F), 3
	} else {
		if len(b) < 4 {
			return unicode.ReplacementChar, 1
		}
		return rune(b0&0x07)<<18 | rune(b[1]&0x3F)<<12 | rune(b[2]&0x3F)<<6 | rune(b[3]&0x3F), 4
	}
}
