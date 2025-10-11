package pbi

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/pbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	infrarepo "github.com/YoshitsuguKoike/deespec/internal/infrastructure/repository"
	"github.com/spf13/cobra"
)

// sbiApproveFlags holds flags for the sbi approve command
type sbiApproveFlags struct {
	notes string
	user  string
}

// NewSBIApproveCommand creates a new sbi approve command
func NewSBIApproveCommand() *cobra.Command {
	flags := &sbiApproveFlags{}

	cmd := &cobra.Command{
		Use:   "approve <pbi-id> <sbi-file>",
		Short: "Approve a generated SBI file",
		Long:  "Mark a generated SBI file as approved, recording reviewer information and optional notes",
		Example: `  # Approve an SBI
  deespec pbi sbi approve PBI-001 sbi_1.md

  # Approve with notes
  deespec pbi sbi approve PBI-001 sbi_2.md --notes "工数を修正済み"

  # Approve with specific reviewer
  deespec pbi sbi approve PBI-001 sbi_1.md --user "alice"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			pbiID := args[0]
			sbiFile := args[1]
			return runSBIApprove(pbiID, sbiFile, flags)
		},
	}

	// Define flags
	cmd.Flags().StringVar(&flags.notes, "notes", "", "承認時のメモ（任意）")
	cmd.Flags().StringVar(&flags.user, "user", "", "レビュー者名（省略時は環境変数 USER を使用）")

	return cmd
}

func runSBIApprove(pbiID string, sbiFile string, flags *sbiApproveFlags) error {
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

	// Update the SBI record
	now := time.Now()
	manifest.SBIs[sbiIndex].Status = pbi.ApprovalStatusApproved
	manifest.SBIs[sbiIndex].ReviewedBy = reviewer
	manifest.SBIs[sbiIndex].ReviewedAt = &now
	manifest.SBIs[sbiIndex].Notes = flags.notes
	manifest.SBIs[sbiIndex].RejectionReason = "" // Clear rejection reason if previously rejected

	// Save the updated manifest
	if err := approvalRepo.SaveManifest(ctx, manifest); err != nil {
		return fmt.Errorf("approval.yamlの保存に失敗しました: %w", err)
	}

	// Display success message
	fmt.Printf("✅ SBIを承認しました: %s\n", sbiFile)
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
