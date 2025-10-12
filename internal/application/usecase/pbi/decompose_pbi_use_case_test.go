package pbi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/label"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/pbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPBIRepository implements pbi.Repository for testing
type mockPBIRepository struct {
	findByIDFunc func(id string) (*pbi.PBI, error)
	getBodyFunc  func(id string) (string, error)
	saveFunc     func(p *pbi.PBI, body string) error
}

func (m *mockPBIRepository) Save(p *pbi.PBI, body string) error {
	if m.saveFunc != nil {
		return m.saveFunc(p, body)
	}
	return nil
}

func (m *mockPBIRepository) FindByID(id string) (*pbi.PBI, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(id)
	}
	return nil, errors.New("not implemented")
}

func (m *mockPBIRepository) GetBody(id string) (string, error) {
	if m.getBodyFunc != nil {
		return m.getBodyFunc(id)
	}
	return "", errors.New("not implemented")
}

func (m *mockPBIRepository) FindAll() ([]*pbi.PBI, error) {
	return nil, errors.New("not implemented")
}

func (m *mockPBIRepository) FindByStatus(status pbi.Status) ([]*pbi.PBI, error) {
	return nil, errors.New("not implemented")
}

func (m *mockPBIRepository) Delete(id string) error {
	return errors.New("not implemented")
}

func (m *mockPBIRepository) Exists(id string) (bool, error) {
	return false, errors.New("not implemented")
}

// mockPromptTemplateRepository implements repository.PromptTemplateRepository for testing
type mockPromptTemplateRepository struct {
	loadPBIDecomposeTemplateFunc func(ctx context.Context) (string, error)
}

func (m *mockPromptTemplateRepository) LoadTemplate(ctx context.Context, status string) (string, error) {
	return "", errors.New("not implemented")
}

func (m *mockPromptTemplateRepository) LoadLabelContent(ctx context.Context, labelName string) string {
	return ""
}

func (m *mockPromptTemplateRepository) LoadMetaLabels(ctx context.Context, sbiID string) ([]string, error) {
	return nil, errors.New("not implemented")
}

func (m *mockPromptTemplateRepository) LoadPBIDecomposeTemplate(ctx context.Context) (string, error) {
	if m.loadPBIDecomposeTemplateFunc != nil {
		return m.loadPBIDecomposeTemplateFunc(ctx)
	}
	return "", errors.New("not implemented")
}

// mockSBIApprovalRepository implements repository.SBIApprovalRepository for testing
type mockSBIApprovalRepository struct {
	saveManifestFunc   func(ctx context.Context, manifest *pbi.SBIApprovalManifest) error
	loadManifestFunc   func(ctx context.Context, pbiID string) (*pbi.SBIApprovalManifest, error)
	manifestExistsFunc func(ctx context.Context, pbiID string) (bool, error)
	deleteManifestFunc func(ctx context.Context, pbiID string) error
}

// mockLabelRepository implements repository.LabelRepository for testing
type mockLabelRepository struct {
	findActiveFunc func(ctx context.Context) ([]*label.Label, error)
}

func (m *mockLabelRepository) Save(ctx context.Context, lbl *label.Label) error {
	return errors.New("not implemented")
}

func (m *mockLabelRepository) FindByID(ctx context.Context, id int) (*label.Label, error) {
	return nil, errors.New("not implemented")
}

func (m *mockLabelRepository) FindByName(ctx context.Context, name string) (*label.Label, error) {
	return nil, errors.New("not implemented")
}

func (m *mockLabelRepository) FindAll(ctx context.Context) ([]*label.Label, error) {
	return nil, errors.New("not implemented")
}

