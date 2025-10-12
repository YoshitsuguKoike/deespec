package pbi

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/pbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence/sqlite"
	infrarepo "github.com/YoshitsuguKoike/deespec/internal/infrastructure/repository"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
)

// sbiApproveFlags holds flags for the sbi approve command
type sbiApproveFlags struct {
	notes string
	user  string
	all   bool
}

// NewSBIApproveCommand creates a new sbi approve command
func NewSBIApproveCommand() *cobra.Command {
	flags := &sbiApproveFlags{}

	cmd := &cobra.Command{
		Use:   "approve <pbi-id> [<sbi-file>]",
		Short: "Approve a generated SBI file or all SBI files",
		Long:  "Mark a generated SBI file as approved, recording reviewer information and optional notes. Use --all to approve all pending SBIs at once.",
		Example: `  # Approve a single SBI
  deespec pbi sbi approve PBI-001 sbi_1.md

  # Approve all SBIs (requires report.md)
  deespec pbi sbi approve PBI-001 --all

  # Approve with notes
  deespec pbi sbi approve PBI-001 sbi_2.md --notes "工数を修正済み"

  # Approve with specific reviewer
  deespec pbi sbi approve PBI-001 sbi_1.md --user "alice"`,
		Args: func(cmd *cobra.Command, args []string) error {
			// If --all is specified, require exactly 1 arg (pbi-id)
			// Otherwise, require exactly 2 args (pbi-id and sbi-file)
			if flags.all {
				if len(args) != 1 {
					return fmt.Errorf("--all フラグ使用時はPBI IDのみを指定してください")
				}
			} else {
				if len(args) != 2 {
					return fmt.Errorf("PBI IDとSBIファイル名の両方を指定してください")
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			pbiID := args[0]
			if flags.all {
				return runSBIApproveAll(pbiID, flags)
			}
			sbiFile := args[1]
			return runSBIApprove(pbiID, sbiFile, flags)
		},
	}

	// Define flags
	cmd.Flags().StringVar(&flags.notes, "notes", "", "承認時のメモ（任意）")
	cmd.Flags().StringVar(&flags.user, "user", "", "レビュー者名（省略時は環境変数 USER を使用）")
	cmd.Flags().BoolVar(&flags.all, "all", false, "全てのpending SBIを一括承認（report.md必須）")

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
		// Update PBI status to planed
		if err := updatePBIStatusToPlaned(ctx, pbiID); err != nil {
			// Don't fail the command, just log the error
			fmt.Printf("⚠️  Warning: Failed to update PBI status: %v\n", err)
		}

		fmt.Println()
		fmt.Println("🎉 すべてのSBIが承認されました！")
		fmt.Println("💡 次のステップ:")
		fmt.Printf("   承認済みのSBIを登録するには以下を実行してください:\n")
		fmt.Printf("   $ deespec pbi register %s\n", pbiID)
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

func runSBIApproveAll(pbiID string, flags *sbiApproveFlags) error {
	ctx := context.Background()

	// 1. Check if report.md exists
	reportPath := fmt.Sprintf(".deespec/specs/pbi/%s/report.md", pbiID)
	if _, err := os.Stat(reportPath); os.IsNotExist(err) {
		return fmt.Errorf("❌ report.mdが見つかりません: %s\n\n"+
			"💡 --allフラグを使用するには、AIエージェントによるSBI生成とreport.md作成が完了している必要があります。\n"+
			"   以下のコマンドでAIエージェントを実行してください:\n"+
			"   $ deespec pbi decompose %s\n\n"+
			"   または、個別にSBIを承認する場合は:\n"+
			"   $ deespec pbi sbi approve %s <sbi-file>", reportPath, pbiID, pbiID)
	}

	// 2. Create approval repository
	approvalRepo := infrarepo.NewSBIApprovalRepositoryImpl()

	// 3. Check if manifest exists
	exists, err := approvalRepo.ManifestExists(ctx, repository.PBIID(pbiID))
	if err != nil {
		return fmt.Errorf("approval.yamlの確認に失敗しました: %w", err)
	}

	if !exists {
		return fmt.Errorf("approval.yamlが見つかりません: PBI %s\n"+
			"ヒント: まず 'deespec pbi decompose %s' を実行してSBIを生成してください", pbiID, pbiID)
	}

	// 4. Load manifest
	manifest, err := approvalRepo.LoadManifest(ctx, repository.PBIID(pbiID))
	if err != nil {
		return fmt.Errorf("approval.yamlの読み込みに失敗しました: %w", err)
	}

	// 5. Get reviewer name
	reviewer := flags.user
	if reviewer == "" {
		reviewer = os.Getenv("USER")
		if reviewer == "" {
			reviewer = "unknown"
		}
	}

	// 6. Count pending SBIs and approve all of them
	now := time.Now()
	approvedCount := 0
	for i := range manifest.SBIs {
		if manifest.SBIs[i].Status == pbi.ApprovalStatusPending {
			manifest.SBIs[i].Status = pbi.ApprovalStatusApproved
			manifest.SBIs[i].ReviewedBy = reviewer
			manifest.SBIs[i].ReviewedAt = &now
			manifest.SBIs[i].Notes = flags.notes
			manifest.SBIs[i].RejectionReason = ""
			approvedCount++
		}
	}

	// 7. Check if there were any pending SBIs
	if approvedCount == 0 {
		fmt.Println("ℹ️  承認が必要なSBIはありません")
		fmt.Printf("   全て承認済み: %d/%d\n", manifest.ApprovedCount(), manifest.TotalSBIs)
		return nil
	}

	// 8. Save the updated manifest
	if err := approvalRepo.SaveManifest(ctx, manifest); err != nil {
		return fmt.Errorf("approval.yamlの保存に失敗しました: %w", err)
	}

	// 9. Display success message
	fmt.Printf("✅ 全てのSBIを一括承認しました: %d件\n", approvedCount)
	fmt.Printf("   レビュー者: %s\n", reviewer)
	fmt.Printf("   レビュー日時: %s\n", now.Format("2006-01-02 15:04:05"))
	if flags.notes != "" {
		fmt.Printf("   メモ: %s\n", flags.notes)
	}
	fmt.Println()

	// 10. Display progress
	totalApprovedCount := manifest.ApprovedCount()
	totalCount := manifest.TotalSBIs
	fmt.Printf("📊 承認進捗: %d/%d 承認済み\n", totalApprovedCount, totalCount)

	// 11. Display next steps if all approved
	if totalApprovedCount == totalCount {
		// Update PBI status to planed
		if err := updatePBIStatusToPlaned(ctx, pbiID); err != nil {
			// Don't fail the command, just log the error
			fmt.Printf("⚠️  Warning: Failed to update PBI status: %v\n", err)
		}

		fmt.Println()
		fmt.Println("🎉 すべてのSBIが承認されました！")
		fmt.Println("💡 次のステップ:")
		fmt.Printf("   承認済みのSBIを登録するには以下を実行してください:\n")
		fmt.Printf("   $ deespec pbi register %s\n", pbiID)
	}

	return nil
}

// updatePBIStatusToPlaned updates PBI status to planed when all SBIs are approved
func updatePBIStatusToPlaned(ctx context.Context, pbiID string) error {
	// Open database
	db, err := sql.Open("sqlite3", ".deespec/deespec.db")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Run migrations
	migrator := sqlite.NewMigrator(db)
	if err := migrator.Migrate(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Create PBI repository
	rootPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	pbiRepo := persistence.NewPBISQLiteRepository(db, rootPath)

	// Get PBI entity
	pbiEntity, err := pbiRepo.FindByID(pbiID)
	if err != nil {
		return fmt.Errorf("failed to find PBI: %w", err)
	}

	// Get PBI body
	pbiBody, err := pbiRepo.GetBody(pbiID)
	if err != nil {
		return fmt.Errorf("failed to get PBI body: %w", err)
	}

	// Update status to planed
	if err := pbiEntity.UpdateStatus(pbi.StatusPlaned); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	// Save updated PBI
	if err := pbiRepo.Save(pbiEntity, pbiBody); err != nil {
		return fmt.Errorf("failed to save PBI: %w", err)
	}

	return nil
}
