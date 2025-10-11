package pbi

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/pbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	infrarepo "github.com/YoshitsuguKoike/deespec/internal/infrastructure/repository"
	"github.com/spf13/cobra"
)

// NewSBICommand creates a new sbi command
func NewSBICommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sbi",
		Short: "Manage SBIs generated from PBI decomposition",
		Long:  "Commands for managing Small Backlog Items (SBIs) that are generated from PBI decomposition",
	}

	// Add subcommands
	cmd.AddCommand(NewSBIListCommand())
	cmd.AddCommand(NewSBIApproveCommand())
	cmd.AddCommand(NewSBIRejectCommand())
	cmd.AddCommand(NewSBIEditCommand())

	return cmd
}

// NewSBIListCommand creates a new sbi list command
func NewSBIListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <pbi-id>",
		Short: "List generated SBIs and their approval status",
		Long:  "Display a list of all generated SBIs with their approval status for a specific PBI",
		Example: `  # List SBIs for a PBI
  deespec pbi sbi list PBI-001`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pbiID := args[0]
			return runSBIList(pbiID)
		},
	}

	return cmd
}

func runSBIList(pbiID string) error {
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

	// Display header
	fmt.Printf("📦 PBI: %s\n", manifest.PBIID)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// Display generation info
	fmt.Printf("🕐 生成日時: %s\n", manifest.GeneratedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()

	// Display summary
	approvedCount := manifest.ApprovedCount()
	pendingCount := manifest.PendingCount()
	rejectedCount := manifest.RejectedCount()

	fmt.Println("📊 承認状況サマリー:")
	fmt.Printf("  総数: %d\n", manifest.TotalSBIs)
	fmt.Printf("  ✅ 承認済み: %d\n", approvedCount)
	fmt.Printf("  ⏳ 保留中: %d\n", pendingCount)
	fmt.Printf("  ❌ 否決: %d\n", rejectedCount)
	fmt.Println()

	// Check if already registered
	if manifest.Registered {
		fmt.Println("✅ 登録済み")
		if manifest.RegisteredAt != nil {
			fmt.Printf("   登録日時: %s\n", manifest.RegisteredAt.Format("2006-01-02 15:04:05"))
		}
		if len(manifest.RegisteredSBIs) > 0 {
			fmt.Printf("   登録されたSBI数: %d\n", len(manifest.RegisteredSBIs))
		}
		fmt.Println()
	}

	// Display SBI list in table format
	fmt.Println("📋 SBI一覧:")
	fmt.Println("─────────────────────────────────────────────────────────────────────────────────")

	// Table header
	fmt.Printf("%-4s %-30s %-10s %-15s %s\n", "No", "ファイル名", "ステータス", "レビュー者", "レビュー日時")
	fmt.Println("─────────────────────────────────────────────────────────────────────────────────")

	// Table rows
	for i, sbiRecord := range manifest.SBIs {
		statusIcon := getStatusIcon(sbiRecord.Status)
		statusText := fmt.Sprintf("%s %s", statusIcon, getStatusText(sbiRecord.Status))

		reviewedBy := "-"
		if sbiRecord.ReviewedBy != "" {
			reviewedBy = sbiRecord.ReviewedBy
		}

		reviewedAt := "-"
		if sbiRecord.ReviewedAt != nil {
			reviewedAt = sbiRecord.ReviewedAt.Format("2006-01-02 15:04")
		}

		// Format filename to fit in column (accounting for Japanese characters)
		filename := truncateStringJP(sbiRecord.File, 30)

		fmt.Printf("%-4d %-30s %-18s %-15s %s\n",
			i+1,
			filename,
			statusText,
			truncateStringJP(reviewedBy, 15),
			reviewedAt,
		)

		// Display notes if present
		if sbiRecord.Notes != "" {
			fmt.Printf("     💬 メモ: %s\n", sbiRecord.Notes)
		}

		// Display rejection reason if rejected
		if sbiRecord.Status == pbi.ApprovalStatusRejected && sbiRecord.RejectionReason != "" {
			fmt.Printf("     ⚠️  否決理由: %s\n", sbiRecord.RejectionReason)
		}
	}

	fmt.Println("─────────────────────────────────────────────────────────────────────────────────")
	fmt.Println()

	// Display next steps
	if !manifest.Registered && approvedCount > 0 {
		fmt.Println("💡 次のステップ:")
		fmt.Printf("   承認済みのSBIを登録するには以下を実行してください:\n")
		fmt.Printf("   $ deespec pbi sbi register-sbis %s\n", pbiID)
		fmt.Println()
	} else if !manifest.Registered && approvedCount == 0 && pendingCount > 0 {
		fmt.Println("💡 次のステップ:")
		fmt.Println("   SBIをレビューして承認してください:")
		fmt.Printf("   $ deespec pbi sbi approve %s <sbi-file>\n", pbiID)
		fmt.Println()
	}

	return nil
}

// getStatusIcon returns the emoji icon for the status
func getStatusIcon(status pbi.SBIApprovalStatus) string {
	switch status {
	case pbi.ApprovalStatusApproved:
		return "✅"
	case pbi.ApprovalStatusPending:
		return "⏳"
	case pbi.ApprovalStatusRejected:
		return "❌"
	case pbi.ApprovalStatusEdited:
		return "✏️"
	default:
		return "❓"
	}
}

// getStatusText returns the Japanese text for the status
func getStatusText(status pbi.SBIApprovalStatus) string {
	switch status {
	case pbi.ApprovalStatusApproved:
		return "承認済み"
	case pbi.ApprovalStatusPending:
		return "保留中"
	case pbi.ApprovalStatusRejected:
		return "否決"
	case pbi.ApprovalStatusEdited:
		return "編集済み"
	default:
		return "不明"
	}
}

// truncateStringJP truncates a string considering Japanese character width
// Japanese characters take roughly 2 display units, ASCII takes 1
func truncateStringJP(s string, maxLen int) string {
	if s == "" {
		return s
	}

	var displayWidth int
	var result strings.Builder

	for _, r := range s {
		charWidth := 1
		// Japanese characters (Hiragana, Katakana, Kanji, fullwidth)
		if r >= 0x3000 || utf8.RuneLen(r) > 2 {
			charWidth = 2
		}

		if displayWidth+charWidth > maxLen {
			if maxLen > 3 {
				result.WriteString("...")
			}
			break
		}

		result.WriteRune(r)
		displayWidth += charWidth
	}

	// Pad with spaces if needed
	padding := maxLen - displayWidth
	if padding > 0 {
		result.WriteString(strings.Repeat(" ", padding))
	}

	return result.String()
}
