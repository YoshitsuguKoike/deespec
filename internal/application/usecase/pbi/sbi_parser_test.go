package pbi

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSBIFile_Success(t *testing.T) {
	content := `# タスク1: ユーザー認証API実装

## 概要
ログインAPIを実装する

## タスク詳細
- 実装するファイル: internal/api/auth.go
- エンドポイント: POST /api/login

## 受け入れ基準
- [ ] ログインAPIが正常に動作する

## 推定工数
3

---
Parent PBI: PBI-001
Sequence: 1
`

	tmpFile := filepath.Join(os.TempDir(), "test_sbi.md")
	defer os.Remove(tmpFile)

	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	spec, err := ParseSBIFile(tmpFile)

	require.NoError(t, err)
	assert.Equal(t, "タスク1: ユーザー認証API実装", spec.Title)
	assert.Contains(t, spec.Body, "ログインAPI")
	assert.Equal(t, 3.0, spec.EstimatedHours)
	assert.Equal(t, "PBI-001", spec.ParentPBIID)
	assert.Equal(t, 1, spec.Sequence)
}

func TestParseSBIFile_WithDecimalHours(t *testing.T) {
	content := `# タスク2: データベース設計

## 概要
ユーザーテーブルを設計する

## 推定工数
2.5

---
Parent PBI: PBI-002
Sequence: 5
`

	tmpFile := filepath.Join(os.TempDir(), "test_sbi_decimal.md")
	defer os.Remove(tmpFile)

	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	spec, err := ParseSBIFile(tmpFile)

	require.NoError(t, err)
	assert.Equal(t, "タスク2: データベース設計", spec.Title)
	assert.Equal(t, 2.5, spec.EstimatedHours)
	assert.Equal(t, "PBI-002", spec.ParentPBIID)
	assert.Equal(t, 5, spec.Sequence)
}

func TestParseSBIFile_MissingMetadata(t *testing.T) {
	content := `# タスク1: テスト

## 概要
テスト

## 推定工数
2
`

	tmpFile := filepath.Join(os.TempDir(), "test_sbi_invalid.md")
	defer os.Remove(tmpFile)

	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	_, err = ParseSBIFile(tmpFile)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "metadata")
}

func TestParseSBIFile_MissingParentPBI(t *testing.T) {
	content := `# タスク1: テスト

## 概要
テスト

## 推定工数
2

---
Sequence: 1
`

	tmpFile := filepath.Join(os.TempDir(), "test_sbi_no_parent.md")
	defer os.Remove(tmpFile)

	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	_, err = ParseSBIFile(tmpFile)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Parent PBI")
}

func TestParseSBIFile_MissingSequence(t *testing.T) {
	content := `# タスク1: テスト

## 概要
テスト

## 推定工数
2

---
Parent PBI: PBI-001
`

	tmpFile := filepath.Join(os.TempDir(), "test_sbi_no_sequence.md")
	defer os.Remove(tmpFile)

	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	_, err = ParseSBIFile(tmpFile)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Sequence")
}

func TestParseSBIFile_InvalidSequence(t *testing.T) {
	content := `# タスク1: テスト

## 概要
テスト

## 推定工数
2

---
Parent PBI: PBI-001
Sequence: invalid
`

	tmpFile := filepath.Join(os.TempDir(), "test_sbi_invalid_sequence.md")
	defer os.Remove(tmpFile)

	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	_, err = ParseSBIFile(tmpFile)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sequence")
}

func TestParseSBIFile_MissingEstimatedHours(t *testing.T) {
	content := `# タスク1: テスト

## 概要
テスト

---
Parent PBI: PBI-001
Sequence: 1
`

	tmpFile := filepath.Join(os.TempDir(), "test_sbi_no_hours.md")
	defer os.Remove(tmpFile)

	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	_, err = ParseSBIFile(tmpFile)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "estimated hours")
}

func TestParseSBIFile_InvalidEstimatedHours(t *testing.T) {
	content := `# タスク1: テスト

## 概要
テスト

## 推定工数
invalid

---
Parent PBI: PBI-001
Sequence: 1
`

	tmpFile := filepath.Join(os.TempDir(), "test_sbi_invalid_hours.md")
	defer os.Remove(tmpFile)

	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	_, err = ParseSBIFile(tmpFile)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "estimated hours")
}

func TestParseSBIFile_MissingTitle(t *testing.T) {
	content := `## 概要
テスト

## 推定工数
2

---
Parent PBI: PBI-001
Sequence: 1
`

	tmpFile := filepath.Join(os.TempDir(), "test_sbi_no_title.md")
	defer os.Remove(tmpFile)

	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	_, err = ParseSBIFile(tmpFile)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "title")
}

func TestParseSBIFile_FileNotFound(t *testing.T) {
	_, err := ParseSBIFile("nonexistent_file.md")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read")
}

