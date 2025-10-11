---
allowed-tools: Bash(git:*), Read, Edit, Write
description: Manage new version - bump version and create release
---

# バージョン管理コマンド

新しいバージョンをリリースするための手順を実行してください。

## 引数の処理

引数が提供されている場合：
- 提供された引数を新しいバージョン番号として使用してください
- 形式: `<MAJOR>.<MINOR>.<PATCH>` (例: 0.3.0, 1.0.0, 0.2.2)

引数がない場合：
1. VERSIONファイルを読み取り、現在のバージョンを確認
2. パッチバージョン（最後の数字）を1つインクリメント
   - 例: 0.2.1 → 0.2.2

## 実行手順

### 1. VERSIONファイルの更新

新しいバージョン番号をVERSIONファイルに書き込んでください：
```bash
echo "<新しいバージョン>" > VERSION
```

### 2. CHANGELOG.mdの更新

CHANGELOG.mdを以下の形式で更新してください：

```markdown
## [Unreleased]

---

## [v<新しいバージョン>] - <今日の日付 YYYY-MM-DD>

### 追加 (Added)
- [変更内容を記載]

### 変更 (Changed)
- [変更内容を記載]

### 修正 (Fixed)
- [変更内容を記載]

---
```

最近のgitコミット履歴を確認して、適切な変更内容を記載してください。

### 3. ローカルビルドテスト

以下のコマンドで動作確認してください：
```bash
make version        # バージョン確認
make build         # ビルド
./dist/deespec version  # バイナリのバージョン確認
```

### 4. コミット＆プッシュ

すべての変更をコミットしてプッシュしてください：
```bash
git add VERSION CHANGELOG.md
git commit -m "chore: bump version to <新しいバージョン>

- Update VERSION file to <新しいバージョン>
- Update CHANGELOG.md with release notes"
git push origin main
```

新しいバージョンでタグを作りリモートにプッシュしてください：
```bash
git tag <新しいバージョン>

git push origin <新しいバージョン>
```

## 注意事項

- コミットメッセージに Anthropic や Claude の署名を含めないでください
- プッシュ後、GitHub Actions が自動的にリリースを作成します（5-10分）
- リリースの確認: https://github.com/YoshitsuguKoike/deespec/actions

## バージョン番号の決め方（Semantic Versioning）

- **PATCH（0.2.1 → 0.2.2）**: バグ修正のみ
- **MINOR（0.2.1 → 0.3.0）**: 新機能追加（後方互換性あり）
- **MAJOR（0.2.1 → 1.0.0）**: 破壊的変更