func (m *mockLabelRepository) FindActive(ctx context.Context) ([]*label.Label, error) {
	if m.findActiveFunc != nil {
		return m.findActiveFunc(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *mockLabelRepository) Update(ctx context.Context, lbl *label.Label) error {
	return errors.New("not implemented")
}

func (m *mockLabelRepository) Delete(ctx context.Context, id int) error {
	return errors.New("not implemented")
}

func (m *mockLabelRepository) ValidateIntegrity(ctx context.Context, labelID int) (*repository.ValidationResult, error) {
	return nil, errors.New("not implemented")
}

func (m *mockLabelRepository) ValidateAllLabels(ctx context.Context) ([]*repository.ValidationResult, error) {
	return nil, errors.New("not implemented")
}

func (m *mockLabelRepository) SyncFromFile(ctx context.Context, labelID int) error {
	return errors.New("not implemented")
}

func (m *mockLabelRepository) FindChildren(ctx context.Context, parentID int) ([]*label.Label, error) {
	return nil, errors.New("not implemented")
}

func (m *mockLabelRepository) FindByParentID(ctx context.Context, parentID *int) ([]*label.Label, error) {
	return nil, errors.New("not implemented")
}

func (m *mockSBIApprovalRepository) SaveManifest(ctx context.Context, manifest *pbi.SBIApprovalManifest) error {
	if m.saveManifestFunc != nil {
		return m.saveManifestFunc(ctx, manifest)
	}
	return nil
}

func (m *mockSBIApprovalRepository) LoadManifest(ctx context.Context, pbiID repository.PBIID) (*pbi.SBIApprovalManifest, error) {
	if m.loadManifestFunc != nil {
		return m.loadManifestFunc(ctx, string(pbiID))
	}
	return nil, errors.New("not implemented")
}

func (m *mockSBIApprovalRepository) ManifestExists(ctx context.Context, pbiID repository.PBIID) (bool, error) {
	if m.manifestExistsFunc != nil {
		return m.manifestExistsFunc(ctx, string(pbiID))
	}
	return false, errors.New("not implemented")
}

func (m *mockSBIApprovalRepository) DeleteManifest(ctx context.Context, pbiID repository.PBIID) error {
	if m.deleteManifestFunc != nil {
		return m.deleteManifestFunc(ctx, string(pbiID))
	}
	return errors.New("not implemented")
}

// TestNewDecomposePBIUseCase verifies usecase initialization
func TestNewDecomposePBIUseCase(t *testing.T) {
	pbiRepo := &mockPBIRepository{}
	promptRepo := &mockPromptTemplateRepository{}
	approvalRepo := &mockSBIApprovalRepository{}
	labelRepo := &mockLabelRepository{}

	useCase := NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, labelRepo, nil)

	require.NotNil(t, useCase)
	assert.NotNil(t, useCase.pbiRepo)
	assert.NotNil(t, useCase.promptRepo)
	assert.NotNil(t, useCase.approvalRepo)
	assert.NotNil(t, useCase.labelRepo)
}

// TestDecomposePBIUseCase_Execute_DryRun tests dry-run mode
func TestDecomposePBIUseCase_Execute_DryRun(t *testing.T) {
	testPBI := &pbi.PBI{
		ID:                   "PBI-001",
		Title:                "Test PBI",
		Status:               pbi.StatusPending,
		EstimatedStoryPoints: 8,
		Priority:             pbi.PriorityHigh,
	}

	pbiRepo := &mockPBIRepository{
		findByIDFunc: func(id string) (*pbi.PBI, error) {
			if id == "PBI-001" {
				return testPBI, nil
			}
			return nil, errors.New("PBI not found")
		},
		getBodyFunc: func(id string) (string, error) {
			return "# Test PBI\n\nThis is a test PBI body.", nil
		},
	}

	promptRepo := &mockPromptTemplateRepository{
		loadPBIDecomposeTemplateFunc: func(ctx context.Context) (string, error) {
			return `PBI: {{.PBIID}}
Title: {{.Title}}
Points: {{.StoryPoints}}
Priority: {{.Priority}}
SBIs: {{.MinSBIs}}-{{.MaxSBIs}}
Body: {{.PBIBody}}
Dir: {{.PBIDir}}`, nil
		},
	}

	approvalRepo := &mockSBIApprovalRepository{}
	useCase := NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, nil, nil)

	opts := DecomposeOptions{
		MinSBIs: 2,
		MaxSBIs: 5,
		DryRun:  true,
	}

	result, err := useCase.Execute(context.Background(), "PBI-001", opts)

	require.NoError(t, err)
	assert.Equal(t, "PBI-001", result.PBIID)
	assert.Equal(t, 0, result.SBICount)
	assert.Empty(t, result.SBIFiles)
	assert.Contains(t, result.Message, "Prompt-only mode")
	assert.Contains(t, result.Prompt, "PBI: PBI-001")
	assert.Contains(t, result.Prompt, "Title: Test PBI")
	assert.Contains(t, result.Prompt, "Points: 8")
	assert.Contains(t, result.Prompt, "Priority: 高")
	assert.Contains(t, result.Prompt, "SBIs: 2-5")
	assert.Contains(t, result.Prompt, "Dir: .deespec/specs/pbi/PBI-001")
}

// TestDecomposePBIUseCase_Execute_PBINotFound tests error when PBI doesn't exist
func TestDecomposePBIUseCase_Execute_PBINotFound(t *testing.T) {
	pbiRepo := &mockPBIRepository{
		findByIDFunc: func(id string) (*pbi.PBI, error) {
			return nil, errors.New("PBI not found")
		},
	}

	promptRepo := &mockPromptTemplateRepository{}
	approvalRepo := &mockSBIApprovalRepository{}
	useCase := NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, nil, nil)

	opts := DecomposeOptions{
		MinSBIs: 2,
		MaxSBIs: 5,
		DryRun:  true,
	}

	result, err := useCase.Execute(context.Background(), "PBI-999", opts)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to find PBI")
}

// TestDecomposePBIUseCase_Execute_InvalidStatus tests error for invalid PBI status
func TestDecomposePBIUseCase_Execute_InvalidStatus(t *testing.T) {
	testCases := []struct {
		name   string
		status pbi.Status
	}{
		{"StatusPlaned", pbi.StatusPlaned},
		{"StatusInProgress", pbi.StatusInProgress},
		{"StatusDone", pbi.StatusDone},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testPBI := &pbi.PBI{
				ID:     "PBI-001",
				Title:  "Test PBI",
				Status: tc.status,
			}

			pbiRepo := &mockPBIRepository{
				findByIDFunc: func(id string) (*pbi.PBI, error) {
					return testPBI, nil
				},
			}

			promptRepo := &mockPromptTemplateRepository{}
			approvalRepo := &mockSBIApprovalRepository{}
			useCase := NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, nil, nil)

			opts := DecomposeOptions{
				MinSBIs: 2,
				MaxSBIs: 5,
				DryRun:  true,
			}

			result, err := useCase.Execute(context.Background(), "PBI-001", opts)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "cannot be decomposed")
			assert.Contains(t, err.Error(), string(tc.status))
		})
	}
}

