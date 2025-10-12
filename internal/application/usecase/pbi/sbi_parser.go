package pbi

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// SBISpec represents a parsed SBI specification from a Markdown file
type SBISpec struct {
	Title          string
	Body           string
	EstimatedHours float64
	ParentPBIID    string
	Sequence       int
	Labels         []string // Label names assigned to this SBI
}

// ParseSBIFile parses an SBI file and extracts all metadata
func ParseSBIFile(filePath string) (*SBISpec, error) {
	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read SBI file %s: %w", filePath, err)
	}

	contentStr := string(content)

	// Extract title
	title, err := extractTitle(contentStr)
	if err != nil {
		return nil, fmt.Errorf("failed to extract title from %s: %w", filePath, err)
	}

	// Extract body (content before metadata section)
	body := extractBody(contentStr)

	// Extract estimated hours
	estimatedHours, err := extractEstimatedHours(contentStr)
	if err != nil {
		return nil, fmt.Errorf("failed to extract estimated hours from %s: %w", filePath, err)
	}

	// Extract metadata
	metadata, err := extractMetadata(contentStr)
	if err != nil {
		return nil, fmt.Errorf("failed to extract metadata from %s: %w", filePath, err)
	}

	// Validate required metadata fields
	parentPBIID, ok := metadata["Parent PBI"]
	if !ok || parentPBIID == "" {
		return nil, fmt.Errorf("missing required metadata: Parent PBI")
	}

	sequenceStr, ok := metadata["Sequence"]
	if !ok || sequenceStr == "" {
		return nil, fmt.Errorf("missing required metadata: Sequence")
	}

	// Parse sequence to int
	sequence, err := strconv.Atoi(strings.TrimSpace(sequenceStr))
	if err != nil {
		return nil, fmt.Errorf("invalid sequence value '%s': %w", sequenceStr, err)
	}

	// Extract labels (optional)
	labels := extractLabels(metadata)

	return &SBISpec{
		Title:          title,
		Body:           body,
		EstimatedHours: estimatedHours,
		ParentPBIID:    parentPBIID,
		Sequence:       sequence,
		Labels:         labels,
	}, nil
}

// extractTitle extracts the title from the first H1 heading (# Title)
func extractTitle(content string) (string, error) {
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines
		if trimmed == "" {
			continue
		}

		// Check if this is an H1 heading
		if strings.HasPrefix(trimmed, "# ") {
			title := strings.TrimPrefix(trimmed, "# ")
			title = strings.TrimSpace(title)

			if title == "" {
				return "", fmt.Errorf("H1 title is empty")
			}

			return title, nil
		}
	}

	return "", fmt.Errorf("no H1 title found")
}

// extractBody extracts the main body content before the metadata section (---\n)
func extractBody(content string) string {
	// Split by "---" to separate body from metadata
	parts := strings.Split(content, "\n---\n")
	if len(parts) > 0 {
		// Return the first part (before metadata section)
		return strings.TrimSpace(parts[0])
	}

	// If no metadata section delimiter found, return trimmed content
	return strings.TrimSpace(content)
}

// extractEstimatedHours extracts estimated hours from the "## 推定工数" section
// Supports formats like: "3時間", "3.5時間", "3 hours", "3.5", etc.
func extractEstimatedHours(content string) (float64, error) {
	// Regular expression to match "## 推定工数" section
	// Captures the value on the next line after the heading
	re := regexp.MustCompile(`(?m)^##\s*推定工数\s*\n\s*([0-9.]+)`)
	matches := re.FindStringSubmatch(content)

	if len(matches) >= 2 {
		hoursStr := strings.TrimSpace(matches[1])
		hours, err := strconv.ParseFloat(hoursStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid estimated hours value '%s': %w", hoursStr, err)
		}
		return hours, nil
	}

	return 0, fmt.Errorf("estimated hours section not found or invalid format")
}

// extractMetadata extracts metadata fields from the metadata section after "---"
// Returns a map of metadata key-value pairs
func extractMetadata(content string) (map[string]string, error) {
	metadata := make(map[string]string)

	// Split by "---" to get the metadata section
	parts := strings.Split(content, "\n---\n")
	if len(parts) < 2 {
		return nil, fmt.Errorf("metadata section not found (missing '---' delimiter)")
	}

	// Get the metadata section (everything after the first "---")
	metadataSection := parts[1]

	// Regular expressions for metadata fields
	parentPBIRegex := regexp.MustCompile(`(?m)^Parent PBI:\s*(.+)$`)
	sequenceRegex := regexp.MustCompile(`(?m)^Sequence:\s*(\d+)$`)
	labelsRegex := regexp.MustCompile(`(?m)^Labels:\s*(.+)$`)

	// Extract Parent PBI
	if matches := parentPBIRegex.FindStringSubmatch(metadataSection); len(matches) >= 2 {
		metadata["Parent PBI"] = strings.TrimSpace(matches[1])
	}

	// Extract Sequence
	if matches := sequenceRegex.FindStringSubmatch(metadataSection); len(matches) >= 2 {
		metadata["Sequence"] = strings.TrimSpace(matches[1])
	}

	// Extract Labels (optional)
	if matches := labelsRegex.FindStringSubmatch(metadataSection); len(matches) >= 2 {
		metadata["Labels"] = strings.TrimSpace(matches[1])
	}

	// Validate that required fields were found
	if _, ok := metadata["Parent PBI"]; !ok {
		return nil, fmt.Errorf("metadata section missing 'Parent PBI' field")
	}

	if _, ok := metadata["Sequence"]; !ok {
		return nil, fmt.Errorf("metadata section missing 'Sequence' field")
	}

	return metadata, nil
}

// extractLabels parses the Labels metadata field and returns a slice of label names
// Supports formats: "security, backend", "frontend", "none", or empty
func extractLabels(metadata map[string]string) []string {
	labelsStr, ok := metadata["Labels"]
	if !ok || labelsStr == "" {
		return []string{}
	}

	// Handle "none" or similar indicators
	if strings.TrimSpace(strings.ToLower(labelsStr)) == "none" {
		return []string{}
	}

	// Split by comma and clean up each label
	labels := strings.Split(labelsStr, ",")
	result := make([]string, 0, len(labels))

	for _, label := range labels {
		trimmed := strings.TrimSpace(label)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}
