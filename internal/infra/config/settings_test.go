package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSettings(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(t *testing.T, tmpDir string)
		wantHome    string
		wantAgent   string
		wantTimeout int
		wantSource  string
	}{
		{
			name:        "Default values only",
			setupFunc:   nil,
			wantHome:    ".deespec",
			wantAgent:   "claude",
			wantTimeout: 900, // Updated to match new default
			wantSource:  "default",
		},
		{
			name: "JSON file only",
			setupFunc: func(t *testing.T, tmpDir string) {
				settings := map[string]interface{}{
					"home":        "json/home",
					"agent_bin":   "json-agent",
					"timeout_sec": 180,
				}
				data, err := json.MarshalIndent(settings, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(tmpDir, "setting.json"), data, 0644); err != nil {
					t.Fatal(err)
				}
			},
			wantHome:    "json/home",
			wantAgent:   "json-agent",
			wantTimeout: 180,
			wantSource:  "json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tmpDir, err := os.MkdirTemp("", "config-test-*")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(tmpDir)

			// Setup test data
			if tt.setupFunc != nil {
				tt.setupFunc(t, tmpDir)
			}

			// Load settings
			cfg, err := LoadSettings(tmpDir)
			if err != nil {
				t.Fatalf("LoadSettings() error = %v", err)
			}

			// Check values
			if got := cfg.Home(); got != tt.wantHome {
				t.Errorf("Home() = %v, want %v", got, tt.wantHome)
			}
			if got := cfg.AgentBin(); got != tt.wantAgent {
				t.Errorf("AgentBin() = %v, want %v", got, tt.wantAgent)
			}
			if got := cfg.TimeoutSec(); got != tt.wantTimeout {
				t.Errorf("TimeoutSec() = %v, want %v", got, tt.wantTimeout)
			}
			if got := cfg.ConfigSource(); got != tt.wantSource {
				t.Errorf("ConfigSource() = %v, want %v", got, tt.wantSource)
			}
		})
	}
}

func TestToBool(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"yes", true},
		{"YES", true},
		{"on", true},
		{"ON", true},
		{"0", false},
		{"false", false},
		{"FALSE", false},
		{"no", false},
		{"off", false},
		{"", false},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := toBool(tt.input); got != tt.want {
				t.Errorf("toBool(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestCreateDefaultSettings(t *testing.T) {
	data := CreateDefaultSettings()

	// Parse the JSON
	var settings RawSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Failed to parse default settings: %v", err)
	}

	// Check some key defaults
	if settings.Home == nil || *settings.Home != ".deespec" {
		t.Errorf("Default home should be .deespec")
	}
	if settings.AgentBin == nil || *settings.AgentBin != "claude" {
		t.Errorf("Default agent_bin should be claude")
	}
	if settings.TimeoutSec == nil || *settings.TimeoutSec != 900 {
		t.Errorf("Default timeout_sec should be 900")
	}
	if settings.DisableRecovery == nil || *settings.DisableRecovery != false {
		t.Errorf("Default disable_recovery should be false")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && hasSubstring(s, substr)
}

func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