// TestDecomposePBIUseCase_Execute_ValidStatuses tests valid statuses for decomposition
func TestDecomposePBIUseCase_Execute_ValidStatuses(t *testing.T) {
	testCases := []struct {
		name   string
		status pbi.Status
	}{
		{"StatusPending", pbi.StatusPending},
		{"StatusPlanning", pbi.StatusPlanning},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testPBI := &pbi.PBI{
				ID:                   "PBI-001",
				Title:                "Test PBI",
				Status:               tc.status,
				EstimatedStoryPoints: 5,
				Priority:             pbi.PriorityNormal,
			}

			pbiRepo := &mockPBIRepository{
				findByIDFunc: func(id string) (*pbi.PBI, error) {
					return testPBI, nil
				},
				getBodyFunc: func(id string) (string, error) {
					return "Test body", nil
				},
			}

			promptRepo := &mockPromptTemplateRepository{
				loadPBIDecomposeTemplateFunc: func(ctx context.Context) (string, error) {
					return "{{.PBIID}}", nil
				},
			}

			approvalRepo := &mockSBIApprovalRepository{}
			useCase := NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, nil, nil)

			opts := DecomposeOptions{
				MinSBIs: 2,
				MaxSBIs: 5,
				DryRun:  true,
			}

			result, err := useCase.Execute(context.Background(), "PBI-001", opts)

			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, "PBI-001", result.PBIID)
		})
	}
}

// TestDecomposePBIUseCase_buildDecomposePrompt tests prompt building logic
func TestDecomposePBIUseCase_buildDecomposePrompt(t *testing.T) {
	testPBI := &pbi.PBI{
		ID:                   "PBI-123",
		Title:                "Implement Feature X",
		Status:               pbi.StatusPending,
		EstimatedStoryPoints: 13,
		Priority:             pbi.PriorityUrgent,
	}

	pbiRepo := &mockPBIRepository{}
	promptRepo := &mockPromptTemplateRepository{
		loadPBIDecomposeTemplateFunc: func(ctx context.Context) (string, error) {
			// Simple template for testing
			return `ID: {{.PBIID}}
Title: {{.Title}}
Points: {{.StoryPoints}}
Priority: {{.Priority}}
Range: {{.MinSBIs}}-{{.MaxSBIs}}
Body:
{{.PBIBody}}
Directory: {{.PBIDir}}`, nil
		},
	}

	approvalRepo := &mockSBIApprovalRepository{}
	useCase := NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, nil, nil)

	opts := DecomposeOptions{
		MinSBIs: 3,
		MaxSBIs: 7,
	}

	prompt, err := useCase.buildDecomposePrompt(
		context.Background(),
		testPBI,
		"This is the PBI body content.",
		opts,
	)

	require.NoError(t, err)
	assert.Contains(t, prompt, "ID: PBI-123")
	assert.Contains(t, prompt, "Title: Implement Feature X")
	assert.Contains(t, prompt, "Points: 13")
	assert.Contains(t, prompt, "Priority: 緊急")
	assert.Contains(t, prompt, "Range: 3-7")
	assert.Contains(t, prompt, "This is the PBI body content.")
	assert.Contains(t, prompt, "Directory: .deespec/specs/pbi/PBI-123")
}

// TestDecomposePBIUseCase_formatPriority tests priority formatting
func TestDecomposePBIUseCase_formatPriority(t *testing.T) {
	pbiRepo := &mockPBIRepository{}
	promptRepo := &mockPromptTemplateRepository{}
	approvalRepo := &mockSBIApprovalRepository{}
	useCase := NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, nil, nil)

	testCases := []struct {
		priority pbi.Priority
		expected string
	}{
		{pbi.PriorityNormal, "通常"},
		{pbi.PriorityHigh, "高"},
		{pbi.PriorityUrgent, "緊急"},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			result := useCase.formatPriority(tc.priority)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestDecomposePBIUseCase_canDecompose tests decomposition eligibility
func TestDecomposePBIUseCase_canDecompose(t *testing.T) {
	pbiRepo := &mockPBIRepository{}
	promptRepo := &mockPromptTemplateRepository{}
	approvalRepo := &mockSBIApprovalRepository{}
	useCase := NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, nil, nil)

	testCases := []struct {
		name      string
		status    pbi.Status
		shouldErr bool
	}{
		{"Pending - Valid", pbi.StatusPending, false},
		{"Planning - Valid", pbi.StatusPlanning, false},
		{"Planed - Invalid", pbi.StatusPlaned, true},
		{"InProgress - Invalid", pbi.StatusInProgress, true},
		{"Done - Invalid", pbi.StatusDone, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testPBI := &pbi.PBI{
				ID:     "PBI-001",
				Status: tc.status,
			}

			err := useCase.canDecompose(testPBI)

			if tc.shouldErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), string(tc.status))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestDecomposePBIUseCase_Execute_TemplateLoadError tests error handling for template loading
func TestDecomposePBIUseCase_Execute_TemplateLoadError(t *testing.T) {
	testPBI := &pbi.PBI{
		ID:     "PBI-001",
		Title:  "Test PBI",
		Status: pbi.StatusPending,
	}

	pbiRepo := &mockPBIRepository{
		findByIDFunc: func(id string) (*pbi.PBI, error) {
			return testPBI, nil
		},
		getBodyFunc: func(id string) (string, error) {
			return "Test body", nil
		},
	}

	promptRepo := &mockPromptTemplateRepository{
		loadPBIDecomposeTemplateFunc: func(ctx context.Context) (string, error) {
			return "", errors.New("template not found")
		},
	}

	approvalRepo := &mockSBIApprovalRepository{}
	useCase := NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, nil, nil)

	opts := DecomposeOptions{
		MinSBIs: 2,
		MaxSBIs: 5,
		DryRun:  true,
	}

	result, err := useCase.Execute(context.Background(), "PBI-001", opts)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to load PBI decompose template")
}

