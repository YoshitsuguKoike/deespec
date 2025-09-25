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
		Short: "Initialize a minimal workflow project with .deespec v0.1.14 structure",
		RunE: func(c *cobra.Command, _ []string) error {
			if dir == "" { dir = "." }

			// Create .deespec directory structure
			deespecDir := filepath.Join(dir, ".deespec")
			dirs := []string{
				filepath.Join(deespecDir, "etc"),
				filepath.Join(deespecDir, "prompts", "system"),
				filepath.Join(deespecDir, "specs", "sbi"),
				filepath.Join(deespecDir, "specs", "pbi"),
				filepath.Join(deespecDir, "var"),
				filepath.Join(deespecDir, "var", "artifacts"),
			}

			for _, d := range dirs {
				if err := os.MkdirAll(d, 0o755); err != nil {
					return fmt.Errorf("failed to create directory %s: %w", d, err)
				}
			}

			// Create .gitkeep files for VCS tracking
			gitkeepFiles := []string{
				filepath.Join(deespecDir, "etc", ".gitkeep"),
				filepath.Join(deespecDir, "prompts", "system", ".gitkeep"),
				filepath.Join(deespecDir, "specs", "sbi", ".gitkeep"),
				filepath.Join(deespecDir, "specs", "pbi", ".gitkeep"),
				filepath.Join(deespecDir, "var", ".keep"),
			}

			for _, f := range gitkeepFiles {
				if err := writeIfNotExists(f, []byte("")); err != nil {
					return fmt.Errorf("failed to create %s: %w", f, err)
				}
			}

			// Write workflow.yaml to .deespec/etc/
			if err := writeIfNotExists(filepath.Join(deespecDir, "etc", "workflow.yaml"), []byte(wfTmpl)); err != nil {
				return err
			}

			// Write state.json to .deespec/var/
			now := time.Now().UTC().Format(time.RFC3339)
			content := []byte(
				fmt.Sprintf(string(stTmpl), now),
			)
			if err := writeIfNotExists(filepath.Join(deespecDir, "var", "state.json"), content); err != nil {
				return err
			}

			fmt.Println("Initialized .deespec v0.1.14 structure:")
			fmt.Println("  .deespec/etc/workflow.yaml")
			fmt.Println("  .deespec/var/state.json")
			fmt.Println("  .deespec/var/artifacts/")
			fmt.Println("  .deespec/prompts/system/")
			fmt.Println("  .deespec/specs/{sbi,pbi}/")
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
