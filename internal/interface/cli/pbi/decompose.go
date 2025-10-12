package pbi

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	appconfig "github.com/YoshitsuguKoike/deespec/internal/app/config"
	pbiusecase "github.com/YoshitsuguKoike/deespec/internal/application/usecase/pbi"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence/migration"
	sqliterepo "github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence/sqlite"
	infrarepo "github.com/YoshitsuguKoike/deespec/internal/infrastructure/repository"
	"github.com/spf13/cobra"
)

// decomposeFlags holds flags for the decompose command
type decomposeFlags struct {
	dryRun  bool
	minSBIs int
	maxSBIs int
}

// NewDecomposeCommand creates a new decompose command
func NewDecomposeCommand() *cobra.Command {
	flags := &decomposeFlags{}

	cmd := &cobra.Command{
		Use:   "decompose <pbi-id>",
		Short: "Decompose a PBI into multiple SBIs",
		Long: `Decompose a Product Backlog Item (PBI) into multiple Small Backlog Items (SBIs).

This command generates a decomposition prompt for AI agents to create SBI specifications.
The command validates the PBI status and generates a prompt file in the PBI directory.

Only PBIs in "pending" or "planning" status can be decomposed.`,
		Example: `  # Decompose a PBI (generates prompt file)
  deespec pbi decompose PBI-001

  # Dry-run mode (only build prompt, no file output)
  deespec pbi decompose PBI-001 --dry-run

  # Specify min/max SBI count
  deespec pbi decompose PBI-001 --min-sbis 3 --max-sbis 7`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pbiID := args[0]
			return runDecompose(pbiID, flags)
		},
	}

	// Define flags
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false, "プロンプト構築のみ（ファイル出力なし）")
	cmd.Flags().IntVar(&flags.minSBIs, "min-sbis", 2, "最小SBI数（デフォルト: 2）")
	cmd.Flags().IntVar(&flags.maxSBIs, "max-sbis", 10, "最大SBI数（デフォルト: 10）")

	return cmd
}

func runDecompose(pbiID string, flags *decomposeFlags) error {
	ctx := context.Background()

	// Validate flags
	if flags.minSBIs < 1 {
		return fmt.Errorf("--min-sbis must be at least 1, got %d", flags.minSBIs)
	}
	if flags.maxSBIs < flags.minSBIs {
		return fmt.Errorf("--max-sbis (%d) must be greater than or equal to --min-sbis (%d)",
			flags.maxSBIs, flags.minSBIs)
	}
	if flags.maxSBIs > 20 {
		return fmt.Errorf("--max-sbis must be at most 20, got %d", flags.maxSBIs)
	}

	// Open database
	db, err := sql.Open("sqlite3", ".deespec/var/deespec.db")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Run migrations first
	migrator := migration.NewMigrator(db)
	if err := migrator.Migrate(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Create repositories
	rootPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	pbiRepo := persistence.NewPBISQLiteRepository(db, rootPath)
	promptRepo := infrarepo.NewPromptTemplateRepositoryImpl()
	approvalRepo := infrarepo.NewSBIApprovalRepositoryImpl()

	// Create label repository with default config
	labelConfig := appconfig.LabelConfig{
		TemplateDirs: []string{".claude", ".deespec/prompts/labels"},
		Import: appconfig.LabelImportConfig{
			AutoPrefixFromDir: true,
			MaxLineCount:      1000,
			ExcludePatterns:   []string{"*.secret.md", "settings.*.json", "tmp/**"},
		},
		Validation: appconfig.LabelValidationConfig{
			AutoSyncOnMismatch: false,
			WarnOnLargeFiles:   true,
		},
	}
	labelRepo := sqliterepo.NewLabelRepository(db, labelConfig)

	// Create use case
	useCase := pbiusecase.NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, labelRepo)

	// Display progress: retrieving PBI
	fmt.Println("🔄 PBIを取得中...")

	// Prepare options
	opts := pbiusecase.DecomposeOptions{
		MinSBIs:    flags.minSBIs,
		MaxSBIs:    flags.maxSBIs,
		DryRun:     flags.dryRun,
		OutputOnly: false,
	}

	// Display progress: building prompt
	fmt.Println("📝 プロンプトを構築中...")

	// Execute use case
	result, err := useCase.Execute(ctx, pbiID, opts)
	if err != nil {
		return fmt.Errorf("PBIの分解に失敗しました: %w", err)
	}

	// Display progress: writing prompt file (if not dry-run)
	if !flags.dryRun {
		fmt.Println("💾 プロンプトファイルを出力中...")
	}

	// Display results
	fmt.Println("✅ 完了")
	fmt.Println()

	if flags.dryRun {
		// Dry-run mode: display prompt to stdout
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("📄 Dry-run mode: プロンプトが正常に生成されました（ファイル出力なし）")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println()
		fmt.Printf("生成されたプロンプト（%d文字）:\n", len(result.Prompt))
		fmt.Println("────────────────────────────────────────────────────────")
		// Display first 500 characters of the prompt
		if len(result.Prompt) > 500 {
			fmt.Println(result.Prompt[:500])
			fmt.Printf("\n... (残り %d 文字)\n", len(result.Prompt)-500)
		} else {
			fmt.Println(result.Prompt)
		}
		fmt.Println("────────────────────────────────────────────────────────")
	} else {
		// Normal mode: display file path and next steps
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("📄 プロンプトファイルが生成されました")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println()
		fmt.Printf("📁 ファイルパス: %s\n", result.PromptFilePath)
		fmt.Println()
		fmt.Println("💡 次のステップ:")
		fmt.Println("   1. プロンプトファイルを確認してください")
		fmt.Printf("      $ cat %s\n", result.PromptFilePath)
		fmt.Println()
		fmt.Println("   2. 将来的には以下のコマンドでAIエージェントを実行できます:")
		fmt.Printf("      $ deespec pbi ai-decompose %s  # (未実装)\n", pbiID)
		fmt.Println()
		fmt.Println("   3. AIが生成したSBIファイルをレビューして承認してください:")
		fmt.Printf("      $ deespec pbi sbi list %s\n", pbiID)
		fmt.Printf("      $ deespec pbi sbi approve %s <sbi-file>\n", pbiID)
	}
	fmt.Println()

	return nil
}
