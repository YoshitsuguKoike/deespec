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

// sbiRejectFlags holds flags for the sbi reject command
type sbiRejectFlags struct {
	reason string
	user   string
}

// NewSBIRejectCommand creates a new sbi reject command
func NewSBIRejectCommand() *cobra.Command {
	flags := &sbiRejectFlags{}

	cmd := &cobra.Command{
		Use:   "reject <pbi-id> <sbi-file>",
		Short: "Reject a generated SBI file",
		Long:  "Mark a generated SBI file as rejected, recording reviewer information and mandatory rejection reason",
		Example: `  # Reject an SBI with reason
  deespec pbi sbi reject PBI-001 sbi_1.md --reason "要件が不明確"

  # Reject with specific reviewer
  deespec pbi sbi reject PBI-001 sbi_2.md --reason "粒度が大きすぎる" --user "alice"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			pbiID := args[0]
			sbiFile := args[1]
			return runSBIReject(pbiID, sbiFile, flags)
		},
	}

	// Define flags
	cmd.Flags().StringVar(&flags.reason, "reason", "", "否決理由（必須）")
	cmd.Flags().StringVar(&flags.user, "user", "", "レビュー者名（省略時は環境変数 USER を使用）")

	// Mark reason flag as required
	if err := cmd.MarkFlagRequired("reason"); err != nil {
		// This should never happen during initialization
		panic(fmt.Sprintf("failed to mark reason flag as required: %v", err))
	}

	return cmd
}

func runSBIReject(pbiID string, sbiFile string, flags *sbiRejectFlags) error {
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
	manifest.SBIs[sbiIndex].Status = pbi.ApprovalStatusRejected
	manifest.SBIs[sbiIndex].ReviewedBy = reviewer
	manifest.SBIs[sbiIndex].ReviewedAt = &now
	manifest.SBIs[sbiIndex].RejectionReason = flags.reason
	manifest.SBIs[sbiIndex].Notes = "" // Clear notes if previously approved

	// Save the updated manifest
	if err := approvalRepo.SaveManifest(ctx, manifest); err != nil {
		return fmt.Errorf("approval.yamlの保存に失敗しました: %w", err)
	}

	// Display success message
	fmt.Printf("❌ SBIを否決しました: %s\n", sbiFile)
	fmt.Printf("   レビュー者: %s\n", reviewer)
	fmt.Printf("   レビュー日時: %s\n", now.Format("2006-01-02 15:04:05"))
	fmt.Printf("   否決理由: %s\n", flags.reason)
	fmt.Println()

	// Display progress
	approvedCount := manifest.ApprovedCount()
	rejectedCount := manifest.RejectedCount()
	pendingCount := manifest.PendingCount()
	totalCount := manifest.TotalSBIs
	fmt.Printf("📊 承認進捗: %d/%d 承認済み (%d 否決, %d 保留中)\n", approvedCount, totalCount, rejectedCount, pendingCount)

	// Display next steps
	if pendingCount > 0 {
		fmt.Println()
		fmt.Println("💡 次のステップ:")
		fmt.Printf("   残りの%d件のSBIをレビューしてください\n", pendingCount)
		fmt.Printf("   $ deespec pbi sbi list %s\n", pbiID)
	} else if approvedCount > 0 {
		fmt.Println()
		fmt.Println("💡 次のステップ:")
		fmt.Printf("   承認済みのSBIを登録するには以下を実行してください:\n")
		fmt.Printf("   $ deespec pbi sbi register-sbis %s\n", pbiID)
	}

	return nil
}