// TestDecomposePBIUseCase_Execute_GetBodyError tests error handling for body retrieval
func TestDecomposePBIUseCase_Execute_GetBodyError(t *testing.T) {
	testPBI := &pbi.PBI{
		ID:     "PBI-001",
		Title:  "Test PBI",
		Status: pbi.StatusPending,
	}

	pbiRepo := &mockPBIRepository{
		findByIDFunc: func(id string) (*pbi.PBI, error) {
			return testPBI, nil
		},
		getBodyFunc: func(id string) (string, error) {
			return "", errors.New("file not found")
		},
	}

	promptRepo := &mockPromptTemplateRepository{}
	approvalRepo := &mockSBIApprovalRepository{}
	useCase := NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, nil, nil)

	opts := DecomposeOptions{
		MinSBIs: 2,
		MaxSBIs: 5,
		DryRun:  true,
	}

	result, err := useCase.Execute(context.Background(), "PBI-001", opts)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get PBI body")
}

// TestDecomposePBIUseCase_Execute_NonDryRun tests that non-dry-run mode creates prompt file
func TestDecomposePBIUseCase_Execute_NonDryRun(t *testing.T) {
	testDir := t.TempDir()

	testPBI := &pbi.PBI{
		ID:                   "PBI-001",
		Title:                "Test PBI",
		Status:               pbi.StatusPending,
		EstimatedStoryPoints: 5,
		Priority:             pbi.PriorityNormal,
	}

	pbiRepo := &mockPBIRepository{
		findByIDFunc: func(id string) (*pbi.PBI, error) {
			return testPBI, nil
		},
		getBodyFunc: func(id string) (string, error) {
			return "Test body", nil
		},
	}

	promptRepo := &mockPromptTemplateRepository{
		loadPBIDecomposeTemplateFunc: func(ctx context.Context) (string, error) {
			return "{{.PBIID}}", nil
		},
	}

	approvalRepo := &mockSBIApprovalRepository{}
	useCase := NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, nil, nil)
	useCase.workingDir = testDir

	opts := DecomposeOptions{
		MinSBIs: 2,
		MaxSBIs: 5,
		DryRun:  false, // Non-dry-run mode
	}

	result, err := useCase.Execute(context.Background(), "PBI-001", opts)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "PBI-001", result.PBIID)
	assert.NotEmpty(t, result.PromptFilePath)
	// NonDryRun mode can have multiple outcomes, accept any of them
	assert.True(t,
		strings.Contains(result.Message, "Agent gateway not available") ||
			strings.Contains(result.Message, "Claude CLI not available") ||
			strings.Contains(result.Message, "AI execution failed") ||
			strings.Contains(result.Message, "AI executed but no SBI files were generated") ||
			strings.Contains(result.Message, "Successfully generated"),
		"Unexpected message: %s", result.Message)

	// Verify file was created
	expectedPath := filepath.Join(testDir, ".deespec", "specs", "pbi", "PBI-001", "decompose_prompt.md")
	assert.FileExists(t, expectedPath)
}

// TestDecomposeOptions_DefaultValues tests default option values
func TestDecomposeOptions_DefaultValues(t *testing.T) {
	opts := DecomposeOptions{}

	assert.Equal(t, 0, opts.MinSBIs)
	assert.Equal(t, 0, opts.MaxSBIs)
	assert.False(t, opts.DryRun)
	assert.False(t, opts.OutputOnly)
}

// TestDecomposeResult_Structure tests result structure
func TestDecomposeResult_Structure(t *testing.T) {
	result := DecomposeResult{
		PBIID:    "PBI-001",
		SBICount: 3,
		SBIFiles: []string{"sbi_1.md", "sbi_2.md", "sbi_3.md"},
		Message:  "Success",
		Prompt:   "Test prompt",
	}

	assert.Equal(t, "PBI-001", result.PBIID)
	assert.Equal(t, 3, result.SBICount)
	assert.Len(t, result.SBIFiles, 3)
	assert.Equal(t, "Success", result.Message)
	assert.Equal(t, "Test prompt", result.Prompt)
}

// TestDecomposePBIUseCase_buildDecomposePrompt_InvalidTemplate tests invalid template syntax
func TestDecomposePBIUseCase_buildDecomposePrompt_InvalidTemplate(t *testing.T) {
	testPBI := &pbi.PBI{
		ID:     "PBI-001",
		Title:  "Test",
		Status: pbi.StatusPending,
	}

	pbiRepo := &mockPBIRepository{}
	promptRepo := &mockPromptTemplateRepository{
		loadPBIDecomposeTemplateFunc: func(ctx context.Context) (string, error) {
			// Invalid template syntax
			return "{{.InvalidField", nil
		},
	}

	approvalRepo := &mockSBIApprovalRepository{}
	useCase := NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, nil, nil)

	opts := DecomposeOptions{MinSBIs: 2, MaxSBIs: 5}

	prompt, err := useCase.buildDecomposePrompt(context.Background(), testPBI, "body", opts)

	require.Error(t, err)
	assert.Empty(t, prompt)
	assert.Contains(t, err.Error(), "failed to parse template")
}

