package parser

import (
	"bufio"
	"errors"
	"strings"
)

// ParseMarkdown parses a Markdown file and extracts the title from the first H1
func ParseMarkdown(content string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Check if this is an H1 (# Title)
		if strings.HasPrefix(line, "# ") {
			title := strings.TrimPrefix(line, "# ")
			title = strings.TrimSpace(title)

			if title == "" {
				return "", errors.New("H1 title is empty")
			}

			return title, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", errors.New("no H1 title found in Markdown")
}

// ExtractTitle is an alias for ParseMarkdown for clarity
func ExtractTitle(content string) (string, error) {
	return ParseMarkdown(content)
}
