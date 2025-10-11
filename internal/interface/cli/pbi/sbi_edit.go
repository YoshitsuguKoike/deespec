package pbi

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/pbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	infrarepo "github.com/YoshitsuguKoike/deespec/internal/infrastructure/repository"
	"github.com/spf13/cobra"
)

// sbiEditFlags holds flags for the sbi edit command
type sbiEditFlags struct {
	editor string
	user   string
	notes  string
}

// NewSBIEditCommand creates a new sbi edit command
func NewSBIEditCommand() *cobra.Command {
	flags := &sbiEditFlags{}

	cmd := &cobra.Command{
		Use:   "edit <pbi-id> <sbi-file>",
		Short: "Edit a generated SBI file and mark as edited",
		Long:  "Open an SBI file in an editor and automatically mark it as 'edited' status after editing",
		Example: `  # Edit an SBI with default editor (from EDITOR env var)
  deespec pbi sbi edit PBI-001 sbi_1.md

  # Edit with specific editor
  deespec pbi sbi edit PBI-001 sbi_2.md --editor vim

  # Edit with notes
  deespec pbi sbi edit PBI-001 sbi_1.md --notes "推定工数を3時間→4時間に修正"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			pbiID := args[0]
			sbiFile := args[1]
			return runSBIEdit(pbiID, sbiFile, flags)
		},
	}

	// Define flags
	cmd.Flags().StringVar(&flags.editor, "editor", "", "使用するエディタ（省略時は環境変数 EDITOR を使用、さらに省略時は vi）")
	cmd.Flags().StringVar(&flags.user, "user", "", "レビュー者名（省略時は環境変数 USER を使用）")
	cmd.Flags().StringVar(&flags.notes, "notes", "", "編集内容のメモ（任意）")

	return cmd
}

func runSBIEdit(pbiID string, sbiFile string, flags *sbiEditFlags) error {
	ctx := context.Background()

	// Create approval repository
	approvalRepo := infrarepo.NewSBIApprovalRepositoryImpl()

	// Check if manifest exists
	exists, err := approvalRepo.ManifestExists(ctx, repository.PBIID(pbiID))
	if err != nil {
		return fmt.Errorf("approval.yamlの確認に失敗しました: %w", err)
	}

	if !exists {
		return fmt.Errorf("approval.yamlが見つかりません: PBI %s\n"+
			"ヒント: まず 'deespec pbi decompose %s' を実行してSBIを生成してください", pbiID, pbiID)
	}

	// Build SBI file path and check existence
	sbiFilePath := filepath.Join(".deespec", "specs", "pbi", pbiID, sbiFile)
	if _, err := os.Stat(sbiFilePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("SBIファイルが見つかりません: %s\n"+
				"ヒント: 'deespec pbi sbi list %s' で利用可能なSBIファイルを確認してください", sbiFile, pbiID)
		}
		return fmt.Errorf("SBIファイルの確認に失敗しました: %w", err)
	}

	// Determine which editor to use
	editorCmd := determineEditor(flags.editor)

	// Display editor launch message
	fmt.Printf("📝 エディタを起動します: %s\n", editorCmd)
	fmt.Printf("   ファイル: %s\n", sbiFilePath)
	fmt.Println()

	// Launch editor
	if err := launchEditor(editorCmd, sbiFilePath); err != nil {
		return fmt.Errorf("エディタの起動に失敗しました: %w", err)
	}

	fmt.Println()
	fmt.Println("✅ エディタを閉じました")
	fmt.Println()

	// Load manifest
	manifest, err := approvalRepo.LoadManifest(ctx, repository.PBIID(pbiID))
	if err != nil {
		return fmt.Errorf("approval.yamlの読み込みに失敗しました: %w", err)
	}

	// Find the SBI record
	sbiIndex := -1
	for i, record := range manifest.SBIs {
		if record.File == sbiFile {
			sbiIndex = i
			break
		}
	}

	if sbiIndex == -1 {
		return fmt.Errorf("指定されたSBIファイルが見つかりません: %s\n"+
			"ヒント: 'deespec pbi sbi list %s' で利用可能なSBIファイルを確認してください", sbiFile, pbiID)
	}

	// Get reviewer name
	reviewer := flags.user
	if reviewer == "" {
		reviewer = os.Getenv("USER")
		if reviewer == "" {
			reviewer = "unknown"
		}
	}

	// Update the SBI record to "edited" status
	now := time.Now()
	manifest.SBIs[sbiIndex].Status = pbi.ApprovalStatusEdited
	manifest.SBIs[sbiIndex].ReviewedBy = reviewer
	manifest.SBIs[sbiIndex].ReviewedAt = &now
	manifest.SBIs[sbiIndex].Notes = flags.notes
	manifest.SBIs[sbiIndex].RejectionReason = "" // Clear rejection reason if previously rejected

	// Save the updated manifest
	if err := approvalRepo.SaveManifest(ctx, manifest); err != nil {
		return fmt.Errorf("approval.yamlの保存に失敗しました: %w", err)
	}

	// Display success message
	fmt.Printf("✏️  SBIを編集済みとしてマークしました: %s\n", sbiFile)
	fmt.Printf("   レビュー者: %s\n", reviewer)
	fmt.Printf("   レビュー日時: %s\n", now.Format("2006-01-02 15:04:05"))
	if flags.notes != "" {
		fmt.Printf("   メモ: %s\n", flags.notes)
	}
	fmt.Println()

	// Display progress
	approvedCount := manifest.ApprovedCount()
	totalCount := manifest.TotalSBIs
	fmt.Printf("📊 承認進捗: %d/%d 承認済み\n", approvedCount, totalCount)

	// Display next steps if all approved
	if approvedCount == totalCount {
		fmt.Println()
		fmt.Println("🎉 すべてのSBIが承認されました！")
		fmt.Println("💡 次のステップ:")
		fmt.Printf("   承認済みのSBIを登録するには以下を実行してください:\n")
		fmt.Printf("   $ deespec pbi sbi register-sbis %s\n", pbiID)
	} else if approvedCount < totalCount {
		pendingCount := manifest.PendingCount()
		if pendingCount > 0 {
			fmt.Println()
			fmt.Println("💡 次のステップ:")
			fmt.Printf("   残りの%d件のSBIをレビューして承認してください\n", pendingCount)
		}
	}

	return nil
}

// determineEditor determines which editor to use based on flags and environment
func determineEditor(editorFlag string) string {
	// Priority: --editor flag > EDITOR env var > vi (fallback)
	if editorFlag != "" {
		return editorFlag
	}

	editorEnv := os.Getenv("EDITOR")
	if editorEnv != "" {
		return editorEnv
	}

	return "vi"
}

// launchEditor launches the specified editor with the given file path
func launchEditor(editorCmd string, filePath string) error {
	// Create command
	cmd := exec.Command(editorCmd, filePath)

	// Connect stdin, stdout, stderr to allow interactive editing
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run the editor and wait for it to complete
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("エディタの実行に失敗しました (%s): %w", editorCmd, err)
	}

	return nil
}
