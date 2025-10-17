---
allowed-tools: Bash(git:*)
description: Commit all changes and push
---

# Commit and Push All Changes

## **重要な注意事項**

**このコマンドはコミットのみを行います。バージョン管理は行いません。**

- ✅ コード変更をコミットしてプッシュ
- ❌ VERSIONファイルを変更しない
- ❌ リリースタグを作成しない

バージョン管理とリリースは `/mnv` コマンドで別途実行してください。

## 実行手順

### 1. Lintの実行

```bash
gofmt -s -w .
go vet ./...
```

変更が必要な場合は修正してください。

### 2. CHANGELOG.mdの更新

現在の **Unreleased** セクションに変更内容を追加してください：

```markdown
## [Unreleased]

### 追加 (Added)
- [新機能の説明]

### 変更 (Changed)
- [変更内容の説明]

### 修正 (Fixed)
- [バグ修正の説明]

---
```

**重要**: バージョン番号のセクション（例: `## [v0.2.14]`）は作成しないでください。これは `/mnv` コマンドで行います。

### 3. コミット＆プッシュ

```bash
git add -A
git commit -m "<適切なコミットメッセージ>"
git push origin main
```

**コミットメッセージの注意**:
- 署名に Anthropic や Claude を含めないでください
- 変更内容から適切なメッセージを作成してください

