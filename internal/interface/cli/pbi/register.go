package pbi

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	pbiusecase "github.com/YoshitsuguKoike/deespec/internal/application/usecase/pbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence/sqlite"
	infrarepo "github.com/YoshitsuguKoike/deespec/internal/infrastructure/repository"
	"github.com/spf13/cobra"
)

// registerFlags holds flags for the register command
type registerFlags struct {
	dryRun bool
	force  bool
}

// NewRegisterCommand creates a new register command
func NewRegisterCommand() *cobra.Command {
	flags := &registerFlags{}

	cmd := &cobra.Command{
		Use:   "register <pbi-id>",
		Short: "Register approved SBIs to the database",
		Long: `Register approved SBIs from approval.yaml to the database.

This command reads the approval.yaml file for the specified PBI and registers
all approved SBIs to the database, making them available for execution in the
deespec workflow.

Only PBIs with approved SBIs can be registered. After registration, the SBIs
can be executed using 'deespec run' or 'deespec sbi list'.`,
		Example: `  # Register all approved SBIs for a PBI
  deespec pbi register PBI-001

  # Preview registration without saving (dry-run mode)
  deespec pbi register PBI-001 --dry-run

  # Force re-registration of already registered SBIs
  deespec pbi register PBI-001 --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pbiID := args[0]
			return runRegister(pbiID, flags)
		},
	}

	// Define flags
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false, "登録予定のSBI一覧を表示（実際の登録は行わない）")
	cmd.Flags().BoolVar(&flags.force, "force", false, "既に登録済みの場合でも再登録する")

	return cmd
}

func runRegister(pbiID string, flags *registerFlags) error {
	ctx := context.Background()

	// Display progress: loading approval manifest
	fmt.Println("📋 approval.yamlを読み込み中...")

	// Create approval repository to check manifest first
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

	// Load manifest to display approved SBI count
	manifest, err := approvalRepo.LoadManifest(ctx, repository.PBIID(pbiID))
	if err != nil {
		return fmt.Errorf("approval.yamlの読み込みに失敗しました: %w", err)
	}

	// Get approved SBI count
	approvedCount := manifest.ApprovedCount()
	if approvedCount == 0 {
		return fmt.Errorf("承認済みのSBIが見つかりません: PBI %s\n"+
			"ヒント: 'deespec pbi sbi list %s' でSBIを確認し、'deespec pbi sbi approve %s <sbi-file>' で承認してください",
			pbiID, pbiID, pbiID)
	}

	// Display approved SBI count
	fmt.Printf("✅ 承認済みSBI: %d個\n", approvedCount)
	fmt.Println()

	// Display SBI list in dry-run mode
	if flags.dryRun {
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("📄 Dry-run mode: 登録予定のSBI一覧")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println()

		approvedFiles := manifest.GetApprovedSBIs()
		for i, sbiFile := range approvedFiles {
			fmt.Printf("  %d. %s\n", i+1, sbiFile)
		}

		fmt.Println()
		fmt.Printf("合計: %d個のSBIを登録予定\n", len(approvedFiles))
		fmt.Println()
		fmt.Println("💡 実際に登録するには --dry-run フラグを外して実行してください:")
		fmt.Printf("   $ deespec pbi register %s\n", pbiID)
		return nil
	}

	// Open database
	db, err := sql.Open("sqlite3", ".deespec/deespec.db")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Run migrations first
	migrator := sqlite.NewMigrator(db)
	if err := migrator.Migrate(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Create repositories
	rootPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	pbiRepo := persistence.NewPBISQLiteRepository(db, rootPath)
	sbiRepo := sqlite.NewSBIRepository(db)

	// Create use case
	useCase := pbiusecase.NewRegisterSBIsUseCase(sbiRepo, pbiRepo, approvalRepo)

	// Prepare options
	opts := pbiusecase.RegisterSBIsOptions{
		DryRun: flags.dryRun,
		Force:  flags.force,
	}

	// Display progress: registering SBIs
	fmt.Printf("💾 SBIを登録中... (0/%d)\n", approvedCount)

	// Execute use case
	result, err := useCase.Execute(ctx, pbiID, opts)
	if err != nil {
		// Display errors if any (even on total failure)
		if result != nil && len(result.Errors) > 0 {
			fmt.Fprintln(os.Stderr, "\n⚠️  エラー詳細:")
			for i, errMsg := range result.Errors {
				fmt.Fprintf(os.Stderr, "  %d. %s\n", i+1, errMsg)
			}
			fmt.Fprintln(os.Stderr, "")
		}
		return fmt.Errorf("SBIの登録に失敗しました: %w", err)
	}

	// Display completion
	fmt.Printf("\r💾 SBIを登録中... (%d/%d)\n", result.RegisteredCount, approvedCount)
	fmt.Println("✅ 登録完了")
	fmt.Println()

	// Display results
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📊 登録結果")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Printf("✅ 登録成功: %d個\n", result.RegisteredCount)

	if result.SkippedCount > 0 {
		fmt.Printf("⏭  スキップ: %d個\n", result.SkippedCount)
	}

	// Display registered SBI IDs
	if len(result.SBIIDs) > 0 {
		fmt.Println()
		fmt.Println("📝 登録されたSBI ID:")
		for i, sbiID := range result.SBIIDs {
			fmt.Printf("  %d. %s\n", i+1, sbiID)
		}
	}

	// Display errors if any
	if len(result.Errors) > 0 {
		fmt.Println()
		fmt.Println("⚠️  エラー:")
		for i, errMsg := range result.Errors {
			fmt.Printf("  %d. %s\n", i+1, errMsg)
		}
	}

	fmt.Println()

	// Display next steps
	if result.RegisteredCount > 0 {
		fmt.Println("💡 次のステップ:")
		fmt.Println("   登録されたSBIを確認するには:")
		fmt.Println("   $ deespec sbi list")
		fmt.Println()
		fmt.Println("   SBIの実行を開始するには:")
		fmt.Println("   $ deespec run")
	}
	fmt.Println()

	return nil
}
