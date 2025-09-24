package cli

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

//go:embed templates/workflow.yaml.tmpl
var wfTmpl string

//go:embed templates/state.json.tmpl
var stTmpl string

func newInitCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a minimal workflow project",
		RunE: func(c *cobra.Command, _ []string) error {
			if dir == "" { dir = "." }
			if err := os.MkdirAll(filepath.Join(dir, ".artifacts"), 0o755); err != nil {
				return err
			}
			// write workflow.yaml
			if err := writeIfNotExists(filepath.Join(dir, "workflow.yaml"), []byte(wfTmpl)); err != nil {
				return err
			}
			// write state.json（最終更新時刻だけ埋める）
			now := time.Now().UTC().Format(time.RFC3339)
			content := []byte(
				fmt.Sprintf(string(stTmpl), now),
			)
			if err := writeIfNotExists(filepath.Join(dir, "state.json"), content); err != nil {
				return err
			}
			fmt.Println("Initialized: workflow.yaml, state.json, ./.artifacts/")
			return nil
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Target directory")
	return cmd
}

func writeIfNotExists(path string, b []byte) error {
	if _, err := os.Stat(path); err == nil {
		return nil // 既存は上書きしない（安全第一）
	}
	return os.WriteFile(path, b, 0o644)
}
