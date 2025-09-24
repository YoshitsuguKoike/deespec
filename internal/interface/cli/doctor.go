package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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
			return nil
		},
	}
}