// TestDecomposePBIUseCase_Integration tests full flow in dry-run mode
func TestDecomposePBIUseCase_Integration(t *testing.T) {
	// Create a realistic PBI
	testPBI := &pbi.PBI{
		ID:                   "PBI-042",
		Title:                "Add user authentication system",
		Status:               pbi.StatusPlanning,
		EstimatedStoryPoints: 8,
		Priority:             pbi.PriorityHigh,
	}

	pbiBody := strings.TrimSpace(`
# Add user authentication system

## Overview
Implement a secure user authentication system with login, logout, and session management.

## Requirements
- JWT-based authentication
- Password hashing with bcrypt
- Session timeout after 30 minutes
`)

	pbiRepo := &mockPBIRepository{
		findByIDFunc: func(id string) (*pbi.PBI, error) {
			if id == "PBI-042" {
				return testPBI, nil
			}
			return nil, errors.New("not found")
		},
		getBodyFunc: func(id string) (string, error) {
			if id == "PBI-042" {
				return pbiBody, nil
			}
			return "", errors.New("not found")
		},
	}

	promptRepo := &mockPromptTemplateRepository{
		loadPBIDecomposeTemplateFunc: func(ctx context.Context) (string, error) {
			return `# Decompose PBI: {{.PBIID}}

**Title**: {{.Title}}
**Story Points**: {{.StoryPoints}}
**Priority**: {{.Priority}}

Create {{.MinSBIs}} to {{.MaxSBIs}} SBIs.

## PBI Content
{{.PBIBody}}

## Output Directory
{{.PBIDir}}`, nil
		},
	}

	approvalRepo := &mockSBIApprovalRepository{}
	useCase := NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, nil, nil)

	opts := DecomposeOptions{
		MinSBIs: 3,
		MaxSBIs: 6,
		DryRun:  true,
	}

	result, err := useCase.Execute(context.Background(), "PBI-042", opts)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify result structure
	assert.Equal(t, "PBI-042", result.PBIID)
	assert.Equal(t, 0, result.SBICount)
	assert.Empty(t, result.SBIFiles)
	assert.Contains(t, result.Message, "Prompt-only mode")

	// Verify prompt content
	assert.Contains(t, result.Prompt, "PBI-042")
	assert.Contains(t, result.Prompt, "Add user authentication system")
	assert.Contains(t, result.Prompt, "8")
	assert.Contains(t, result.Prompt, "高")
	assert.Contains(t, result.Prompt, "3 to 6")
	assert.Contains(t, result.Prompt, "JWT-based authentication")
	assert.Contains(t, result.Prompt, ".deespec/specs/pbi/PBI-042")
}

// TestDecomposePBIUseCase_Execute_PromptFileGeneration tests actual file creation
func TestDecomposePBIUseCase_Execute_PromptFileGeneration(t *testing.T) {
	// Create temporary directory for test
	testDir := t.TempDir()

	testPBI := &pbi.PBI{
		ID:                   "PBI-TEST-001",
		Title:                "Test Feature Implementation",
		Status:               pbi.StatusPending,
		EstimatedStoryPoints: 8,
		Priority:             pbi.PriorityHigh,
	}

	pbiBody := `# Test Feature Implementation

## Overview
This is a test PBI for integration testing.

## Requirements
- Feature A
- Feature B
- Feature C`

	pbiRepo := &mockPBIRepository{
		findByIDFunc: func(id string) (*pbi.PBI, error) {
			if id == "PBI-TEST-001" {
				return testPBI, nil
			}
			return nil, errors.New("not found")
		},
		getBodyFunc: func(id string) (string, error) {
			if id == "PBI-TEST-001" {
				return pbiBody, nil
			}
			return "", errors.New("not found")
		},
	}

	promptRepo := &mockPromptTemplateRepository{
		loadPBIDecomposeTemplateFunc: func(ctx context.Context) (string, error) {
			return `# Decompose: {{.PBIID}}
Title: {{.Title}}
Points: {{.StoryPoints}}
Priority: {{.Priority}}
SBIs: {{.MinSBIs}}-{{.MaxSBIs}}

{{.PBIBody}}

Output: {{.PBIDir}}`, nil
		},
	}

	approvalRepo := &mockSBIApprovalRepository{}
	useCase := NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, nil, nil)

	// Override working directory to use test directory
	useCase.workingDir = testDir

	opts := DecomposeOptions{
		MinSBIs: 3,
		MaxSBIs: 6,
		DryRun:  false, // Actually write file
	}

	result, err := useCase.Execute(context.Background(), "PBI-TEST-001", opts)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify result
	assert.Equal(t, "PBI-TEST-001", result.PBIID)
	assert.NotEmpty(t, result.PromptFilePath)
	// NonDryRun mode can have multiple outcomes, accept any of them
	assert.True(t,
		strings.Contains(result.Message, "Agent gateway not available") ||
			strings.Contains(result.Message, "Claude CLI not available") ||
			strings.Contains(result.Message, "AI execution failed") ||
			strings.Contains(result.Message, "AI executed but no SBI files were generated") ||
			strings.Contains(result.Message, "Successfully generated"),
		"Unexpected message: %s", result.Message)

	// Verify file was created
	expectedPath := filepath.Join(testDir, ".deespec", "specs", "pbi", "PBI-TEST-001", "decompose_prompt.md")
	assert.FileExists(t, expectedPath)

	// Verify file contents
	content, err := os.ReadFile(expectedPath)
	require.NoError(t, err)
	contentStr := string(content)

	assert.Contains(t, contentStr, "PBI-TEST-001")
	assert.Contains(t, contentStr, "Test Feature Implementation")
	assert.Contains(t, contentStr, "8")
	assert.Contains(t, contentStr, "高")
	assert.Contains(t, contentStr, "3-6")
	assert.Contains(t, contentStr, "Feature A")

	// Verify file permissions
	info, err := os.Stat(expectedPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0644), info.Mode().Perm())

	// Verify directory permissions
	dirInfo, err := os.Stat(filepath.Dir(expectedPath))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), dirInfo.Mode().Perm())
}

