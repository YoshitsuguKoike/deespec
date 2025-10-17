package upgrade

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/YoshitsuguKoike/deespec/internal/buildinfo"
	"github.com/YoshitsuguKoike/deespec/internal/embed"
	"github.com/spf13/cobra"
)

const (
	githubAPIURL     = "https://api.github.com/repos/YoshitsuguKoike/deespec/releases/latest"
	githubReleaseURL = "https://github.com/YoshitsuguKoike/deespec/releases/download"
)

// GitHubRelease represents GitHub release API response
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
}

func NewCommand() *cobra.Command {
	var forceUpgrade bool
	var promptOnly bool

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade deespec to the latest version",
		Long: `Download and install the latest version of deespec from GitHub releases.

This command will:
1. Check the latest version available on GitHub
2. Download the appropriate binary for your platform
3. Replace the current binary with the new one
4. Update prompt templates to .deespec/prompts/
5. Verify the installation

Example:
  deespec upgrade              # Upgrade binary and prompts
  deespec upgrade --force      # Force upgrade even if already latest
  deespec upgrade --prompt-only # Update only prompt templates`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if promptOnly {
				return upgradePromptsOnly()
			}
			return upgradeToLatest(forceUpgrade)
		},
	}

	cmd.Flags().BoolVarP(&forceUpgrade, "force", "f", false, "Force upgrade even if already latest version")
	cmd.Flags().BoolVar(&promptOnly, "prompt-only", false, "Update only prompt templates without upgrading binary")

	return cmd
}

func upgradeToLatest(force bool) error {
	// 1. 現在のバージョン確認
	currentVersion := buildinfo.GetVersion()
	fmt.Printf("Current version: %s\n", currentVersion)

	// 2. 最新バージョン取得（GitHub API）
	fmt.Println("Checking for latest version...")
	latestVersion, err := getLatestVersion()
	if err != nil {
		return fmt.Errorf("failed to get latest version: %w", err)
	}
	fmt.Printf("Latest version:  %s\n", latestVersion)

	// バージョン比較
	if currentVersion == latestVersion && !force {
		fmt.Println("✅ Already up to date!")
		return nil
	}

	if force {
		fmt.Println("⚠️  Force upgrade requested")
	}

	// 3. バイナリダウンロード
	downloadURL := getBinaryURL(latestVersion)
	fmt.Printf("\nDownloading from: %s\n", downloadURL)

	tmpFile, err := downloadBinary(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}
	defer os.Remove(tmpFile)

	// 4. バイナリ置き換え
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// シンボリックリンクの場合は実体を取得
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve symlink: %w", err)
	}

	fmt.Printf("Installing to: %s\n", execPath)

	if err := replaceBinary(tmpFile, execPath); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	fmt.Printf("\n✅ Successfully upgraded to %s\n", latestVersion)

	// Also update prompt templates
	fmt.Println("\nUpdating prompt templates...")
	if err := updatePromptTemplates(); err != nil {
		// Don't fail the upgrade if prompt update fails
		fmt.Printf("⚠️  Warning: Failed to update prompts: %v\n", err)
		fmt.Println("    You can update prompts manually with: deespec upgrade --prompt-only")
	} else {
		fmt.Println("✅ Prompt templates updated")
	}

	fmt.Println("\nRun 'deespec version' to verify the installation")

	return nil
}

func getLatestVersion() (string, error) {
	resp, err := http.Get(githubAPIURL)
	if err != nil {
		return "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if release.TagName == "" {
		return "", fmt.Errorf("no tag_name found in release")
	}

	return release.TagName, nil
}

func getBinaryURL(version string) string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// バージョンがvで始まっていない場合は追加
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}

	// https://github.com/YoshitsuguKoike/deespec/releases/download/v0.2.4/deespec_darwin_arm64
	return fmt.Sprintf("%s/%s/deespec_%s_%s", githubReleaseURL, version, goos, goarch)
}

func downloadBinary(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d (URL: %s)", resp.StatusCode, url)
	}

	tmpFile, err := os.CreateTemp("", "deespec-upgrade-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	written, err := io.Copy(tmpFile, resp.Body)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write binary: %w", err)
	}

	tmpFile.Close()

	fmt.Printf("Downloaded %d bytes\n", written)

	// 実行権限付与
	if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to set permissions: %w", err)
	}

	return tmpFile.Name(), nil
}

func replaceBinary(newPath, oldPath string) error {
	// バックアップ作成
	backupPath := oldPath + ".backup"
	if err := os.Rename(oldPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	// 新しいバイナリを配置
	if err := os.Rename(newPath, oldPath); err != nil {
		// ロールバック
		if rollbackErr := os.Rename(backupPath, oldPath); rollbackErr != nil {
			return fmt.Errorf("failed to install and rollback failed: install error: %w, rollback error: %v", err, rollbackErr)
		}
		return fmt.Errorf("failed to install new binary: %w", err)
	}

	// バックアップ削除
	os.Remove(backupPath)

	return nil
}

// upgradePromptsOnly updates only prompt templates to .deespec/prompts/
func upgradePromptsOnly() error {
	fmt.Println("Updating prompt templates...")

	if err := updatePromptTemplates(); err != nil {
		return fmt.Errorf("failed to update prompts: %w", err)
	}

	fmt.Println("✅ Prompt templates updated successfully")
	return nil
}

// updatePromptTemplates copies prompt templates from embedded files to .deespec/prompts/
func updatePromptTemplates() error {
	// Get all templates
	templates, err := embed.GetTemplates()
	if err != nil {
		return fmt.Errorf("failed to get templates: %w", err)
	}

	// Filter only prompt templates
	var promptTemplates []embed.Template
	for _, tmpl := range templates {
		if strings.HasPrefix(tmpl.Path, "prompts/") {
			promptTemplates = append(promptTemplates, tmpl)
		}
	}

	if len(promptTemplates) == 0 {
		return fmt.Errorf("no prompt templates found")
	}

	// Write prompt templates to .deespec/prompts/
	baseDir := ".deespec"
	updatedCount := 0

	for _, tmpl := range promptTemplates {
		result, err := embed.WriteTemplate(baseDir, tmpl, true) // force=true to overwrite
		if err != nil {
			return fmt.Errorf("failed to write %s: %w", tmpl.Path, err)
		}

		if result.Action == "WROTE" || result.Action == "WROTE (force)" {
			fmt.Printf("  ✓ Updated: %s\n", tmpl.Path)
			updatedCount++
		}
	}

	fmt.Printf("\nUpdated %d prompt template(s)\n", updatedCount)
	return nil
}
