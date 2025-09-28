## ステップ1 — Domain / UseCase の骨組みを作る

**目的**
`SBI` の登録ユースケースを Clean Architecture + DDD 構成で作成。ID は `SBI-<ULID>` を自動採番。`spec.md` の内容は「ガイドラインブロック + タイトル見出し + 本文」を組み立てる（保存はまだしない）。

**前提/制約**

* docs/IMPLEMENTATION_GUIDELINES.md と docs/DIRECTORY_STRUCTURE_TEMPLATE.md を遵守
* 依存方向: interface → usecase → domain（内向き）
* ID 生成は ULID（oklog/ulid）
* 本文は検証しない（生テキスト可）


## 実装後のレポート作成について

* **各ステップの実装完了ごとに、必ず実装レポート (`r_SBI-<ID>_stepN.md`) を作成すること。**

    * `N` はステップ番号（1〜4）を付与する
    * 保存先:

      ```
      .claude/flows/r_SBI-<ULID>_stepN.md
      ```
* レポート内容は以下を含めること:

    1. **Summary**: 実装対象・概要
    2. **Commit/Tag**: 実装を反映したコミットIDやタグ
    3. **Evidence**: 動作確認コマンドやテスト結果の抜粋
    4. **Checklist**: ガイドライン遵守状況（依存方向・責務分離・テスタビリティなど）
    5. **Verdict**: PASS / NEEDS CHANGES

* レポートはgit に強制的にadd しコミットプッシュすること。
---



**変更対象（新規ファイル）**

```
internal/domain/sbi/sbi.go
internal/domain/sbi/repository.go
internal/usecase/sbi/register_sbi_usecase.go
internal/usecase/sbi/input.go
internal/usecase/sbi/output.go
internal/usecase/sbi/spec_preamble.go
```

**実装タスク**

1. Domain

* `internal/domain/sbi/sbi.go`

    * `type SBI struct { ID string; Title string; Body string }`
    * コンストラクタ的関数 `NewSBI(id, title, body string) (*SBI, error)`（title の空チェックのみ。body は任意）
* `internal/domain/sbi/repository.go`

    * `type SBIRepository interface { Save(ctx context.Context, s *SBI) (specPath string, err error) }`

2. UseCase 入出力 DTO

* `internal/usecase/sbi/input.go`

    * `type RegisterSBIInput struct { Title string; Body string }`
* `internal/usecase/sbi/output.go`

    * `type RegisterSBIOutput struct { ID string; SpecPath string }`

3. UseCase 本体

* `internal/usecase/sbi/spec_preamble.go`

    * `func BuildSpecMarkdown(title, body string) string`

        * 先頭に**固定ガイドラインブロック**を挿入し、その後に `# <title>`、続けて `body` を連結（末尾改行は任意）
* `internal/usecase/sbi/register_sbi_usecase.go`

    * `type RegisterSBIUseCase struct { Repo domain.SBIRepository; Now func() time.Time; Rand io.Reader }`
    * `func (uc *RegisterSBIUseCase) Execute(ctx context.Context, in RegisterSBIInput) (*RegisterSBIOutput, error)`

        * `ULID` 生成 → `id := "SBI-" + ulid.MustNew(ulid.Timestamp(uc.Now()), uc.Rand).String()`
        * `content := BuildSpecMarkdown(in.Title, in.Body)`
        * `entity := domain.NewSBI(id, in.Title, content)`
        * `specPath, err := uc.Repo.Save(ctx, entity)`
        * `return &RegisterSBIOutput{ID: id, SpecPath: specPath}, nil`

**受け入れ基準**

* `RegisterSBIUseCase.Execute` が `ID` を `SBI-` で始まる ULID 形式で返すこと
* `BuildSpecMarkdown` がガイドラインブロック＋`# <title>`＋本文を結合すること
* repository は**まだ**実体不要（モック前提）

**動作確認（最小テスト雛形／擬似）**

```go
// internal/usecase/sbi/register_sbi_usecase_test.go
// - Rand を固定乱数化、Now を固定時刻にして ID 再現性確認
// - Repo.Save をモックして呼び出し検証
```

**注意点**

* interface/infra に触れない。CLIやファイル出力は次ステップ以降。
* プレアンブル文字列は docs のパスをそのまま含める。

---

## ステップ2 — Infrastructure(File) リポジトリ実装

**目的**
`SBIRepository` をファイル実装。保存先は**固定**：`tmp/test01/.deespec/specs/sbi/SBI-<ULID>/spec.md`。Atomic write と `MkdirAll` のエラーチェックを実装。

**変更対象（新規ファイル）**

```
internal/infra/repository/sbi/file_sbi_repository.go
internal/infra/persistence/file/atomic_writer.go
```

**実装タスク**

1. Atomic Writer（共通部品）

* `internal/infra/persistence/file/atomic_writer.go`

    * `func WriteFileAtomic(fs afero.Fs, path string, data []byte) error`

        * `dir := filepath.Dir(path)` → `MkdirAll(dir, 0o755)`（**errチェック必須**）
        * `tmp, _ := afero.TempFile(fs, dir, ".tmp-*")` → write → `Sync` → `Close` → `Rename` → `Remove` tmp

2. FileSBIRepository

