package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/YoshitsuguKoike/deespec/internal/infra/config"
)

// DoctorJSON represents the JSON output structure for doctor command
type DoctorJSON struct {
	Runner           string   `json:"runner"`
	Active           bool     `json:"active"`
	WorkingDir       string   `json:"working_dir"`
	AgentBin         string   `json:"agent_bin"`
	StartIntervalSec int      `json:"start_interval_sec,omitempty"`
	Next             string   `json:"next,omitempty"`
	Errors           []string `json:"errors"`
}

func newDoctorCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check environment & configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if jsonOutput {
				return runDoctorJSON()
			}
			cfg := config.Load()
			fmt.Println("AgentBin:", cfg.AgentBin)
			fmt.Println("ArtifactsDir:", cfg.ArtifactsDir)
			fmt.Println("Timeout:", cfg.Timeout)

			if _, err := exec.LookPath(cfg.AgentBin); err != nil {
				fmt.Printf("WARN: %s not found in PATH\n", cfg.AgentBin)
			} else {
				fmt.Printf("OK: %s found\n", cfg.AgentBin)
			}
			if err := os.MkdirAll(cfg.ArtifactsDir, 0o755); err != nil {
				return fmt.Errorf("artifacts dir error: %w", err)
			}
			probeFile := filepath.Join(cfg.ArtifactsDir, ".probe")
			if f, err := os.Create(probeFile); err != nil {
				return fmt.Errorf("write check failed: %w", err)
			} else {
				f.Close()
				os.Remove(probeFile) // Clean up probe file
			}
			fmt.Println("OK: write permission in artifacts dir")

			// Check for scheduler (launchd/systemd)
			checkScheduler()

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	return cmd
}

func checkScheduler() {
	switch runtime.GOOS {
	case "darwin":
		// Check launchd on macOS
		plistPath := os.ExpandEnv("$HOME/Library/LaunchAgents/com.deespec.runner.plist")
		if _, err := os.Stat(plistPath); err == nil {
			// Check if loaded
			cmd := exec.Command("launchctl", "list")
			output, _ := cmd.Output()
			if contains(string(output), "com.deespec.runner") {
				fmt.Println("OK: launchd service loaded (com.deespec.runner)")
			} else {
				fmt.Println("INFO: launchd plist exists but not loaded")
				fmt.Printf("  Run: launchctl load %s\n", plistPath)
			}
		} else {
			fmt.Println("INFO: launchd not configured")
			fmt.Println("  See: https://github.com/YoshitsuguKoike/deespec#5-min-loop")
		}
	case "linux":
		// Check systemd on Linux
		servicePath := os.ExpandEnv("$HOME/.config/systemd/user/deespec.service")
		timerPath := os.ExpandEnv("$HOME/.config/systemd/user/deespec.timer")

		if _, err := os.Stat(servicePath); err == nil {
			// Check timer status
			cmd := exec.Command("systemctl", "--user", "is-active", "deespec.timer")
			output, _ := cmd.Output()
			status := string(output)
			if status == "active\n" {
				fmt.Println("OK: systemd timer active (deespec.timer)")
			} else if _, err := os.Stat(timerPath); err == nil {
				fmt.Println("INFO: systemd timer exists but not active")
				fmt.Println("  Run: systemctl --user enable --now deespec.timer")
			} else {
				fmt.Println("INFO: systemd service exists but timer not configured")
			}
		} else {
			fmt.Println("INFO: systemd not configured")
			fmt.Println("  See: https://github.com/YoshitsuguKoike/deespec#5-min-loop")
		}
	default:
		fmt.Printf("INFO: Scheduler check not available for %s\n", runtime.GOOS)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func runDoctorJSON() error {
	cfg := config.Load()
	result := DoctorJSON{
		Runner:     "none",
		Active:     false,
		WorkingDir: "",
		AgentBin:   cfg.AgentBin,
		Errors:     []string{},
	}

	// Check working directory
	if wd, err := os.Getwd(); err == nil {
		result.WorkingDir = wd
		// Check write permission
		probeFile := filepath.Join(wd, ".probe")
		if f, err := os.Create(probeFile); err != nil {
			result.Errors = append(result.Errors, "working_dir not writable")
		} else {
			f.Close()
			os.Remove(probeFile)
		}
	}

	// Check agent binary
	if _, err := exec.LookPath(cfg.AgentBin); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("agent_bin '%s' not found", cfg.AgentBin))
	}

	// Check scheduler based on OS
	switch runtime.GOOS {
	case "darwin":
		checkLaunchdJSON(&result)
	case "linux":
		checkSystemdJSON(&result)
	}

	// Determine exit code
	exitCode := 0
	if len(result.Errors) > 0 {
		for _, err := range result.Errors {
			if strings.Contains(err, "not writable") || strings.Contains(err, "not found") {
				exitCode = 1
				break
			}
		}
		if exitCode == 0 {
			exitCode = 2
		}
	} else if !result.Active {
		exitCode = 2
	}

	// Output JSON
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "")
	if err := enc.Encode(result); err != nil {
		return err
	}

	os.Exit(exitCode)
	return nil
}

func checkLaunchdJSON(result *DoctorJSON) {
	plistPath := os.ExpandEnv("$HOME/Library/LaunchAgents/com.deespec.runner.plist")

	if _, err := os.Stat(plistPath); err == nil {
		result.Runner = "launchd"

		// Check if loaded
		cmd := exec.Command("launchctl", "list")
		output, _ := cmd.Output()
		if contains(string(output), "com.deespec.runner") {
			result.Active = true
		}

		// Extract StartInterval from plist
		cmd = exec.Command("plutil", "-p", plistPath)
		if output, err := cmd.Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "StartInterval") {
					parts := strings.Split(line, "=>")
					if len(parts) == 2 {
						if val, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
							result.StartIntervalSec = val
						}
					}
				}
				if strings.Contains(line, "WorkingDirectory") {
					// Extract value after => and remove quotes
					parts := strings.Split(line, "=>")
					if len(parts) == 2 {
						val := strings.TrimSpace(parts[1])
						val = strings.Trim(val, "\"")
						result.WorkingDir = val
					}
				}
			}
		}
	}
}

func checkSystemdJSON(result *DoctorJSON) {
	servicePath := os.ExpandEnv("$HOME/.config/systemd/user/deespec.service")
	timerPath := os.ExpandEnv("$HOME/.config/systemd/user/deespec.timer")

	if _, err := os.Stat(servicePath); err == nil {
		result.Runner = "systemd"

		// Check timer status
		cmd := exec.Command("systemctl", "--user", "is-active", "deespec.timer")
		output, _ := cmd.Output()
		if strings.TrimSpace(string(output)) == "active" {
			result.Active = true
		}

		// Try to get timer info
		if _, err := os.Stat(timerPath); err == nil {
			// Default to 5 minutes for systemd timer
			result.StartIntervalSec = 300
		}
	}
}