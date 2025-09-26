package agents

import (
	"bytes"
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// BuiltinAgents defines the default set of agents when no agents.yaml exists
var BuiltinAgents = []string{"claude_cli", "system", "gpt4", "sonnet"}

// Config represents the agents.yaml structure
type Config struct {
	Agents []string `yaml:"agents"`
}

// LoadResult contains the loaded agents and metadata
type LoadResult struct {
	Agents []string
	Source string // "file" or "builtin"
	Issues []Issue
}

// Issue represents a validation issue
type Issue struct {
	Type    string `json:"type"`    // "error" or "warn"
	Field   string `json:"field"`
	Message string `json:"message"`
}

// validIdentRegex defines valid agent name pattern
var validIdentRegex = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// LoadAgents loads agent definitions from file or returns builtin defaults
func LoadAgents(path string) (*LoadResult, error) {
	// Check if file exists
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, return builtin agents
			return &LoadResult{
				Agents: BuiltinAgents,
				Source: "builtin",
				Issues: []Issue{},
			}, nil
		}
		// Other read error
		return &LoadResult{
			Agents: nil,
			Source: "file",
			Issues: []Issue{{
				Type:    "error",
				Field:   "/",
				Message: fmt.Sprintf("cannot read agents.yaml: %v", err),
			}},
		}, nil
	}

	// Parse YAML with strict mode
	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true) // Enable strict mode to detect unknown fields

	if err := dec.Decode(&cfg); err != nil {
		return &LoadResult{
			Agents: nil,
			Source: "file",
			Issues: []Issue{{
				Type:    "error",
				Field:   "/",
				Message: fmt.Sprintf("invalid agents.yaml: %v", err),
			}},
		}, nil
	}

	// Validate agents
	issues := validateAgents(cfg.Agents)

	if len(issues) > 0 {
		// Return issues but still provide agents if possible
		return &LoadResult{
			Agents: cfg.Agents,
			Source: "file",
			Issues: issues,
		}, nil
	}

	return &LoadResult{
		Agents: cfg.Agents,
		Source: "file",
		Issues: []Issue{},
	}, nil
}

// validateAgents checks for duplicates and invalid identifiers
func validateAgents(agents []string) []Issue {
	var issues []Issue
	seen := make(map[string]bool)

	if len(agents) == 0 {
		issues = append(issues, Issue{
			Type:    "error",
			Field:   "/agents",
			Message: "agents array cannot be empty",
		})
		return issues
	}

	for i, agent := range agents {
		// Check for empty agent
		if agent == "" {
			issues = append(issues, Issue{
				Type:    "error",
				Field:   fmt.Sprintf("/agents/%d", i),
				Message: "agent name cannot be empty",
			})
			continue
		}

		// Check valid identifier
		if !validIdentRegex.MatchString(agent) {
			issues = append(issues, Issue{
				Type:    "error",
				Field:   fmt.Sprintf("/agents/%d", i),
				Message: fmt.Sprintf("invalid agent name: %s (must match ^[A-Za-z0-9_]+$)", agent),
			})
		}

		// Check for duplicates
		if seen[agent] {
			issues = append(issues, Issue{
				Type:    "error",
				Field:   fmt.Sprintf("/agents/%d", i),
				Message: fmt.Sprintf("duplicate agent: %s", agent),
			})
		}
		seen[agent] = true
	}

	return issues
}

// ToMap converts agent slice to map for quick lookup
func ToMap(agents []string) map[string]bool {
	m := make(map[string]bool, len(agents))
	for _, agent := range agents {
		m[agent] = true
	}
	return m
}