// TestDecomposePBIUseCase_writePromptFile_DirectoryCreation tests directory auto-creation
func TestDecomposePBIUseCase_writePromptFile_DirectoryCreation(t *testing.T) {
	testDir := t.TempDir()

	pbiRepo := &mockPBIRepository{}
	promptRepo := &mockPromptTemplateRepository{}
	approvalRepo := &mockSBIApprovalRepository{}
	useCase := NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, nil, nil)
	useCase.workingDir = testDir

	promptContent := "Test prompt content"
	pbiID := "PBI-NEW-001"

	promptFilePath, err := useCase.writePromptFile(pbiID, promptContent)

	require.NoError(t, err)
	assert.NotEmpty(t, promptFilePath)

	// Verify directory structure was created
	expectedDir := filepath.Join(testDir, ".deespec", "specs", "pbi", pbiID)
	assert.DirExists(t, expectedDir)

	// Verify file exists
	expectedPath := filepath.Join(expectedDir, "decompose_prompt.md")
	assert.FileExists(t, expectedPath)
	assert.Equal(t, expectedPath, promptFilePath)

	// Verify content
	content, err := os.ReadFile(expectedPath)
	require.NoError(t, err)
	assert.Equal(t, promptContent, string(content))
}

// TestDecomposePBIUseCase_listGeneratedSBIs tests SBI file listing
func TestDecomposePBIUseCase_listGeneratedSBIs(t *testing.T) {
	testDir := t.TempDir()

	pbiRepo := &mockPBIRepository{}
	promptRepo := &mockPromptTemplateRepository{}
	approvalRepo := &mockSBIApprovalRepository{}
	useCase := NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, nil, nil)
	useCase.workingDir = testDir

	pbiID := "PBI-SBI-TEST-001"
	pbiDir := filepath.Join(testDir, ".deespec", "specs", "pbi", pbiID)
	require.NoError(t, os.MkdirAll(pbiDir, 0755))

	// Create test SBI files
	sbiFiles := []string{"sbi_01.md", "sbi_02.md", "sbi_03.md"}
	for _, file := range sbiFiles {
		filePath := filepath.Join(pbiDir, file)
		require.NoError(t, os.WriteFile(filePath, []byte("test content"), 0644))
	}

	// Create non-SBI files (should be ignored)
	require.NoError(t, os.WriteFile(filepath.Join(pbiDir, "pbi.md"), []byte("pbi"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(pbiDir, "notes.txt"), []byte("notes"), 0644))

	// List generated SBIs
	foundFiles, err := useCase.listGeneratedSBIs(pbiID)

	require.NoError(t, err)
	assert.Len(t, foundFiles, 3)

	// Verify filenames (order may vary)
	for _, expectedFile := range sbiFiles {
		assert.Contains(t, foundFiles, expectedFile)
	}

	// Verify non-SBI files are not included
	assert.NotContains(t, foundFiles, "pbi.md")
	assert.NotContains(t, foundFiles, "notes.txt")
}

// TestDecomposePBIUseCase_listGeneratedSBIs_NoFiles tests empty directory
func TestDecomposePBIUseCase_listGeneratedSBIs_NoFiles(t *testing.T) {
	testDir := t.TempDir()

	pbiRepo := &mockPBIRepository{}
	promptRepo := &mockPromptTemplateRepository{}
	approvalRepo := &mockSBIApprovalRepository{}
	useCase := NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, nil, nil)
	useCase.workingDir = testDir

	pbiID := "PBI-EMPTY-001"
	pbiDir := filepath.Join(testDir, ".deespec", "specs", "pbi", pbiID)
	require.NoError(t, os.MkdirAll(pbiDir, 0755))

	// List generated SBIs from empty directory
	foundFiles, err := useCase.listGeneratedSBIs(pbiID)

	require.NoError(t, err)
	assert.Empty(t, foundFiles)
}

// TestDecomposePBIUseCase_createApprovalManifest tests approval.yaml creation
func TestDecomposePBIUseCase_createApprovalManifest(t *testing.T) {
	pbiID := "PBI-APPROVAL-001"
	sbiFiles := []string{"sbi_01.md", "sbi_02.md", "sbi_03.md"}

	var savedManifest *pbi.SBIApprovalManifest

	pbiRepo := &mockPBIRepository{}
	promptRepo := &mockPromptTemplateRepository{}
	approvalRepo := &mockSBIApprovalRepository{
		saveManifestFunc: func(ctx context.Context, manifest *pbi.SBIApprovalManifest) error {
			savedManifest = manifest
			return nil
		},
	}

	useCase := NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, nil, nil)

	err := useCase.createApprovalManifest(context.Background(), pbiID, sbiFiles)

	require.NoError(t, err)
	require.NotNil(t, savedManifest)

	// Verify manifest structure
	assert.Equal(t, pbiID, savedManifest.PBIID)
	assert.Equal(t, 3, savedManifest.TotalSBIs)
	assert.Len(t, savedManifest.SBIs, 3)
	assert.False(t, savedManifest.Registered)

	// Verify all SBIs are in pending status
	for i, sbi := range savedManifest.SBIs {
		assert.Equal(t, sbiFiles[i], sbi.File)
		assert.Equal(t, pbi.ApprovalStatusPending, sbi.Status)
	}
}

