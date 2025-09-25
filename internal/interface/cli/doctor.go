package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/YoshitsuguKoike/deespec/internal/infra/config"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check environment & configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
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