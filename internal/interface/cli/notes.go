package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AppendNote appends a new note section to the rolling note file
// kind: "implement" or "review"
// decision: "OK", "NEEDS_CHANGES", or "PENDING" (for implement, always "PENDING")
// body: The AI-generated note content
// turn: The current turn number from state.json
// now: The timestamp for the note
func AppendNote(kind string, decision string, body string, turn int, now time.Time) error {
	// Determine file path based on kind
	path := ""
	switch kind {
	case "implement":
		path = ".deespec/var/artifacts/impl_note.md"
		// For implement, decision is always PENDING
		if decision == "" {
			decision = "PENDING"
		}
	case "review":
		path = ".deespec/var/artifacts/review_note.md"
		// For review, normalize decision to OK|NEEDS_CHANGES|PENDING
		if decision == "" {
			decision = "PENDING"
		}
		decision = normalizeDecision(decision)
	default:
		return fmt.Errorf("unknown note kind: %s", kind)
	}

	// Create the header section
	header := fmt.Sprintf("## Turn %d — %s\n", turn, now.UTC().Format(time.RFC3339Nano))
	header += "- Author: aiagent\n"
	header += fmt.Sprintf("- Step: %s\n", kind)
	header += fmt.Sprintf("- Decision: %s\n", decision)

	// Add optional summary if we can extract it
	if summary := extractSummary(body); summary != "" {
		header += fmt.Sprintf("- Summary: %s\n", summary)
	}
	header += "\n"

	// Build the complete section
	section := header + ensureLF(body) + "\n\n---\n"

	// Read existing content if file exists
	oldContent := ""
	if existingBytes, err := os.ReadFile(path); err == nil {
		oldContent = string(existingBytes)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read existing note file: %w", err)
	}

	// Ensure old content ends with LF if not empty
	if oldContent != "" && !strings.HasSuffix(oldContent, "\n") {
		oldContent += "\n"
	}

	// Combine old and new content
	newContent := oldContent + section

	// Ensure content is UTF-8 with LF line endings and ends with newline
	newContent = ensureLF(newContent)
	if !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Atomic write: write to tmp file then rename
	return atomicWrite(path, []byte(newContent))
}

// normalizeDecision normalizes review decision to one of: OK, NEEDS_CHANGES, PENDING
func normalizeDecision(decision string) string {
	upper := strings.ToUpper(strings.TrimSpace(decision))
	switch upper {
	case "OK", "APPROVED", "PASS":
		return "OK"
	case "NEEDS_CHANGES", "NEEDS CHANGES", "FAIL", "FAILED":
		return "NEEDS_CHANGES"
	case "PENDING", "":
		return "PENDING"
	default:
		// Default to NEEDS_CHANGES for unknown values
		return "NEEDS_CHANGES"
	}
}

// extractSummary attempts to extract a brief summary from the note body
func extractSummary(body string) string {
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Look for lines that appear to be summaries
		if strings.HasPrefix(trimmed, "Summary:") ||
			strings.HasPrefix(trimmed, "## Summary") ||
			strings.HasPrefix(trimmed, "# Summary") {
			// Return the rest of the line or the next non-empty line
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return strings.TrimSpace(trimmed[idx+1:])
			}
		}
		// Return first non-empty, non-header line as summary
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "-") {
			if len(trimmed) > 100 {
				return trimmed[:100] + "..."
			}
			return trimmed
		}
	}
	return ""
}

// ExtractNoteBody extracts the note content from AI output
// It looks for specific sections like "## Implementation Note" or "## Review Note"
func ExtractNoteBody(aiOutput string, kind string) string {
	// Look for specific note sections in AI output
	var sectionHeaders []string
	if kind == "implement" {
		sectionHeaders = []string{
			"## Implementation Note",
			"# Implementation Note",
			"### Implementation Note",
		}
	} else if kind == "review" {
		sectionHeaders = []string{
			"## Review Note",
			"# Review Note",
			"### Review Note",
		}
	}

	lines := strings.Split(aiOutput, "\n")
	foundSection := false
	noteLines := []string{}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check if we found the note section header
		for _, header := range sectionHeaders {
			if strings.HasPrefix(trimmed, header) {
				foundSection = true
				break
			}
		}

		// If we found the section, collect subsequent lines
		if foundSection && trimmed != "" {
			// Skip the header line itself
			isHeader := false
			for _, header := range sectionHeaders {
				if strings.HasPrefix(trimmed, header) {
					isHeader = true
					break
				}
			}
			if !isHeader {
				noteLines = append(noteLines, line)
			}
		}
	}

	// If we found a specific section, return it
	if len(noteLines) > 0 {
		return strings.Join(noteLines, "\n")
	}

	// Otherwise return the full output (AI might not include the expected headers)
	return aiOutput
}

// ensureLF ensures the content uses LF line endings (not CRLF)
func ensureLF(content string) string {
	// Replace CRLF with LF
	content = strings.ReplaceAll(content, "\r\n", "\n")
	// Remove any standalone CR
	content = strings.ReplaceAll(content, "\r", "\n")
	return content
}

// atomicWrite writes data to a file atomically by writing to a temp file and renaming
func atomicWrite(path string, data []byte) error {
	// Create temp file in the same directory as the target
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Ensure cleanup in case of error
	defer func() {
		// Remove temp file if it still exists (in case of error)
		_ = os.Remove(tmpPath)
	}()

	// Write data to temp file
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write to temp file: %w", err)
	}

	// Sync to ensure data is written to disk
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Close the temp file
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename temp file to %s: %w", path, err)
	}

	return nil
}