// TestDecomposePBIUseCase_createApprovalManifest_SaveError tests error handling
func TestDecomposePBIUseCase_createApprovalManifest_SaveError(t *testing.T) {
	pbiID := "PBI-ERROR-001"
	sbiFiles := []string{"sbi_01.md"}

	pbiRepo := &mockPBIRepository{}
	promptRepo := &mockPromptTemplateRepository{}
	approvalRepo := &mockSBIApprovalRepository{
		saveManifestFunc: func(ctx context.Context, manifest *pbi.SBIApprovalManifest) error {
			return errors.New("disk full")
		},
	}

	useCase := NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, nil, nil)

	err := useCase.createApprovalManifest(context.Background(), pbiID, sbiFiles)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to save approval manifest")
	assert.Contains(t, err.Error(), "disk full")
}

// TestDecomposePBIUseCase_ValidateSBIFile_Success tests validation with correct format
func TestDecomposePBIUseCase_ValidateSBIFile_Success(t *testing.T) {
	// Create a valid SBI file content
	content := `# テストSBI

## 概要
テストタスク

## タスク詳細
実装内容

## 受け入れ基準
- [ ] テスト1

## 推定工数
2時間

---
Parent PBI: PBI-001
Sequence: 1
`

	tmpFile := filepath.Join(os.TempDir(), "test_sbi.md")
	defer os.Remove(tmpFile)

	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	pbiRepo := &mockPBIRepository{}
	promptRepo := &mockPromptTemplateRepository{}
	approvalRepo := &mockSBIApprovalRepository{}
	useCase := NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, nil, nil)

	err = useCase.ValidateSBIFile(tmpFile)

	assert.NoError(t, err)
}

// TestDecomposePBIUseCase_ValidateSBIFile_MissingSection tests missing required sections
func TestDecomposePBIUseCase_ValidateSBIFile_MissingSection(t *testing.T) {
	testCases := []struct {
		name           string
		content        string
		missingSection string
	}{
		{
			name: "Missing 概要",
			content: `# テストSBI

## タスク詳細
実装内容

## 受け入れ基準
- [ ] テスト1

## 推定工数
2時間

---
Parent PBI: PBI-001
Sequence: 1
`,
			missingSection: "## 概要",
		},
		{
			name: "Missing タスク詳細",
			content: `# テストSBI

## 概要
テストタスク

## 受け入れ基準
- [ ] テスト1

## 推定工数
2時間

---
Parent PBI: PBI-001
Sequence: 1
`,
			missingSection: "## タスク詳細",
		},
		{
			name: "Missing 受け入れ基準",
			content: `# テストSBI

## 概要
テストタスク

## タスク詳細
実装内容

## 推定工数
2時間

---
Parent PBI: PBI-001
Sequence: 1
`,
			missingSection: "## 受け入れ基準",
		},
		{
			name: "Missing 推定工数",
			content: `# テストSBI

## 概要
テストタスク

## タスク詳細
実装内容

## 受け入れ基準
- [ ] テスト1

---
Parent PBI: PBI-001
Sequence: 1
`,
			missingSection: "## 推定工数",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpFile := filepath.Join(os.TempDir(), "test_sbi_invalid.md")
			defer os.Remove(tmpFile)

			err := os.WriteFile(tmpFile, []byte(tc.content), 0644)
			require.NoError(t, err)

			pbiRepo := &mockPBIRepository{}
			promptRepo := &mockPromptTemplateRepository{}
			approvalRepo := &mockSBIApprovalRepository{}
			useCase := NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, nil, nil)

			err = useCase.ValidateSBIFile(tmpFile)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), "missing required sections")
			assert.Contains(t, err.Error(), tc.missingSection)
		})
	}
}

// TestDecomposePBIUseCase_ValidateSBIFile_MissingMetadata tests missing metadata
func TestDecomposePBIUseCase_ValidateSBIFile_MissingMetadata(t *testing.T) {
	testCases := []struct {
		name            string
		content         string
		missingMetadata string
	}{
		{
			name: "Missing Parent PBI",
			content: `# テストSBI

## 概要
テストタスク

## タスク詳細
実装内容

## 受け入れ基準
- [ ] テスト1

## 推定工数
2時間

---
Sequence: 1
`,
			missingMetadata: "Parent PBI:",
		},
		{
			name: "Missing Sequence",
			content: `# テストSBI

## 概要
テストタスク

## タスク詳細
実装内容

## 受け入れ基準
- [ ] テスト1

## 推定工数
2時間

---
Parent PBI: PBI-001
`,
			missingMetadata: "Sequence:",
		},
		{
			name: "Missing Both Metadata",
			content: `# テストSBI

## 概要
テストタスク

## タスク詳細
実装内容

## 受け入れ基準
- [ ] テスト1

## 推定工数
2時間
`,
			missingMetadata: "Parent PBI:",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpFile := filepath.Join(os.TempDir(), "test_sbi_no_metadata.md")
			defer os.Remove(tmpFile)

			err := os.WriteFile(tmpFile, []byte(tc.content), 0644)
			require.NoError(t, err)

			pbiRepo := &mockPBIRepository{}
			promptRepo := &mockPromptTemplateRepository{}
			approvalRepo := &mockSBIApprovalRepository{}
			useCase := NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, nil, nil)

			err = useCase.ValidateSBIFile(tmpFile)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), "missing required metadata")
			assert.Contains(t, err.Error(), tc.missingMetadata)
		})
	}
}

// TestDecomposePBIUseCase_ValidateSBIFile_FileNotFound tests file not found error
func TestDecomposePBIUseCase_ValidateSBIFile_FileNotFound(t *testing.T) {
	pbiRepo := &mockPBIRepository{}
	promptRepo := &mockPromptTemplateRepository{}
	approvalRepo := &mockSBIApprovalRepository{}
	useCase := NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, nil, nil)

	err := useCase.ValidateSBIFile("nonexistent_file.md")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read file")
}

