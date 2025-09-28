package sbi

import (
	"fmt"
	"strings"
)

// BuildSpecMarkdown constructs the full markdown content for a specification
// It combines the guideline block, title heading, and body content
func BuildSpecMarkdown(title, body string) string {
	var sb strings.Builder

	// Fixed guideline block (preamble)
	sb.WriteString(`## ガイドライン

このドキュメントは、チームで共有される仕様書です。以下のガイドラインに従って記述してください。

### 記述ルール

1. **明確性**: 曖昧な表現を避け、具体的に記述する
2. **完全性**: 必要な情報をすべて含める
3. **一貫性**: 用語や形式を統一する
4. **追跡可能性**: 変更履歴を明確にする

### セクション構成

- 概要: 機能の目的と背景
- 詳細仕様: 具体的な要求事項
- 制約事項: 技術的・業務的制約
- 受け入れ条件: 完了の定義

---

`)

	// Title as H1
	sb.WriteString(fmt.Sprintf("# %s\n\n", title))

	// Body content
	if body != "" {
		sb.WriteString(body)
		// Ensure trailing newline
		if !strings.HasSuffix(body, "\n") {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