* `internal/infra/repository/sbi/file_sbi_repository.go`

    * `type FileSBIRepository struct { FS afero.Fs }`
    * `const baseSBI = "tmp/test01/.deespec/specs/sbi"`
    * `func (r *FileSBIRepository) Save(ctx context.Context, s *domain.SBI) (string, error)`

        * `specDir := filepath.Join(baseSBI, s.ID)`
        * `specPath := filepath.Join(specDir, "spec.md")`
        * `WriteFileAtomic(r.FS, specPath, []byte(s.Body))`
        * 返り値は `specPath`

**受け入れ基準**

* `Save` が指定パスに `spec.md` を作成する（既存あっても毎回上書きでOK）
* 例外時に適切なエラーを返す（`MkdirAll` 失敗、Rename 失敗など）
* 文字列は**そのまま**保存（本文検証なし）

**動作確認**

```bash
# （後続のCLI実装前のため、usecaseテストで repository を実体に差し替えて動作確認）
go test ./...
```

**注意点**

* すべて `afero.Fs` で注入可能にし、テストで `afero.NewMemMapFs()` を使えるようにする。
* 既存 lint 指摘の「戻り値未チェック」を必ず回避。

---

## ステップ3 — Interface(CLI) `sbi register` 実装

**目的**
CLI から `RegisterSBIUseCase` を呼び出し、`spec.md` を作成できるようにする。
フラグ：`--title`（必須）、`--body`（任意・未指定なら stdin）、`--json`、`--dry-run`、`--quiet`。

**変更対象（新規/更新ファイル）**

```
internal/interface/cli/register_cmd.go
internal/app/bootstrap/bootstrap.go（DI結線があれば）
```

**実装タスク**

1. cobra コマンド

* `Use: "sbi register"`
* Flags:

    * `--title string`（必須、空ならエラー）
    * `--body string`（任意。空なら stdin を `io.ReadAll(os.Stdin)` で取得）
    * `--json`, `--dry-run`, `--quiet`（既存方針踏襲）

2. ハンドラ

* 入力収集 → `RegisterSBIUseCase.Execute`
* `--dry-run` の場合は**実際の保存呼び出しは行わず**、IDと想定パスを計算して JSON/テキスト出力のみ

    * 想定パス：`tmp/test01/.deespec/specs/sbi/<ID>/spec.md`
* `--json` 時は `{"ok":true,"id":"...","spec_path":"...","created":true}` を出力
* それ以外は人間向け短文（1行〜2行）

3. DI（必要であれば）

* UseCase に `FileSBIRepository`・`Now`・`Rand` を注入

**受け入れ基準**

* コマンドが `--title` 無しでエラーになる
* `--body` 省略で stdin 取り込み可能
* 実行後、指定パスに `spec.md` が存在し、先頭にガイドラインブロック、次に `# <title>`、本文が続く
* `--json` 出力が想定キーを含む

**動作確認コマンド**

```bash
# 1) body 引数あり
./deespec sbi register --title "Hello World" --body "本文" --json

# 2) stdin
printf "自由テキスト" | ./deespec sbi register --title "LINE連携" --json

# 3) 生成物確認
ls tmp/test01/.deespec/specs/sbi/SBI-*/spec.md | tail -n 1 | xargs -I{} sh -c 'echo "== {} =="; sed -n "1,25p" "{}"'
```

**注意点**

* CLI 層にビジネスロジックを書かない（詰め替えと表示のみ）。
* 例外はエラーメッセージを STDERR に短く出す。

---

## ステップ4 — テスト整備と最終チェック

**目的**
ユニット/統合/E2E の最小テストで回帰と将来拡張の安全網を用意。

**変更対象（新規テスト）**

```
internal/usecase/sbi/register_sbi_usecase_test.go
internal/infra/repository/sbi/file_sbi_repository_test.go
internal/interface/cli/register_cmd_test.go（可能なら）
```

**実装タスク（要点）**

1. UseCase テスト

* 固定 `Now`/`Rand` で `ID` 再現可能に
* Repo をモックして、`Save` の呼び出し内容（`Body` にガイドラインブロック＋タイトルが含まれる）を検証
* Title 空時はエラーになること

2. Infra テスト

* `afero.NewMemMapFs()` で `Save` を呼び、`spec.md` 内容がそのまま保存されることを確認
* `MkdirAll`/`Rename` の失敗をシミュレーションできるなら簡単に

3. CLI（任意）

* `--title` 無しでエラー終了
* `--json` 出力のキー存在チェック（スモーク）

**受け入れ基準**

* `go test ./...` が通る
* 手動 E2E（ステップ3のコマンド）が成功
* 生成された `spec.md` の1行目〜にガイドラインブロックが出力され、その直後に `# <title>` がある

**確認コマンド**

```bash
go test ./...
./deespec sbi register --title "E2E確認" --body "本文" --json
ls tmp/test01/.deespec/specs/sbi/SBI-*/spec.md | tail -n 1 | xargs -I{} sed -n '1,40p' "{}"
```

**作業完了**
テスト実施後goのlintを実施formatを整えること。
全ての作業が完了後コミットしてプッシュすること。

**注意点**

* 将来的に `--from-md` / API 連携を追加しても、UseCase と Repo を再利用できる分離になっていることを確認。