// TestDecomposePBIUseCase_ValidateGeneratedSBIs_Success tests validation of multiple files
func TestDecomposePBIUseCase_ValidateGeneratedSBIs_Success(t *testing.T) {
	testDir := t.TempDir()

	pbiID := "PBI-VALIDATE-001"
	pbiDir := filepath.Join(testDir, ".deespec", "specs", "pbi", pbiID)
	require.NoError(t, os.MkdirAll(pbiDir, 0755))

	// Create valid SBI files
	validContent := `# テストSBI

## 概要
テストタスク

## タスク詳細
実装内容

## 受け入れ基準
- [ ] テスト1

## 推定工数
2時間

---
Parent PBI: PBI-VALIDATE-001
Sequence: 1
`

	sbiFiles := []string{"sbi_01.md", "sbi_02.md", "sbi_03.md"}
	for _, file := range sbiFiles {
		filePath := filepath.Join(pbiDir, file)
		require.NoError(t, os.WriteFile(filePath, []byte(validContent), 0644))
	}

	pbiRepo := &mockPBIRepository{}
	promptRepo := &mockPromptTemplateRepository{}
	approvalRepo := &mockSBIApprovalRepository{}
	useCase := NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, nil, nil)
	useCase.workingDir = testDir

	validFiles, err := useCase.ValidateGeneratedSBIs(pbiID)

	require.NoError(t, err)
	assert.Len(t, validFiles, 3)
}

// TestDecomposePBIUseCase_ValidateGeneratedSBIs_NoFiles tests error when no files exist
func TestDecomposePBIUseCase_ValidateGeneratedSBIs_NoFiles(t *testing.T) {
	testDir := t.TempDir()

	pbiID := "PBI-EMPTY-001"
	pbiDir := filepath.Join(testDir, ".deespec", "specs", "pbi", pbiID)
	require.NoError(t, os.MkdirAll(pbiDir, 0755))

	pbiRepo := &mockPBIRepository{}
	promptRepo := &mockPromptTemplateRepository{}
	approvalRepo := &mockSBIApprovalRepository{}
	useCase := NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, nil, nil)
	useCase.workingDir = testDir

	validFiles, err := useCase.ValidateGeneratedSBIs(pbiID)

	require.Error(t, err)
	assert.Nil(t, validFiles)
	assert.Contains(t, err.Error(), "no SBI files found")
}

// TestDecomposePBIUseCase_ValidateGeneratedSBIs_PartialFailure tests mixed valid/invalid files
func TestDecomposePBIUseCase_ValidateGeneratedSBIs_PartialFailure(t *testing.T) {
	testDir := t.TempDir()

	pbiID := "PBI-MIXED-001"
	pbiDir := filepath.Join(testDir, ".deespec", "specs", "pbi", pbiID)
	require.NoError(t, os.MkdirAll(pbiDir, 0755))

	// Valid content
	validContent := `# テストSBI

## 概要
テストタスク

## タスク詳細
実装内容

## 受け入れ基準
- [ ] テスト1

## 推定工数
2時間

---
Parent PBI: PBI-MIXED-001
Sequence: 1
`

	// Invalid content (missing sections)
	invalidContent := `# テストSBI

## 概要
テストタスク
`

	// Create mixed files
	require.NoError(t, os.WriteFile(filepath.Join(pbiDir, "sbi_01.md"), []byte(validContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(pbiDir, "sbi_02.md"), []byte(invalidContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(pbiDir, "sbi_03.md"), []byte(validContent), 0644))

	pbiRepo := &mockPBIRepository{}
	promptRepo := &mockPromptTemplateRepository{}
	approvalRepo := &mockSBIApprovalRepository{}
	useCase := NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, nil, nil)
	useCase.workingDir = testDir

	validFiles, err := useCase.ValidateGeneratedSBIs(pbiID)

	require.Error(t, err)
	assert.Nil(t, validFiles)
	assert.Contains(t, err.Error(), "validation failed for 1/3 SBI files")
	assert.Contains(t, err.Error(), "sbi_02.md")
}

// TestDecomposePBIUseCase_ValidateGeneratedSBIs_AllInvalid tests all files invalid
func TestDecomposePBIUseCase_ValidateGeneratedSBIs_AllInvalid(t *testing.T) {
	testDir := t.TempDir()

	pbiID := "PBI-INVALID-001"
	pbiDir := filepath.Join(testDir, ".deespec", "specs", "pbi", pbiID)
	require.NoError(t, os.MkdirAll(pbiDir, 0755))

	// Invalid content
	invalidContent := `# テストSBI

## 概要
テストタスク
`

	// Create all invalid files
	for i := 1; i <= 3; i++ {
		fileName := fmt.Sprintf("sbi_%02d.md", i)
		require.NoError(t, os.WriteFile(filepath.Join(pbiDir, fileName), []byte(invalidContent), 0644))
	}

	pbiRepo := &mockPBIRepository{}
	promptRepo := &mockPromptTemplateRepository{}
	approvalRepo := &mockSBIApprovalRepository{}
	useCase := NewDecomposePBIUseCase(pbiRepo, promptRepo, approvalRepo, nil, nil)
	useCase.workingDir = testDir

	validFiles, err := useCase.ValidateGeneratedSBIs(pbiID)

	require.Error(t, err)
	assert.Nil(t, validFiles)
	assert.Contains(t, err.Error(), "validation failed for 3/3 SBI files")
}
