package pbi

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/YoshitsuguKoike/deespec/internal/adapter/gateway/agent"
	appconfig "github.com/YoshitsuguKoike/deespec/internal/app/config"
	pbiusecase "github.com/YoshitsuguKoike/deespec/internal/application/usecase/pbi"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence/sqlite"
	sqliterepo "github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence/sqlite"
	infrarepo "github.com/YoshitsuguKoike/deespec/internal/infrastructure/repository"
	"github.com/spf13/cobra"
)

// decomposeFlags holds flags for the decompose command
type decomposeFlags struct {
	promptOnly bool // Generate prompt file only, without AI execution
	minSBIs    int
	maxSBIs    int
}

// NewDecomposeCommand creates a new decompose command
func NewDecomposeCommand() *cobra.Command {
	flags := &decomposeFlags{}

	cmd := &cobra.Command{
		Use:   "decompose <pbi-id>",
		Short: "Decompose a PBI into multiple SBIs using AI",
		Long: `Decompose a Product Backlog Item (PBI) into multiple Small Backlog Items (SBIs).

By default, this command:
1. Generates a decomposition prompt with PBI details and label instructions
2. Executes AI agent to create SBI specification files
3. Creates approval.yaml for SBI review

Use --prompt-only to generate the prompt file without AI execution (for manual review).

Only PBIs in "pending" or "planning" status can be decomposed.`,
		Example: `  # Decompose a PBI with AI agent execution (default)
  deespec pbi decompose PBI-001

  # Generate prompt file only (for manual review)
  deespec pbi decompose PBI-001 --prompt-only

  # Specify min/max SBI count
  deespec pbi decompose PBI-001 --min-sbis 3 --max-sbis 7`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pbiID := args[0]
			return runDecompose(pbiID, flags)
		},
	}

	// Define flags
	cmd.Flags().BoolVar(&flags.promptOnly, "prompt-only", false, "Generate prompt file only without AI execution")
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

	// Create agent gateway
	agentGateway := agent.NewClaudeCodeCLIGateway()

	// Create use case
	useCase := pbiusecase.NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, labelRepo, agentGateway)

	// Display progress: retrieving PBI
	fmt.Println("🔄 PBIを取得中...")

	// Prepare options
	opts := pbiusecase.DecomposeOptions{
		MinSBIs:    flags.minSBIs,
		MaxSBIs:    flags.maxSBIs,
		DryRun:     flags.promptOnly, // PromptOnly mode = DryRun (no AI execution)
		OutputOnly: false,
	}

	// Display progress: building prompt
	fmt.Println("📝 プロンプトを構築中...")

	// Execute use case
	result, err := useCase.Execute(ctx, pbiID, opts)
	if err != nil {
		return fmt.Errorf("PBIの分解に失敗しました: %w", err)
	}

	// Display progress: writing prompt file (if not prompt-only)
	if !flags.promptOnly {
		fmt.Println("💾 プロンプトファイルを出力中...")
		fmt.Println("🤖 AIエージェントを実行中...")
	}

	// Display results
	fmt.Println("✅ 完了")
	fmt.Println()

	if flags.promptOnly {
		// Prompt-only mode: display file path and preview
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("📄 Prompt-only mode: プロンプトが正常に生成されました（AI実行なし）")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println()
		fmt.Printf("📁 ファイルパス: %s\n", result.PromptFilePath)
		fmt.Printf("📊 プロンプトサイズ: %d 文字\n", len(result.Prompt))
		fmt.Println()
		fmt.Println("💡 次のステップ:")
		fmt.Println("   1. プロンプトファイルを確認してください")
		fmt.Printf("      $ cat %s\n", result.PromptFilePath)
		fmt.Println()
		fmt.Println("   2. 手動でClaude Code CLIを実行してください:")
		fmt.Printf("      $ claude -p --dangerously-skip-permissions \"$(cat %s)\"\n", result.PromptFilePath)
	} else if result.SBICount > 0 {
		// Success: SBI files were generated
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("📄 SBIファイルが生成されました")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println()
		fmt.Printf("📁 生成されたSBIファイル:\n")
		for _, sbiFile := range result.SBIFiles {
			fmt.Printf("   - %s\n", sbiFile)
		}
		fmt.Println()
		fmt.Println("📋 approval.yaml作成済み")
		fmt.Println()
		fmt.Println("💡 次のステップ:")
		fmt.Println("   1. 生成されたSBIをレビューしてください")
		fmt.Printf("      $ deespec pbi sbi list %s\n", pbiID)
		fmt.Println()
		fmt.Println("   2. 承認してください")
		fmt.Printf("      $ deespec pbi sbi approve %s <sbi-file>\n", pbiID)
		fmt.Println()
		fmt.Println("   3. 登録してください")
		fmt.Printf("      $ deespec pbi register %s\n", pbiID)
	} else {
		// Partial success: prompt created but AI execution failed or no SBIs generated
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("⚠️  プロンプトファイルが生成されました")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println()
		fmt.Printf("📁 ファイルパス: %s\n", result.PromptFilePath)
		fmt.Println()
		fmt.Printf("ℹ️  %s\n", result.Message)
		fmt.Println()
		fmt.Println("💡 次のステップ:")
		fmt.Println("   1. プロンプトファイルを確認してください")
		fmt.Printf("      $ cat %s\n", result.PromptFilePath)
		fmt.Println()
		fmt.Println("   2. 手動でClaude Code CLIを実行してください:")
		fmt.Printf("      $ claude -p --dangerously-skip-permissions \"$(cat %s)\"\n", result.PromptFilePath)
		fmt.Println()
		fmt.Println("   3. または、手動でSBIファイルを作成してください")
	}
	fmt.Println()

	return nil
}