func TestParseSBIFile_ComplexBody(t *testing.T) {
	content := `# タスク3: 複雑な機能実装

## 概要
複数のセクションを持つ複雑な機能

## 背景
これは背景説明です

## タスク詳細
- 実装するファイル: internal/complex/feature.go
- 変更箇所: 詳細説明

### サブセクション
追加の詳細情報

## 受け入れ基準
- [ ] 基準1
- [ ] 基準2
- [ ] 基準3

## 技術的制約
- 制約1
- 制約2

## 推定工数
5.5

## テスト方法
テストの詳細

---
Parent PBI: PBI-003
Sequence: 10
`

	tmpFile := filepath.Join(os.TempDir(), "test_sbi_complex.md")
	defer os.Remove(tmpFile)

	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	spec, err := ParseSBIFile(tmpFile)

	require.NoError(t, err)
	assert.Equal(t, "タスク3: 複雑な機能実装", spec.Title)
	assert.Contains(t, spec.Body, "複数のセクション")
	assert.Contains(t, spec.Body, "## 背景")
	assert.Contains(t, spec.Body, "## タスク詳細")
	assert.Contains(t, spec.Body, "## 受け入れ基準")
	assert.NotContains(t, spec.Body, "Parent PBI:") // Metadata should not be in body
	assert.NotContains(t, spec.Body, "Sequence:")   // Metadata should not be in body
	assert.Equal(t, 5.5, spec.EstimatedHours)
	assert.Equal(t, "PBI-003", spec.ParentPBIID)
	assert.Equal(t, 10, spec.Sequence)
}

func TestExtractTitle_Success(t *testing.T) {
	content := `# タイトルテスト

## セクション1
内容
`

	title, err := extractTitle(content)

	require.NoError(t, err)
	assert.Equal(t, "タイトルテスト", title)
}

func TestExtractTitle_WithWhitespace(t *testing.T) {
	content := `
# タイトル with spaces

## セクション1
`

	title, err := extractTitle(content)

	require.NoError(t, err)
	assert.Equal(t, "タイトル with spaces", title)
}

func TestExtractTitle_NoTitle(t *testing.T) {
	content := `## セクション1

内容
`

	_, err := extractTitle(content)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no H1 title")
}

func TestExtractBody_WithMetadata(t *testing.T) {
	content := `# タイトル

## 概要
本文内容

---
Parent PBI: PBI-001
Sequence: 1
`

	body := extractBody(content)

	assert.Contains(t, body, "本文内容")
	assert.NotContains(t, body, "Parent PBI:")
	assert.NotContains(t, body, "Sequence:")
}

func TestExtractBody_NoMetadata(t *testing.T) {
	content := `# タイトル

## 概要
本文内容のみ
`

	body := extractBody(content)

	assert.Contains(t, body, "本文内容のみ")
}

func TestExtractEstimatedHours_Integer(t *testing.T) {
	content := `## 推定工数
3

---
`

	hours, err := extractEstimatedHours(content)

	require.NoError(t, err)
	assert.Equal(t, 3.0, hours)
}

func TestExtractEstimatedHours_Decimal(t *testing.T) {
	content := `## 推定工数
3.5

---
`

	hours, err := extractEstimatedHours(content)

	require.NoError(t, err)
	assert.Equal(t, 3.5, hours)
}

func TestExtractEstimatedHours_NotFound(t *testing.T) {
	content := `## 概要
テスト
`

	_, err := extractEstimatedHours(content)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestExtractMetadata_Success(t *testing.T) {
	content := `# タイトル

## 概要
本文

---
Parent PBI: PBI-123
Sequence: 7
`

	metadata, err := extractMetadata(content)

	require.NoError(t, err)
	assert.Equal(t, "PBI-123", metadata["Parent PBI"])
	assert.Equal(t, "7", metadata["Sequence"])
}

func TestExtractMetadata_NoDelimiter(t *testing.T) {
	content := `# タイトル

## 概要
本文

Parent PBI: PBI-123
Sequence: 7
`

	_, err := extractMetadata(content)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "metadata section not found")
}

func TestExtractMetadata_MissingParentPBI(t *testing.T) {
	content := `# タイトル

---
Sequence: 7
`

	_, err := extractMetadata(content)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Parent PBI")
}

func TestExtractMetadata_MissingSequence(t *testing.T) {
	content := `# タイトル

---
Parent PBI: PBI-123
`

	_, err := extractMetadata(content)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Sequence")
}

func TestParseSBIFile_WithParenthesesInParentPBI(t *testing.T) {
	content := `# タスク4: テスト

## 概要
テスト

## 推定工数
3

---
Parent PBI: PBI-001（PBI分解機能の実装）
Sequence: 13
`

	tmpFile := filepath.Join(os.TempDir(), "test_sbi_parentheses.md")
	defer os.Remove(tmpFile)

	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	spec, err := ParseSBIFile(tmpFile)

	require.NoError(t, err)
	assert.Equal(t, "PBI-001（PBI分解機能の実装）", spec.ParentPBIID)
	assert.Equal(t, 13, spec.Sequence)
}
