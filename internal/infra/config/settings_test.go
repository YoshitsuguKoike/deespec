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
		envVars     map[string]string
		wantHome    string
		wantAgent   string
		wantTimeout int
		wantSource  string
	}{
		{
			name:        "Default values only",
			setupFunc:   nil,
			envVars:     nil,
			wantHome:    ".deespec",
			wantAgent:   "claude",
			wantTimeout: 60,
			wantSource:  "default",
		},
		{
			name:      "Environment variables only",
			setupFunc: nil,
			envVars: map[string]string{
				"DEE_HOME":        "/custom/home",
				"DEE_AGENT_BIN":   "custom-agent",
				"DEE_TIMEOUT_SEC": "120",
			},
			wantHome:    "/custom/home",
			wantAgent:   "custom-agent",
			wantTimeout: 120,
			wantSource:  "env",
		},
		{
			name: "JSON file only",
			setupFunc: func(t *testing.T, tmpDir string) {
				settings := map[string]interface{}{
					"home":        "/json/home",
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
			envVars:     nil,
			wantHome:    "/json/home",
			wantAgent:   "json-agent",
			wantTimeout: 180,
			wantSource:  "json",
		},
		{
			name: "JSON with ENV override",
			setupFunc: func(t *testing.T, tmpDir string) {
				settings := map[string]interface{}{
					"home":        "/json/home",
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
			envVars: map[string]string{
				"DEE_AGENT_BIN": "env-override-agent",
			},
			wantHome:    "/json/home",
			wantAgent:   "env-override-agent", // ENV overrides JSON
			wantTimeout: 180,
			wantSource:  "json", // Source is still JSON since it was loaded
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

			// Clear all environment variables first
			for _, env := range []string{
				"DEE_HOME", "DEE_AGENT_BIN", "DEE_TIMEOUT_SEC", "DEE_ARTIFACTS_DIR",
				"DEE_PROJECT_NAME", "DEE_LANGUAGE", "DEE_TURN", "DEE_TASK_ID",
				"DEE_VALIDATE", "DEE_AUTO_FB", "DEE_STRICT_FSYNC",
				"DEESPEC_TX_DEST_ROOT", "DEESPEC_DISABLE_RECOVERY", "DEESPEC_DISABLE_STATE_TX",
				"DEESPEC_USE_TX", "DEESPEC_DISABLE_METRICS_ROTATION", "DEESPEC_FSYNC_AUDIT",
				"DEESPEC_TEST_MODE", "DEESPEC_TEST_QUIET", "DEESPEC_WORKFLOW",
				"DEESPEC_POLICY_PATH", "DEESPEC_STDERR_LEVEL",
			} {
				os.Unsetenv(env)
			}

			// Setup test data
			if tt.setupFunc != nil {
				tt.setupFunc(t, tmpDir)
			}

			// Set environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
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
	if settings.TimeoutSec == nil || *settings.TimeoutSec != 60 {
		t.Errorf("Default timeout_sec should be 60")
	}
	if settings.DisableRecovery == nil || *settings.DisableRecovery != false {
		t.Errorf("Default disable_recovery should be false")
	}
}

func TestDeprecatedWarning(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "deprecated-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create setting.json with deprecated setting
	settings := map[string]interface{}{
		"disable_state_tx": true,
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "setting.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Load settings (should print warning)
	cfg, err := LoadSettings(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Restore stderr
	w.Close()
	os.Stderr = oldStderr

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Check warning was printed
	if !contains(output, "WARN") || !contains(output, "deprecated") {
		t.Errorf("Expected deprecation warning, got: %s", output)
	}

	// Check value was still loaded
	if !cfg.DisableStateTx() {
		t.Error("DisableStateTx should be true even though it's deprecated")
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
