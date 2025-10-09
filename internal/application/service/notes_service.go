package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// NotesService handles note management logic
type NotesService struct {
	notesRepo repository.NotesRepository
}

// NewNotesService creates a new notes service
func NewNotesService(notesRepo repository.NotesRepository) *NotesService {
	return &NotesService{
		notesRepo: notesRepo,
	}
}

// AppendNote appends a new note section to the rolling note file
func (s *NotesService) AppendNote(ctx context.Context, input *dto.NoteInput) error {
	if input.SBIID == "" {
		return fmt.Errorf("SBI ID is required for note storage")
	}

	// Validate kind
	if input.Kind != "implement" && input.Kind != "review" {
		return fmt.Errorf("unknown note kind: %s", input.Kind)
	}

	// Normalize decision
	decision := input.Decision
	if input.Kind == "implement" {
		// For implement, decision is always PENDING
		decision = "PENDING"
	} else {
		// For review, normalize decision
		decision = s.normalizeDecision(decision)
	}

	// Extract summary from body
	summary := s.extractSummary(input.Body)

	// Build note header
	header := fmt.Sprintf("## Turn %d — %s\n", input.Turn, input.Now.UTC().Format(time.RFC3339Nano))
	header += "- Author: aiagent\n"
	header += fmt.Sprintf("- Step: %s\n", input.Kind)
	header += fmt.Sprintf("- Decision: %s\n", decision)

	if summary != "" {
		header += fmt.Sprintf("- Summary: %s\n", summary)
	}
	header += "\n"

	// Build the complete section
	section := header + s.ensureLF(input.Body) + "\n\n---\n"

	// Delegate to repository for file operations
	return s.notesRepo.AppendNote(ctx, input.SBIID, input.Kind, section)
}

// ExtractNoteBody extracts the note content from AI output
func (s *NotesService) ExtractNoteBody(aiOutput string, kind string) string {
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

	// Otherwise return the full output
	return aiOutput
}

// normalizeDecision normalizes review decision to one of: OK, NEEDS_CHANGES, PENDING
func (s *NotesService) normalizeDecision(decision string) string {
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
func (s *NotesService) extractSummary(body string) string {
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

// ensureLF ensures the content uses LF line endings (not CRLF)
func (s *NotesService) ensureLF(content string) string {
	// Replace CRLF with LF
	content = strings.ReplaceAll(content, "\r\n", "\n")
	// Remove any standalone CR
	content = strings.ReplaceAll(content, "\r", "\n")
	return content
}
