package agents

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAgents(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		exists         bool
		expectedSource string
		expectedAgents []string
		expectError    bool
		errorContains  string
	}{
		{
			name:           "file not exists - use builtin",
			exists:         false,
			expectedSource: "builtin",
			expectedAgents: BuiltinAgents,
			expectError:    false,
		},
		{
			name: "valid agents.yaml",
			content: `agents:
  - custom_agent
  - another_agent
  - test_agent`,
			exists:         true,
			expectedSource: "file",
			expectedAgents: []string{"custom_agent", "another_agent", "test_agent"},
			expectError:    false,
		},
		{
			name:           "empty agents array",
			content:        `agents: []`,
			exists:         true,
			expectedSource: "file",
			expectedAgents: []string{},
			expectError:    true,
			errorContains:  "agents array cannot be empty",
		},
		{
			name: "duplicate agents",
			content: `agents:
  - agent1
  - agent2
  - agent1`,
			exists:         true,
			expectedSource: "file",
			expectedAgents: []string{"agent1", "agent2", "agent1"},
			expectError:    true,
			errorContains:  "duplicate agent: agent1",
		},
		{
			name: "invalid agent name",
			content: `agents:
  - valid_agent
  - bad-agent!
  - another_valid`,
			exists:         true,
			expectedSource: "file",
			expectedAgents: []string{"valid_agent", "bad-agent!", "another_valid"},
			expectError:    true,
			errorContains:  "invalid agent name: bad-agent!",
		},
		{
			name: "unknown fields",
			content: `agents:
  - agent1
extra_field: should_not_exist`,
			exists:         true,
			expectedSource: "file",
			expectError:    true,
			errorContains:  "field extra_field not found",
		},
		{
			name: "invalid YAML",
			content: `agents:
  - agent1
  invalid yaml content`,
			exists:         true,
			expectedSource: "file",
			expectError:    true,
			errorContains:  "invalid agents.yaml",
		},
		{
			name: "empty agent name",
			content: `agents:
  - agent1
  - ""
  - agent3`,
			exists:         true,
			expectedSource: "file",
			expectError:    true,
			errorContains:  "agent name cannot be empty",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			agentsPath := filepath.Join(tmpDir, "agents.yaml")

			if tc.exists {
				if err := os.WriteFile(agentsPath, []byte(tc.content), 0644); err != nil {
					t.Fatal(err)
				}
			}

			result, err := LoadAgents(agentsPath)
			if err != nil {
				t.Fatalf("LoadAgents returned error: %v", err)
			}

			// Check source
			if result.Source != tc.expectedSource {
				t.Errorf("expected source %q, got %q", tc.expectedSource, result.Source)
			}

			// Check agents (if no error expected)
			if !tc.expectError && len(result.Agents) > 0 {
				if !equalSlices(result.Agents, tc.expectedAgents) {
					t.Errorf("expected agents %v, got %v", tc.expectedAgents, result.Agents)
				}
			}

			// Check for expected errors
			if tc.expectError {
				if len(result.Issues) == 0 {
					t.Error("expected issues but got none")
				} else {
					found := false
					for _, issue := range result.Issues {
						if containsString(issue.Message, tc.errorContains) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected error containing %q, got issues: %v", tc.errorContains, result.Issues)
					}
				}
			} else {
				if len(result.Issues) > 0 {
					t.Errorf("expected no issues, got: %v", result.Issues)
				}
			}
		})
	}
}

func TestValidateAgents(t *testing.T) {
	tests := []struct {
		name          string
		agents        []string
		expectIssues  bool
		issueContains string
	}{
		{
			name:         "valid agents",
			agents:       []string{"agent1", "agent_2", "Agent3", "AGENT_4"},
			expectIssues: false,
		},
		{
			name:          "empty array",
			agents:        []string{},
			expectIssues:  true,
			issueContains: "cannot be empty",
		},
		{
			name:          "duplicate agents",
			agents:        []string{"agent1", "agent2", "agent1"},
			expectIssues:  true,
			issueContains: "duplicate",
		},
		{
			name:          "invalid characters",
			agents:        []string{"agent-1", "agent.2"},
			expectIssues:  true,
			issueContains: "invalid agent name",
		},
		{
			name:          "empty string",
			agents:        []string{"agent1", "", "agent3"},
			expectIssues:  true,
			issueContains: "cannot be empty",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			issues := validateAgents(tc.agents)

			if tc.expectIssues {
				if len(issues) == 0 {
					t.Error("expected issues but got none")
				} else {
					found := false
					for _, issue := range issues {
						if containsString(issue.Message, tc.issueContains) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected issue containing %q, got: %v", tc.issueContains, issues)
					}
				}
			} else {
				if len(issues) > 0 {
					t.Errorf("expected no issues, got: %v", issues)
				}
			}
		})
	}
}

func TestToMap(t *testing.T) {
	agents := []string{"agent1", "agent2", "agent3"}
	m := ToMap(agents)

	if len(m) != len(agents) {
		t.Errorf("expected map size %d, got %d", len(agents), len(m))
	}

	for _, agent := range agents {
		if !m[agent] {
			t.Errorf("expected agent %q in map", agent)
		}
	}

	if m["nonexistent"] {
		t.Error("unexpected agent in map")
	}
}

// Helper functions
func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && containsSubstring(s, substr)
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
