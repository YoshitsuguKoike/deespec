# PBI登録フローの設計：技術実装提案

> この文書は技術実装の詳細とAIツール統合の提案を含みます。
> ユーザー体験については [pbi_how_to_work_02.md](./pbi_how_to_work_02.md) を参照してください。

---

## 目次

1. [AIツールがdeespecを認識する方法](#1-aiツールがdeespecを認識する方法)
2. [Agent管理コマンド設計](#2-agent管理コマンド設計)
3. [段階的実装計画](#3-段階的実装計画)
4. [AIツール統合のための設計](#4-aiツール統合のための設計)
5. [ファイルフォーマット詳細](#5-ファイルフォーマット詳細)
6. [実装ロードマップ](#6-実装ロードマップ)

---

## 1. AIツールがdeespecを認識する方法

### 1.1 課題

Claude Code、Cursor、その他のAIツールがdeespecの存在を事前に認識している必要があります。ユーザーが「PBI化してください」と言ったときに、AIツールはdeespecコマンドを使用できる必要があります。

### 1.2 推奨アプローチ：`DEESPEC.md`の作成とユーザー指示

**方針**:
- ❌ CLAUDE.mdなどAIツール固有の機能に依存しない
- ✅ DEESPEC.mdをプロジェクトルートに作成
- ✅ ユーザーが明示的にAIツールに「DEESPEC.mdを読んで」と指示
- ✅ `deespec about`コマンドで動的な情報を提供

**利点**:
- ✅ AIツール（Claude Code、Cursor等）に依存しない汎用的な設計
- ✅ ユーザーの制御下で動作（いつdeespecを使うかユーザーが決定）
- ✅ ユーザー環境を汚さない（特定AIツールの設定ファイルを作らない）
- ✅ 明示的でわかりやすい

**実装方法**:
```bash
# deespec init 実行時に DEESPEC.md を生成
```

**`DEESPEC.md`の例**:
```markdown
# deespec - AI-Driven Agile Development Tool

このプロジェクトは **deespec** を使用しています。

## 概要

deespecは、Product Backlog Item (PBI) と Small Backlog Item (SBI) を管理し、AIエージェントによる自動実装をサポートするツールです。

## 使い方

### 1. 情報取得

```bash
# deespecの詳細情報を取得
deespec about
```

### 2. PBI化ワークフロー

#### ステップ1: 計画ドキュメントを作成

`docs/`配下にMarkdownで計画を作成します。

```markdown
# docs/feature-plan.md

## 目的
新しい認証機能を追加する

## 受け入れ基準
- [ ] OAuth 2.0が動作する
- [ ] セキュリティテストに合格する
```

#### ステップ2: PBI化

AIツールに以下のように依頼します：

```
「docs/feature-plan.md をPBI化してください」
```

AIツールは以下のコマンドを実行します：

```bash
deespec pbi create-from-doc docs/feature-plan.md --register
```

#### ステップ3: 確認

```bash
# PBI一覧
deespec pbi list

# PBI詳細
deespec pbi show PBI-001
```

## 利用可能なコマンド

### PBI管理

| コマンド | 説明 |
|---------|------|
| `deespec pbi list` | PBI一覧表示 |
| `deespec pbi show <id>` | PBI詳細表示 |
| `deespec pbi create-from-doc <path> --register` | ドキュメントからPBI作成・登録 |
| `deespec pbi register -f <file>` | YAMLファイルからPBI登録 |

### SBI管理

| コマンド | 説明 |
|---------|------|
| `deespec sbi list` | SBI一覧表示 |
| `deespec sbi history <id>` | SBI実行履歴 |
| `deespec sbi run <id>` | SBI実行 |

### Agent管理

| コマンド | 説明 |
|---------|------|
| `deespec agent list` | エージェント一覧 |
| `deespec agent set <name> <cmd>` | エージェント登録 |

### 情報取得

| コマンド | 説明 |
|---------|------|
| `deespec about` | deespecの詳細情報表示 |
| `deespec version` | バージョン表示 |

## ディレクトリ構造

```
project/
├── docs/                    # 計画ドキュメント（人間が管理）
│   └── *.md
├── .deespec/                # deespecの内部状態（自動管理）
│   ├── specs/pbi/          # PBI定義
│   ├── specs/sbi/          # SBI定義
│   ├── var/journal.ndjson  # 実行履歴
│   └── prompts/            # プロンプトテンプレート
└── DEESPEC.md              # このファイル
```

**重要**: `.deespec/`配下は直接触る必要はありません。すべてdeespecコマンドで管理されます。

## AIツールとの連携

### 初回利用時

```
User: 「DEESPEC.mdを読んで、deespecの使い方を把握してください」

AIツール:
（DEESPEC.mdを読み込み、deespecの使い方を理解）
「了解しました。deespecを使ってPBI管理ができますね」
```

### PBI化の依頼

```
User: 「docs/plan.md をPBI化してください」

AIツール:
1. docs/plan.md を読み取り
2. deespec pbi create-from-doc docs/plan.md --register を実行
3. 結果を報告
```

### フォールバック（コマンドが未実装の場合）

`deespec pbi create-from-doc`が存在しない場合、AIツールは以下を実行：

1. ドキュメントを読み取り
2. PBI YAMLファイルを生成（.deespec/specs/pbi/PBI-XXX.yaml）
3. `deespec pbi register -f .deespec/specs/pbi/PBI-XXX.yaml` を実行

## 詳細ドキュメント

- [PBI作成方法](./docs/pbi_how_to_work.md)
- [ユーザー体験設計](./docs/pbi_how_to_work_02.md)
- [技術実装提案](./docs/pbi_how_to_work_03.md)

## フィロソフィー

**Invisible Infrastructure**: ユーザーは`.deespec/`を意識しない

- 計画は`docs/`に普通にMarkdownで書く
- AIツールに「PBI化してください」と言うだけ
- あとはdeespecが自動で管理

最高のツールは存在を意識させません。
```

**利点**:
- ✅ AIツール非依存（Claude Code、Cursor、その他でも動作）
- ✅ ユーザーの制御下（明示的に読ませる）
- ✅ シンプルで明確
- ✅ 環境を汚さない

**使用フロー**:
```
1. deespec init → DEESPEC.md生成
2. ユーザー: 「DEESPEC.mdを読んで」とAIツールに指示
3. AIツール: DEESPEC.mdを読み込み、deespecの使い方を把握
4. ユーザー: 「PBI化してください」
5. AIツール: deespecコマンドを実行
```

---

### 1.3 実装方法

#### Phase 1: DEESPEC.md生成（必須）

```go
// internal/interface/cli/init/deespec_md_setup.go

func SetupDeespecMD(projectRoot string) error {
    deespecMDPath := filepath.Join(projectRoot, "DEESPEC.md")

    content := `# deespec - AI-Driven Agile Development Tool

このプロジェクトは **deespec** を使用しています。

## 概要

deespecは、Product Backlog Item (PBI) と Small Backlog Item (SBI) を管理し、
AIエージェントによる自動実装をサポートするツールです。

## 使い方

### 1. 情報取得

` + "```bash" + `
# deespecの詳細情報を取得
deespec about
` + "```" + `

### 2. PBI化ワークフロー

#### ステップ1: 計画ドキュメントを作成

` + "`docs/`" + `配下にMarkdownで計画を作成します。

#### ステップ2: PBI化

AIツールに以下のように依頼します：

` + "```" + `
「docs/feature-plan.md をPBI化してください」
` + "```" + `

AIツールは以下のコマンドを実行します：

` + "```bash" + `
deespec pbi create-from-doc docs/feature-plan.md --register
` + "```" + `

#### ステップ3: 確認

` + "```bash" + `
# PBI一覧
deespec pbi list

# PBI詳細
deespec pbi show PBI-001
` + "```" + `

## 利用可能なコマンド

### PBI管理
- ` + "`deespec pbi list`" + ` - PBI一覧表示
- ` + "`deespec pbi show <id>`" + ` - PBI詳細表示
- ` + "`deespec pbi create-from-doc <path> --register`" + ` - ドキュメントからPBI作成・登録

### SBI管理
- ` + "`deespec sbi list`" + ` - SBI一覧表示
- ` + "`deespec sbi history <id>`" + ` - SBI実行履歴

### 情報取得
- ` + "`deespec about`" + ` - deespecの詳細情報表示
- ` + "`deespec version`" + ` - バージョン表示

## AIツールとの連携

### 初回利用時

` + "```" + `
User: 「DEESPEC.mdを読んで、deespecの使い方を把握してください」

AIツール:
（DEESPEC.mdを読み込み、deespecの使い方を理解）
「了解しました。deespecを使ってPBI管理ができますね」
` + "```" + `

### PBI化の依頼

` + "```" + `
User: 「docs/plan.md をPBI化してください」

AIツール:
1. docs/plan.md を読み取り
2. deespec pbi create-from-doc docs/plan.md --register を実行
3. 結果を報告
` + "```" + `

## フィロソフィー

**Invisible Infrastructure**: ユーザーは` + "`.deespec/`" + `を意識しない

- 計画は` + "`docs/`" + `に普通にMarkdownで書く
- AIツールに「PBI化してください」と言うだけ
- あとはdeespecが自動で管理

最高のツールは存在を意識させません。

詳細: [docs/pbi_how_to_work.md](docs/pbi_how_to_work.md)
`

    // DEESPEC.mdを作成（上書きしない）
    if fileExists(deespecMDPath) {
        return fmt.Errorf("DEESPEC.md already exists")
    }

    return os.WriteFile(deespecMDPath, []byte(content), 0644)
}
```

#### Phase 1: `deespec about`コマンド（必須）

```go
// internal/interface/cli/about/about.go

func NewCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "about",
        Short: "Show information about deespec",
        Long:  "Display detailed information about deespec installation and configuration",
        Run: func(cmd *cobra.Command, args []string) {
            showAbout()
        },
    }

    return cmd
}

func showAbout() {
    // バージョン情報
    fmt.Println("deespec - AI-Driven Agile Development Tool")
    fmt.Println()
    fmt.Printf("Version: %s\n", version.Version)
    fmt.Printf("Build: %s\n", version.BuildDate)
    fmt.Println()

    // プロジェクト情報
    projectRoot, err := findProjectRoot()
    if err != nil {
        fmt.Printf("Project: Not in a deespec project\n")
        fmt.Println()
        fmt.Println("Run 'deespec init' to initialize a deespec project")
        return
    }

    fmt.Printf("Project: %s\n", projectRoot)
    fmt.Println()

    // 統計情報
    stats, err := getProjectStats(projectRoot)
    if err == nil {
        fmt.Println("## Statistics")
        fmt.Printf("  PBIs: %d\n", stats.TotalPBIs)
        fmt.Printf("  SBIs: %d\n", stats.TotalSBIs)
        fmt.Printf("  Agents: %d\n", stats.TotalAgents)
        fmt.Println()
    }

    // 利用可能なコマンド
    fmt.Println("## Available Commands")
    fmt.Println()
    fmt.Println("### PBI Management")
    fmt.Println("  deespec pbi list                      # List all PBIs")
    fmt.Println("  deespec pbi show <id>                 # Show PBI details")
    fmt.Println("  deespec pbi create-from-doc <path>    # Create PBI from document")
    fmt.Println()
    fmt.Println("### SBI Management")
    fmt.Println("  deespec sbi list                      # List all SBIs")
    fmt.Println("  deespec sbi run <id>                  # Run SBI")
    fmt.Println("  deespec sbi history <id>              # Show SBI history")
    fmt.Println()
    fmt.Println("### Agent Management")
    fmt.Println("  deespec agent list                    # List agents")
    fmt.Println("  deespec agent set <name> <command>    # Register agent")
    fmt.Println()
    fmt.Println("### Information")
    fmt.Println("  deespec about                         # This command")
    fmt.Println("  deespec version                       # Show version")
    fmt.Println()

    // ドキュメント
    fmt.Println("## Documentation")
    deespecMD := filepath.Join(projectRoot, "DEESPEC.md")
    if fileExists(deespecMD) {
        fmt.Printf("  DEESPEC.md: %s\n", deespecMD)
    }
    fmt.Printf("  Project docs: %s/docs/\n", projectRoot)
    fmt.Println()

    // 次のステップ
    fmt.Println("## Getting Started with AI Tools")
    fmt.Println()
    fmt.Println("Tell your AI tool:")
    fmt.Println(`  "DEESPEC.mdを読んで、deespecの使い方を把握してください"`)
    fmt.Println()
    fmt.Println("Then you can ask:")
    fmt.Println(`  "docs/my-plan.md をPBI化してください"`)
}

type ProjectStats struct {
    TotalPBIs   int
    TotalSBIs   int
    TotalAgents int
}

func getProjectStats(projectRoot string) (*ProjectStats, error) {
    stats := &ProjectStats{}

    // PBI数をカウント
    pbiDir := filepath.Join(projectRoot, ".deespec", "specs", "pbi")
    if fileExists(pbiDir) {
        files, _ := os.ReadDir(pbiDir)
        stats.TotalPBIs = len(files)
    }

    // SBI数をカウント
    sbiDir := filepath.Join(projectRoot, ".deespec", "specs", "sbi")
    if fileExists(sbiDir) {
        files, _ := os.ReadDir(sbiDir)
        stats.TotalSBIs = len(files)
    }

    // Agent数をカウント
    agentConfig, err := LoadAgentConfig()
    if err == nil {
        stats.TotalAgents = len(agentConfig.Agents)
    }

    return stats, nil
}
```

---

## 2. Agent管理コマンド設計

### 2.1 現状と課題

**現在の暫定実装**:
- `agent_bin`にclaudeを直接登録
- ハードコーディングされた設定

**課題**:
- エージェント追加・削除が柔軟にできない
- 複数のAIエージェント（Claude, GPT-4, Geminiなど）をサポートしにくい
- PBI YAMLの`assigned_agent`フィールドとの連携が不明確

---

### 2.2 Agent管理コマンド仕様

#### コマンド一覧

```bash
# エージェント一覧表示
deespec agent list

# エージェント登録
deespec agent set <name> <command> [options]

# エージェント削除
deespec agent remove <name>

# デフォルトエージェント設定
deespec agent default <name>

# エージェント動作確認
deespec agent test <name>

# エージェント詳細表示
deespec agent show <name>
```

---

#### `deespec agent list`

**機能**: 登録済みエージェント一覧を表示

**出力例**:
```bash
$ deespec agent list

登録済みエージェント:

  claude-code * (デフォルト)
    Command: claude
    Type: interactive
    Status: Available ✓

  openai-gpt4
    Command: openai-chat --model gpt-4
    Type: api
    Status: Available ✓

  local-llm
    Command: ollama run codellama
    Type: local
    Status: Not found ✗

* = デフォルトエージェント
```

**実装**:
```go
// internal/interface/cli/agent/list.go

func listAgents(cfg *AgentConfig) {
    for _, agent := range cfg.Agents {
        isDefault := agent.Name == cfg.DefaultAgent
        status := checkAgentAvailability(agent.Command)

        fmt.Printf("%s %s\n", agent.Name, defaultMarker(isDefault))
        fmt.Printf("  Command: %s\n", agent.Command)
        fmt.Printf("  Type: %s\n", agent.Type)
        fmt.Printf("  Status: %s\n", status)
        fmt.Println()
    }
}
```

---

#### `deespec agent set <name> <command> [options]`

**機能**: 新しいエージェントを登録または更新

**引数**:
- `<name>`: エージェント名（例: `claude-code`, `gpt-4`）
- `<command>`: 実行するコマンド（例: `claude`, `openai-chat --model gpt-4`）

**オプション**:
- `--type <type>`: エージェントタイプ（`interactive`, `api`, `local`）
- `--default`: このエージェントをデフォルトに設定
- `--env <key=value>`: 環境変数を設定（複数指定可）

**使用例**:
```bash
# Claude Codeを登録
$ deespec agent set claude-code claude --type interactive --default

# OpenAI GPT-4を登録
$ deespec agent set openai-gpt4 "openai-chat --model gpt-4" \
    --type api \
    --env OPENAI_API_KEY=sk-xxx

# ローカルLLMを登録
$ deespec agent set local-llm "ollama run codellama" --type local
```

**実装**:
```go
// internal/interface/cli/agent/set.go

func setAgent(cfg *AgentConfig, name, command string, opts AgentOptions) error {
    agent := Agent{
        Name:    name,
        Command: command,
        Type:    opts.Type,
        Env:     opts.Env,
    }

    // 既存エージェントを更新または追加
    updated := false
    for i, a := range cfg.Agents {
        if a.Name == name {
            cfg.Agents[i] = agent
            updated = true
            break
        }
    }
    if !updated {
        cfg.Agents = append(cfg.Agents, agent)
    }

    // デフォルト設定
    if opts.SetDefault {
        cfg.DefaultAgent = name
    }

    // 設定ファイルに保存
    return cfg.Save()
}
```

---

#### `deespec agent remove <name>`

**機能**: エージェントを削除

**使用例**:
```bash
$ deespec agent remove local-llm

✓ エージェント 'local-llm' を削除しました
```

**実装**:
```go
// internal/interface/cli/agent/remove.go

func removeAgent(cfg *AgentConfig, name string) error {
    // デフォルトエージェントは削除不可
    if cfg.DefaultAgent == name {
        return fmt.Errorf("デフォルトエージェントは削除できません。先に別のエージェントをデフォルトに設定してください")
    }

    // エージェントを削除
    for i, a := range cfg.Agents {
        if a.Name == name {
            cfg.Agents = append(cfg.Agents[:i], cfg.Agents[i+1:]...)
            return cfg.Save()
        }
    }

    return fmt.Errorf("エージェント '%s' が見つかりません", name)
}
```

---

#### `deespec agent default <name>`

**機能**: デフォルトエージェントを設定

**使用例**:
```bash
$ deespec agent default openai-gpt4

✓ デフォルトエージェントを 'openai-gpt4' に設定しました
```

---

#### `deespec agent test <name>`

**機能**: エージェントの動作確認

**使用例**:
```bash
$ deespec agent test claude-code

Testing agent 'claude-code'...
Command: claude
Status: ✓ Available

Test execution:
$ claude --version
Claude Code v1.2.3

✓ Agent is working correctly
```

**実装**:
```go
// internal/interface/cli/agent/test.go

func testAgent(agent Agent) error {
    // コマンドが実行可能かチェック
    cmd := exec.Command("which", strings.Split(agent.Command, " ")[0])
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("コマンドが見つかりません: %s", agent.Command)
    }

    // 簡単なテスト実行（--version など）
    testCmd := exec.Command(strings.Split(agent.Command, " ")[0], "--version")
    output, err := testCmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("コマンド実行エラー: %w", err)
    }

    fmt.Printf("✓ Agent is working correctly\n")
    fmt.Printf("Version: %s\n", string(output))

    return nil
}
```

---

### 2.3 Agent設定ファイル

**`.deespec/config/agents.yaml`**:

```yaml
# Agent設定

version: "1.0"

# デフォルトエージェント
default_agent: claude-code

# 登録済みエージェント
agents:
  - name: claude-code
    command: claude
    type: interactive
    description: "Claude Code CLI"
    env: {}

  - name: openai-gpt4
    command: openai-chat --model gpt-4
    type: api
    description: "OpenAI GPT-4"
    env:
      OPENAI_API_KEY: sk-xxxxx

  - name: local-llm
    command: ollama run codellama
    type: local
    description: "Local Code Llama via Ollama"
    env: {}
```

**Go構造体**:

```go
// internal/domain/model/agent/agent.go

type AgentConfig struct {
    Version      string  `yaml:"version"`
    DefaultAgent string  `yaml:"default_agent"`
    Agents       []Agent `yaml:"agents"`
}

type Agent struct {
    Name        string            `yaml:"name"`
    Command     string            `yaml:"command"`
    Type        string            `yaml:"type"` // interactive, api, local
    Description string            `yaml:"description"`
    Env         map[string]string `yaml:"env"`
}

func (c *AgentConfig) Save() error {
    configPath := ".deespec/config/agents.yaml"
    data, err := yaml.Marshal(c)
    if err != nil {
        return err
    }
    return os.WriteFile(configPath, data, 0644)
}

func LoadAgentConfig() (*AgentConfig, error) {
    configPath := ".deespec/config/agents.yaml"
    data, err := os.ReadFile(configPath)
    if err != nil {
        // デフォルト設定を返す
        return &AgentConfig{
            Version:      "1.0",
            DefaultAgent: "claude-code",
            Agents: []Agent{
                {
                    Name:    "claude-code",
                    Command: "claude",
                    Type:    "interactive",
                },
            },
        }, nil
    }

    var config AgentConfig
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, err
    }

    return &config, nil
}
```

---

### 2.4 PBIワークフローへの影響

#### PBI YAMLの`assigned_agent`フィールド

```yaml
# .deespec/specs/pbi/PBI-001.yaml

id: PBI-001
title: "テストカバレッジ50%達成"
assigned_agent: claude-code  # ← 登録済みエージェント名を参照
```

#### `deespec pbi run`コマンドの動作

```bash
$ deespec pbi run PBI-001
```

**内部動作**:

```go
// internal/application/usecase/pbi/run_pbi_use_case.go

func (u *RunPBIUseCase) Execute(pbiID string) error {
    // 1. PBIを取得
    pbi, err := u.pbiRepo.FindByID(pbiID)
    if err != nil {
        return err
    }

    // 2. Agent設定を読み込み
    agentConfig, err := LoadAgentConfig()
    if err != nil {
        return err
    }

    // 3. 割り当てられたエージェントを取得
    agentName := pbi.AssignedAgent
    if agentName == "" {
        agentName = agentConfig.DefaultAgent
    }

    agent := agentConfig.FindAgent(agentName)
    if agent == nil {
        return fmt.Errorf("エージェント '%s' が見つかりません。`deespec agent list` で確認してください", agentName)
    }

    // 4. エージェントコマンドを実行
    return u.executeAgent(agent, pbi)
}

func (u *RunPBIUseCase) executeAgent(agent *Agent, pbi *PBI) error {
    // プロンプトを準備
    prompt := u.generatePrompt(pbi)

    // エージェントタイプに応じた実行
    switch agent.Type {
    case "interactive":
        return u.executeInteractive(agent, prompt)
    case "api":
        return u.executeAPI(agent, prompt)
    case "local":
        return u.executeLocal(agent, prompt)
    default:
        return fmt.Errorf("未知のエージェントタイプ: %s", agent.Type)
    }
}
```

---

### 2.5 移行戦略：現在の暫定実装から新コマンドへ

#### Phase 1: 互換性維持

```go
// internal/interface/cli/init/init.go

func initDeespec() error {
    // 既存の処理...

    // Agent設定の初期化
    if err := initAgentConfig(); err != nil {
        return err
    }

    return nil
}

func initAgentConfig() error {
    config := &AgentConfig{
        Version:      "1.0",
        DefaultAgent: "claude-code",
        Agents: []Agent{
            {
                Name:        "claude-code",
                Command:     "claude",  // 現在のagent_binと同じ
                Type:        "interactive",
                Description: "Claude Code CLI",
            },
        },
    }

    return config.Save()
}
```

#### Phase 2: マイグレーション

```bash
# 既存のagent_bin設定から agents.yaml へ移行
$ deespec migrate agents

✓ agent_bin 設定を agents.yaml に移行しました
✓ デフォルトエージェント: claude-code
```

---

## 3. 段階的実装計画

### Phase 1: 基本的なPBI化フロー（最優先、1-2週間）

#### 3.1 ユーザーの操作

```markdown
User: "docs/test-coverage-plan.md をPBI化してください"
```

#### 3.2 AIツールの内部動作

```
1. docs/test-coverage-plan.md を読み取り
2. 内容を解析
   - タイトルを抽出（最初の # 見出し）
   - 説明を抽出（## 目的、## 背景など）
   - 受け入れ基準を抽出（## 受け入れ基準、チェックリスト）
3. .deespec/specs/pbi/PBI-XXX.yaml を生成
4. `deespec pbi register -f .deespec/specs/pbi/PBI-XXX.yaml` を実行
5. 結果をユーザーに報告
```

#### 3.3 実装すべきコマンド

**Option A: 明示的なファイル指定**

```bash
# 基本的な登録（Phase 1で実装）
deespec pbi register -f .deespec/specs/pbi/PBI-001.yaml
```

**Option B: docsから直接生成（推奨、Phase 1-2で実装）**

```bash
# ドキュメントから直接PBI生成
deespec pbi create-from-doc docs/test-coverage-plan.md

# 自動登録まで実行
deespec pbi create-from-doc docs/test-coverage-plan.md --register

# IDを指定
deespec pbi create-from-doc docs/test-coverage-plan.md \
  --id PBI-001 \
  --register

# 出力先を指定（デバッグ用）
deespec pbi create-from-doc docs/test-coverage-plan.md \
  --output .deespec/specs/pbi/PBI-001.yaml
```

#### 3.4 実装項目

- [ ] **PBI登録コマンド**
  ```bash
  deespec pbi register -f <file>
  ```

- [ ] **PBI表示コマンド**
  ```bash
  deespec pbi show <id>
  deespec pbi list
  deespec pbi list --status IMPLEMENTING
  ```

- [ ] **ドキュメントからPBI生成（シンプルパーサー）**
  ```bash
  deespec pbi create-from-doc <doc-path> --register
  ```

- [ ] **AIツール統合**
  - `deespec init`時にDEESPEC.md生成
  - `deespec about`コマンド実装
  - フォールバック実装（create-from-docが未実装の場合の手動処理）

---

### Phase 2: Agent管理とインタラクティブ機能（2-3週間）

#### 実装項目

- [ ] **Agent管理コマンド**
  ```bash
  deespec agent list
  deespec agent set <name> <command>
  deespec agent remove <name>
  deespec agent default <name>
  deespec agent test <name>
  ```

- [ ] **Agent設定ファイル**
  - `.deespec/config/agents.yaml` の読み書き
  - デフォルトエージェント管理

- [ ] **LLM統合パーサー**
  ```bash
  deespec pbi create-from-doc <doc-path> --use-llm
  ```

- [ ] **インタラクティブモード**
  ```bash
  deespec pbi create --interactive
  ```

---

### Phase 3: 高度な機能（1-2ヶ月）

#### 実装項目

- [ ] **外部サービス連携**
  ```bash
  deespec pbi import-github --issue 123
  deespec pbi import-jira --ticket PROJ-456
  ```

- [ ] **双方向sync**
  ```bash
  deespec pbi sync --github
  ```

- [ ] **Webhook連携**
  ```bash
  deespec server --port 8080
  ```

---

## 4. AIツール統合のための設計

### 4.1 AIツールが実行するコマンドパターン

#### Pattern 1: ドキュメントからPBI生成（推奨）

```bash
# ワンステップで実行
deespec pbi create-from-doc docs/plan.md --register

# 詳細指定
deespec pbi create-from-doc docs/plan.md \
  --id PBI-001 \
  --story-points 5 \
  --priority 1 \
  --register
```

#### Pattern 2: 段階的実行（デバッグ用）

```bash
# 1. ドラフト生成
deespec pbi create-from-doc docs/plan.md \
  --output .deespec/specs/pbi/PBI-001.yaml

# 2. YAMLを確認・編集（AIツールが内容チェック）

# 3. 登録
deespec pbi register -f .deespec/specs/pbi/PBI-001.yaml
```

#### Pattern 3: 完全手動（Phase 1のフォールバック）

```bash
# 1. AIツールがYAMLファイルを生成（Writeツール使用）

# 2. deespecコマンドで登録
deespec pbi register -f .deespec/specs/pbi/PBI-001.yaml

# 3. 確認
deespec pbi show PBI-001
```

---

### 4.2 deespec側で提供すべき機能

#### コマンド仕様

```bash
deespec pbi create-from-doc <doc-path> [options]

Options:
  --id <id>              PBI ID（省略時は自動生成）
  --story-points <num>   ストーリーポイント
  --priority <0|1|2>     優先度（0=通常, 1=高, 2=緊急）
  --labels <label,...>   ラベル（カンマ区切り）
  --output <path>        出力先YAML（省略時は自動）
  --register             生成後に自動登録
  --draft                登録せずにYAMLを標準出力に出力
  --format <yaml|json>   出力フォーマット
  --use-llm              LLMパーサーを使用（APIキー必要）

Examples:
  # 基本的な使い方
  deespec pbi create-from-doc docs/plan.md --register

  # 詳細指定
  deespec pbi create-from-doc docs/plan.md \
    --id PBI-CUSTOM-001 \
    --story-points 8 \
    --priority 1 \
    --labels "testing,ci-fix" \
    --register

  # ドラフト確認
  deespec pbi create-from-doc docs/plan.md --draft
```

---

### 4.3 実装の階層

#### Phase 1: シンプルなMarkdownパーサー（LLM不要）

```go
// internal/interface/cli/pbi/create_from_doc.go

func createFromDocSimple(docPath string) (*PBI, error) {
    content, err := os.ReadFile(docPath)
    if err != nil {
        return nil, err
    }

    // Markdownの構造を解析
    parser := NewMarkdownParser(string(content))

    return &PBI{
        ID:          generatePBIID(),
        Title:       parser.ExtractTitle(),        // 最初の # 見出し
        Description: parser.ExtractDescription(),  // ## 目的 or ## 背景
        AcceptanceCriteria: parser.ExtractCriteria(), // ## 受け入れ基準
        SourceDocument: docPath,
        GeneratedFrom: "markdown-parser",
        CreatedAt: time.Now(),
    }, nil
}

// 簡易Markdownパーサー
type MarkdownParser struct {
    content string
    lines   []string
}

func (p *MarkdownParser) ExtractTitle() string {
    // 最初の # 見出しを取得
    for _, line := range p.lines {
        if strings.HasPrefix(line, "# ") {
            return strings.TrimPrefix(line, "# ")
        }
    }
    return ""
}

func (p *MarkdownParser) ExtractDescription() string {
    // ## 目的、## 背景、## Description などを結合
    sections := []string{}
    inSection := false
    currentSection := ""

    for _, line := range p.lines {
        if strings.HasPrefix(line, "## ") {
            header := strings.TrimPrefix(line, "## ")
            if contains([]string{"目的", "背景", "Description", "概要"}, header) {
                inSection = true
                currentSection = ""
            } else {
                if currentSection != "" {
                    sections = append(sections, currentSection)
                }
                inSection = false
            }
        } else if inSection {
            currentSection += line + "\n"
        }
    }

    return strings.Join(sections, "\n\n")
}

func (p *MarkdownParser) ExtractCriteria() []AcceptanceCriterion {
    // ## 受け入れ基準 セクションのチェックリストを取得
    criteria := []AcceptanceCriterion{}
    inSection := false

    for _, line := range p.lines {
        if strings.HasPrefix(line, "## 受け入れ基準") ||
           strings.HasPrefix(line, "## Acceptance Criteria") {
            inSection = true
        } else if strings.HasPrefix(line, "## ") {
            inSection = false
        } else if inSection && strings.HasPrefix(line, "- [ ] ") {
            criteria = append(criteria, AcceptanceCriterion{
                Condition: strings.TrimPrefix(line, "- [ ] "),
                Status: "pending",
            })
        }
    }

    return criteria
}
```

---

#### Phase 2: LLM統合パーサー（高精度）

```go
// internal/interface/cli/pbi/create_from_doc_llm.go

func createFromDocWithLLM(docPath string) (*PBI, error) {
    content, err := os.ReadFile(docPath)
    if err != nil {
        return nil, err
    }

    // LLM APIを呼び出し（環境変数でAPIキー取得）
    prompt := fmt.Sprintf(`
あなたはプロジェクト管理のエキスパートです。
以下のドキュメントからPBI（Product Backlog Item）を構造化してください。

ドキュメント:
---
%s
---

以下のYAMLフォーマットで出力してください:

id: （自動生成されるのでnullで良い）
title: "ドキュメントから抽出したタイトル"
description: |
  ドキュメントの内容を要約した説明

acceptance_criteria:
  - condition: "条件1"
    expected: "期待値"
  - condition: "条件2"
    expected: "期待値"

estimated_story_points: 見積もり（1-13のフィボナッチ数）
priority: 優先度（0=通常, 1=高, 2=緊急）
labels: [抽出したラベル]

出力はYAMLのみで、説明文は不要です。
`, string(content))

    // LLM API呼び出し（Claude API、OpenAI API、またはローカルLLM）
    response, err := callLLMAPI(prompt)
    if err != nil {
        return nil, fmt.Errorf("LLM API call failed: %w", err)
    }

    // YAMLをパース
    var pbi PBI
    if err := yaml.Unmarshal([]byte(response), &pbi); err != nil {
        return nil, fmt.Errorf("failed to parse LLM response: %w", err)
    }

    // メタデータを追加
    pbi.SourceDocument = docPath
    pbi.GeneratedFrom = "llm-parser"
    pbi.CreatedAt = time.Now()

    return &pbi, nil
}
```

---

## 5. ファイルフォーマット詳細

### 5.1 PBI YAMLスキーマ

```yaml
# .deespec/specs/pbi/PBI-001.yaml

# === メタデータ ===
version: "1.0"  # スキーマバージョン
type: pbi

# === 基本情報 ===
id: PBI-001
title: "テストカバレッジ50%達成"
description: |
  CIの要件である50%カバレッジを達成するため、
  ドメインモデル、Repository、Application層のテストを追加する。

# === ステータス ===
status: PENDING  # PENDING | PICKED | IMPLEMENTING | REVIEWING | DONE | FAILED
current_step: PICK  # PICK | IMPLEMENT | REVIEW | DONE

# === 見積もりと優先度 ===
estimated_story_points: 8
priority: 1  # 0=通常, 1=高, 2=緊急
labels:
  - testing
  - ci-fix
  - technical-debt

# === 担当 ===
assigned_agent: claude-code  # Agent管理コマンドで登録されたエージェント名

# === 階層構造 ===
parent_epic: null  # EPICのIDまたはnull
child_sbis:
  - TEST-COV-SBI-001
  - TEST-COV-SBI-002
  - TEST-COV-SBI-003

# === 受け入れ基準 ===
acceptance_criteria:
  - condition: "Coverage"
    expected: ">= 50%"
    current: "34.9%"
    status: pending  # pending | passed | failed
    measurement_command: "go test -cover ./..."

  - condition: "CI"
    expected: "passing"
    status: pending
    verification: "GitHub Actions must be green"

# === 関連ドキュメント ===
source_document: ../../docs/test-coverage-plan.md
related_documents:
  - ../../docs/architecture.md

# === 生成情報 ===
generated_from: "claude-code conversation on 2025-10-11"
generation_method: "markdown-parser"  # markdown-parser | llm-parser | manual

# === タイムスタンプ ===
created_at: "2025-10-11T05:00:00Z"
updated_at: "2025-10-11T05:05:00Z"
```

---

## 6. 実装ロードマップ

### Phase 1: 基本機能（最優先、1-2週間）

#### 実装項目

- [ ] **PBI登録コマンド**
  - YAMLファイルからPBIを登録
  - `.deespec/specs/pbi/`配下に保存
  - journal.ndjsonに記録

- [ ] **PBI表示コマンド**
  - `deespec pbi show <id>`
  - `deespec pbi list`
  - `deespec pbi list --status IMPLEMENTING`

- [ ] **ドキュメントからPBI生成（シンプルパーサー）**
  - Markdownの構造解析
  - 基本的な情報抽出
  - YAML生成

- [ ] **AIツール統合**
  - `deespec init`時にDEESPEC.md生成
  - `deespec about`コマンド実装
  - プロジェクト統計情報の取得機能

#### 検証

```bash
# テストシナリオ

# 1. プロジェクト初期化
deespec init
# → DEESPEC.md が生成される

# 2. 情報確認
deespec about
# → deespecの情報が表示される

# 3. AIツールに指示
「DEESPEC.mdを読んで、deespecの使い方を把握してください」

# 4. 計画ドキュメント作成
docs/test-plan.md を作成

# 5. AIツールにPBI化を依頼
「これをPBI化してください」

# 6. AIツールがPBIを登録
# → deespec pbi create-from-doc docs/test-plan.md --register を実行

# 7. 確認
deespec pbi show PBI-001

# Success!
```

---

### Phase 2: Agent管理と利便性向上（2-3週間）

#### 実装項目

- [ ] **Agent管理コマンド**
  - `deespec agent list/set/remove/default/test`
  - `.deespec/config/agents.yaml`管理

- [ ] **LLM統合パーサー**
  - Claude API連携
  - 高精度な情報抽出

- [ ] **インタラクティブモード**
  - ウィザード形式での入力

- [ ] **PBI更新・編集**
  - `deespec pbi update <id> --status IMPLEMENTING`
  - `deespec pbi edit <id>`

---

### Phase 3: 高度な機能（1-2ヶ月）

#### 実装項目

- [ ] **外部サービス連携**
  - GitHub Issue/PR連携
  - Jira連携

- [ ] **双方向sync**
  - PBI状態変更 → GitHubに反映

- [ ] **Webhook連携**
  - 自動PBI化

---

## まとめ

### 推奨実装順序

1. **Phase 1 (Week 1-2)**:
   - DEESPEC.md生成（AIツール認識）
   - `deespec about`コマンド（情報提供）
   - PBI基本コマンド（register, show, list）
   - シンプルMarkdownパーサー

2. **Phase 2 (Week 3-5)**:
   - Agent管理コマンド実装
   - 暫定実装からの移行
   - LLM統合パーサー（オプション）

3. **Phase 3 (Month 2-3)**:
   - 外部連携機能
   - 高度な自動化

### 重要な設計判断

✅ **DEESPEC.md生成**: Phase 1で必須実装（AIツール認識のため）
✅ **`deespec about`コマンド**: Phase 1で必須実装（動的情報提供のため）
✅ **Agent管理コマンド**: Phase 2で実装（柔軟性と拡張性のため）
✅ **AIツール非依存**: Claude Code、Cursor等、どのAIツールでも動作
✅ **ユーザー主導**: ユーザーが明示的にDEESPEC.mdを読ませる

### 認知フロー

```
Phase 1: 静的情報
  DEESPEC.md（ユーザーが明示的に読ませる）
  ↓
  AIツールがdeespecの使い方を理解

Phase 2: 動的情報
  deespec about（AIツールがコマンド実行）
  ↓
  バージョン、統計、設定情報を取得
```

これにより、ユーザーはdeespecの内部を意識せず、どのAIツールとでも自然な対話でPBIを管理できるようになります。
