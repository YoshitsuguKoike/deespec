package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/YoshitsuguKoike/deespec/internal/app"
	"github.com/YoshitsuguKoike/deespec/internal/app/state"
)

// Allowed placeholders for prompt expansion
var Allowed = []string{"turn", "task_id", "project_name", "language"}

// allowedSet for quick lookup
var allowedSet = map[string]struct{}{
	"turn":         {},
	"task_id":      {},
	"project_name": {},
	"language":     {},
}

// Regular expression to match placeholders {name}
var rePH = regexp.MustCompile(`\{[a-zA-Z_][a-zA-Z0-9_]*\}`)

// BuildVarMap builds the variable map from various sources with priority:
// 1. Environment variables (highest priority)
// 2. Workflow vars
// 3. Runtime defaults (lowest priority)
func BuildVarMap(ctx context.Context, p app.Paths, wfVars map[string]string, st *state.State) map[string]string {
	vars := map[string]string{}

	// Default values
	// turn: from state
	if st != nil {
		vars["turn"] = strconv.Itoa(st.Turn)
		// task_id: extract from meta if available
		if st.Meta != nil {
			if taskID, ok := st.Meta["task_id"].(string); ok {
				vars["task_id"] = taskID
			} else {
				vars["task_id"] = ""
			}
		} else {
			vars["task_id"] = ""
		}
	} else {
		vars["turn"] = "0"
		vars["task_id"] = ""
	}

	// project_name: derive from working directory
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}
	projectName := filepath.Base(wd)
	if projectName == "" || projectName == "." || projectName == "/" {
		projectName = "project"
	}
	vars["project_name"] = projectName

	// language: default to "ja"
	vars["language"] = "ja"

	// Override with workflow vars if provided
	if wfVars != nil {
		if v, ok := wfVars["project_name"]; ok && v != "" {
			vars["project_name"] = v
		}
		if v, ok := wfVars["language"]; ok && v != "" {
			vars["language"] = v
		}
	}

	// Override with environment variables (highest priority)
	if v := os.Getenv("DEE_PROJECT_NAME"); v != "" {
		vars["project_name"] = v
	}
	if v := os.Getenv("DEE_LANGUAGE"); v != "" {
		vars["language"] = v
	}
	if v := os.Getenv("DEE_TURN"); v != "" {
		vars["turn"] = v
	}
	if v := os.Getenv("DEE_TASK_ID"); v != "" {
		vars["task_id"] = v
	}

	return vars
}

// ValidatePlaceholders checks for unknown placeholders and returns lists of unknown and used placeholders
func ValidatePlaceholders(text string, allowed []string) (unknown []string, used []string) {
	// Remove escaped braces from consideration
	// Replace \{ with a placeholder that won't match our regex
	cleanText := strings.ReplaceAll(text, `\{`, "\x00ESCAPED_BRACE\x00")

	matches := rePH.FindAllString(cleanText, -1)
	seenUnknown := map[string]struct{}{}
	seenUsed := map[string]struct{}{}

	for _, ph := range matches {
		// Extract placeholder name (remove { and })
		name := ph[1 : len(ph)-1]

		if _, ok := allowedSet[name]; !ok {
			// Unknown placeholder
			if _, exists := seenUnknown[name]; !exists {
				unknown = append(unknown, name)
				seenUnknown[name] = struct{}{}
			}
		} else {
			// Known placeholder
			if _, exists := seenUsed[name]; !exists {
				used = append(used, name)
				seenUsed[name] = struct{}{}
			}
		}
	}

	return unknown, used
}

// ExpandPrompt expands placeholders in the prompt text with provided variables
func ExpandPrompt(text string, vars map[string]string) (string, error) {
	// First validate placeholders
	unknown, _ := ValidatePlaceholders(text, Allowed)
	if len(unknown) > 0 {
		return "", fmt.Errorf("prompt: unknown placeholders %v (allowed: %v)", unknown, Allowed)
	}

	// Perform replacements
	out := text
	for k, v := range vars {
		placeholder := "{" + k + "}"
		out = strings.ReplaceAll(out, placeholder, v)
	}

	// Check for any remaining unresolved placeholders
	// This shouldn't happen if vars contains all allowed keys
	remainingUnknown, _ := ValidatePlaceholders(out, Allowed)
	if len(remainingUnknown) > 0 {
		return "", fmt.Errorf("prompt: unresolved placeholders after expand: %v", remainingUnknown)
	}

	// Handle escaped braces (convert \{ to {)
	out = strings.ReplaceAll(out, `\{`, "{")

	return out, nil
}
