package pbi

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/pbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSBIRepository implements repository.SBIRepository for testing
type mockSBIRepository struct {
	sbis         map[string]*sbi.SBI
	dependencies map[string][]string
	saveFunc     func(ctx context.Context, s *sbi.SBI) error
}

func newMockSBIRepository() *mockSBIRepository {
	return &mockSBIRepository{
		sbis:         make(map[string]*sbi.SBI),
		dependencies: make(map[string][]string),
	}
}

func (m *mockSBIRepository) Find(ctx context.Context, id repository.SBIID) (*sbi.SBI, error) {
	s, exists := m.sbis[string(id)]
	if !exists {
		return nil, os.ErrNotExist
	}
	return s, nil
}

func (m *mockSBIRepository) Save(ctx context.Context, s *sbi.SBI) error {
	if m.saveFunc != nil {
		return m.saveFunc(ctx, s)
	}
	m.sbis[s.ID().String()] = s
	return nil
}

func (m *mockSBIRepository) Delete(ctx context.Context, id repository.SBIID) error {
	delete(m.sbis, string(id))
	return nil
}

func (m *mockSBIRepository) List(ctx context.Context, filter repository.SBIFilter) ([]*sbi.SBI, error) {
	var result []*sbi.SBI
	for _, s := range m.sbis {
		result = append(result, s)
	}
	return result, nil
}

func (m *mockSBIRepository) FindByPBIID(ctx context.Context, pbiID repository.PBIID) ([]*sbi.SBI, error) {
	var result []*sbi.SBI
	for _, s := range m.sbis {
		if s.ParentTaskID() != nil && s.ParentTaskID().String() == string(pbiID) {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockSBIRepository) GetNextSequence(ctx context.Context) (int, error) {
	return len(m.sbis) + 1, nil
}

func (m *mockSBIRepository) ResetSBIState(ctx context.Context, id repository.SBIID, toStatus string) error {
	return nil
}

func (m *mockSBIRepository) GetDependencies(ctx context.Context, sbiID repository.SBIID) ([]string, error) {
	deps, exists := m.dependencies[string(sbiID)]
	if !exists {
		return []string{}, nil
	}
	return deps, nil
}

func (m *mockSBIRepository) GetDependents(ctx context.Context, sbiID repository.SBIID) ([]string, error) {
	return []string{}, nil
}

func (m *mockSBIRepository) SaveDependencies(ctx context.Context, sbiID repository.SBIID, dependsOn []string) error {
	m.dependencies[string(sbiID)] = dependsOn
	return nil
}

// Test helper functions
func setupTestEnvironment(t *testing.T) (string, func()) {
	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "deespec-test-*")
	require.NoError(t, err)

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

func createTestSBIFile(t *testing.T, tmpDir, pbiID, filename, title string, sequence int, hours float64) {
	pbiDir := filepath.Join(tmpDir, ".deespec", "specs", "pbi", pbiID)
	err := os.MkdirAll(pbiDir, 0755)
	require.NoError(t, err)

	content := fmt.Sprintf(`# %s

## 概要
Test SBI description

## タスク詳細
- Task details here

## 受け入れ基準
- [ ] Acceptance criteria 1

## 推定工数
%.1f

---
Parent PBI: %s
Sequence: %d
`, title, hours, pbiID, sequence)

	filePath := filepath.Join(pbiDir, filename)
	err = os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err)
}

// Test cases
func TestRegisterSBIsUseCase_Execute_Success(t *testing.T) {
	// Setup
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()
	pbiID := "PBI-001"

	// Setup test PBI
	testPBI := &pbi.PBI{
		ID:                   pbiID,
		Title:                "Test PBI",
		Status:               pbi.StatusPlanning,
		EstimatedStoryPoints: 8,
		Priority:             pbi.PriorityNormal,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	// Create mock repositories
	var savedPBI *pbi.PBI
	pbiRepo := &mockPBIRepository{
		findByIDFunc: func(id string) (*pbi.PBI, error) {
			if id == pbiID {
				return testPBI, nil
			}
			return nil, os.ErrNotExist
		},
		saveFunc: func(p *pbi.PBI, body string) error {
			savedPBI = p
			return nil
		},
	}
	sbiRepo := newMockSBIRepository()

	var savedManifest *pbi.SBIApprovalManifest
	manifests := make(map[string]*pbi.SBIApprovalManifest)
	approvalRepo := &mockSBIApprovalRepository{
		loadManifestFunc: func(ctx context.Context, id string) (*pbi.SBIApprovalManifest, error) {
			manifest, exists := manifests[id]
			if !exists {
				return nil, os.ErrNotExist
			}
			return manifest, nil
		},
		saveManifestFunc: func(ctx context.Context, manifest *pbi.SBIApprovalManifest) error {
			savedManifest = manifest
			manifests[manifest.PBIID] = manifest
			return nil
		},
	}

	// Create test SBI files
	createTestSBIFile(t, tmpDir, pbiID, "sbi_01_setup.md", "Setup Infrastructure", 1, 2.0)
	createTestSBIFile(t, tmpDir, pbiID, "sbi_02_implement.md", "Implement Feature", 2, 3.0)
	createTestSBIFile(t, tmpDir, pbiID, "sbi_03_test.md", "Add Tests", 3, 1.5)

	// Create approval manifest with approved SBIs
	manifest := &pbi.SBIApprovalManifest{
		PBIID:       pbiID,
		GeneratedAt: time.Now(),
		TotalSBIs:   3,
		SBIs: []pbi.SBIApprovalRecord{
			{File: "sbi_01_setup.md", Status: pbi.ApprovalStatusApproved},
			{File: "sbi_02_implement.md", Status: pbi.ApprovalStatusApproved},
			{File: "sbi_03_test.md", Status: pbi.ApprovalStatusApproved},
		},
		Registered: false,
	}
	manifests[pbiID] = manifest

	// Create use case
	useCase := NewRegisterSBIsUseCase(sbiRepo, pbiRepo, approvalRepo)
	useCase.SetWorkingDir(tmpDir)

	// Execute
	opts := RegisterSBIsOptions{
		DryRun: false,
		Force:  false,
	}

	result, err := useCase.Execute(ctx, pbiID, opts)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 3, result.RegisteredCount)
	assert.Equal(t, 0, result.SkippedCount)
	assert.Len(t, result.SBIIDs, 3)
	assert.Empty(t, result.Errors)

	// Verify SBIs were saved to repository
	assert.Len(t, sbiRepo.sbis, 3)

	// Verify dependencies were set correctly (sequential chain)
	sbiIDs := result.SBIIDs
	deps1, _ := sbiRepo.GetDependencies(ctx, repository.SBIID(sbiIDs[0]))
	assert.Empty(t, deps1, "First SBI should have no dependencies")

	deps2, _ := sbiRepo.GetDependencies(ctx, repository.SBIID(sbiIDs[1]))
	assert.Equal(t, []string{sbiIDs[0]}, deps2, "Second SBI should depend on first")

	deps3, _ := sbiRepo.GetDependencies(ctx, repository.SBIID(sbiIDs[2]))
	assert.Equal(t, []string{sbiIDs[1]}, deps3, "Third SBI should depend on second")

	// Verify approval manifest was updated
	assert.NotNil(t, savedManifest)
	assert.True(t, savedManifest.Registered)
	assert.NotNil(t, savedManifest.RegisteredAt)
	assert.Len(t, savedManifest.RegisteredSBIs, 3)

	// Verify PBI status was updated to "planed"
	assert.NotNil(t, savedPBI, "PBI should have been saved with updated status")
	assert.Equal(t, pbi.StatusPlaned, savedPBI.Status, "PBI status should be updated to 'planed'")
}

func TestRegisterSBIsUseCase_Execute_DryRun(t *testing.T) {
	// Setup
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()
	pbiID := "PBI-002"

	// Setup test PBI
	testPBI := &pbi.PBI{
		ID:        pbiID,
		Title:     "Test PBI",
		Status:    pbi.StatusPlanning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Create mock repositories
	var savedPBI *pbi.PBI
	pbiRepo := &mockPBIRepository{
		findByIDFunc: func(id string) (*pbi.PBI, error) {
			if id == pbiID {
				return testPBI, nil
			}
			return nil, os.ErrNotExist
		},
		saveFunc: func(p *pbi.PBI, body string) error {
			savedPBI = p
			return nil
		},
	}
	sbiRepo := newMockSBIRepository()

	manifests := make(map[string]*pbi.SBIApprovalManifest)
	approvalRepo := &mockSBIApprovalRepository{
		loadManifestFunc: func(ctx context.Context, id string) (*pbi.SBIApprovalManifest, error) {
			manifest, exists := manifests[id]
			if !exists {
				return nil, os.ErrNotExist
			}
			return manifest, nil
		},
		saveManifestFunc: func(ctx context.Context, manifest *pbi.SBIApprovalManifest) error {
			manifests[manifest.PBIID] = manifest
			return nil
		},
	}

	// Create test SBI files
	createTestSBIFile(t, tmpDir, pbiID, "sbi_01_test.md", "Test SBI", 1, 2.0)

	// Create approval manifest
	manifest := &pbi.SBIApprovalManifest{
		PBIID:       pbiID,
		GeneratedAt: time.Now(),
		TotalSBIs:   1,
		SBIs: []pbi.SBIApprovalRecord{
			{File: "sbi_01_test.md", Status: pbi.ApprovalStatusApproved},
		},
		Registered: false,
	}
	manifests[pbiID] = manifest

	// Create use case
	useCase := NewRegisterSBIsUseCase(sbiRepo, pbiRepo, approvalRepo)
	useCase.SetWorkingDir(tmpDir)

	// Execute with DryRun
	opts := RegisterSBIsOptions{
		DryRun: true,
		Force:  false,
	}

	result, err := useCase.Execute(ctx, pbiID, opts)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, result.RegisteredCount)
	assert.Len(t, result.SBIIDs, 1)

	// Verify SBIs were NOT saved to repository (dry-run mode)
	assert.Empty(t, sbiRepo.sbis)

	// Verify manifest was NOT updated (dry-run mode)
	updatedManifest := manifests[pbiID]
	assert.False(t, updatedManifest.Registered, "Manifest should not be updated in dry-run mode")

	// Verify PBI status was NOT updated (dry-run mode)
	assert.Nil(t, savedPBI, "PBI should not be saved in dry-run mode")
}

func TestRegisterSBIsUseCase_Execute_NoApprovedSBIs(t *testing.T) {
	// Setup
	ctx := context.Background()
	pbiID := "PBI-003"

	// Setup test PBI
	testPBI := &pbi.PBI{
		ID:        pbiID,
		Title:     "Test PBI",
		Status:    pbi.StatusPlanning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Create approval manifest with NO approved SBIs
	manifests := make(map[string]*pbi.SBIApprovalManifest)
	manifests[pbiID] = &pbi.SBIApprovalManifest{
		PBIID:       pbiID,
		GeneratedAt: time.Now(),
		TotalSBIs:   2,
		SBIs: []pbi.SBIApprovalRecord{
			{File: "sbi_01_test.md", Status: pbi.ApprovalStatusPending},
			{File: "sbi_02_test.md", Status: pbi.ApprovalStatusRejected},
		},
		Registered: false,
	}

	// Create mock repositories
	pbiRepo := &mockPBIRepository{
		findByIDFunc: func(id string) (*pbi.PBI, error) {
			if id == pbiID {
				return testPBI, nil
			}
			return nil, os.ErrNotExist
		},
	}
	sbiRepo := newMockSBIRepository()
	approvalRepo := &mockSBIApprovalRepository{
		loadManifestFunc: func(ctx context.Context, id string) (*pbi.SBIApprovalManifest, error) {
			manifest, exists := manifests[id]
			if !exists {
				return nil, os.ErrNotExist
			}
			return manifest, nil
		},
	}

	// Create use case
	useCase := NewRegisterSBIsUseCase(sbiRepo, pbiRepo, approvalRepo)

	// Execute
	opts := RegisterSBIsOptions{
		DryRun: false,
		Force:  false,
	}

	_, err := useCase.Execute(ctx, pbiID, opts)

	// Assert - should return error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no approved SBIs found")
}

func TestRegisterSBIsUseCase_Execute_AlreadyRegistered(t *testing.T) {
	// Setup
	ctx := context.Background()
	pbiID := "PBI-004"

	// Setup test PBI
	testPBI := &pbi.PBI{
		ID:        pbiID,
		Title:     "Test PBI",
		Status:    pbi.StatusPlanning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Create approval manifest that is already registered
	now := time.Now()
	manifests := make(map[string]*pbi.SBIApprovalManifest)
	manifests[pbiID] = &pbi.SBIApprovalManifest{
		PBIID:       pbiID,
		GeneratedAt: time.Now(),
		TotalSBIs:   1,
		SBIs: []pbi.SBIApprovalRecord{
			{File: "sbi_01_test.md", Status: pbi.ApprovalStatusApproved},
		},
		Registered:     true,
		RegisteredAt:   &now,
		RegisteredSBIs: []string{"some-sbi-id"},
	}

	// Create mock repositories
	pbiRepo := &mockPBIRepository{
		findByIDFunc: func(id string) (*pbi.PBI, error) {
			if id == pbiID {
				return testPBI, nil
			}
			return nil, os.ErrNotExist
		},
	}
	sbiRepo := newMockSBIRepository()
	approvalRepo := &mockSBIApprovalRepository{
		loadManifestFunc: func(ctx context.Context, id string) (*pbi.SBIApprovalManifest, error) {
			manifest, exists := manifests[id]
			if !exists {
				return nil, os.ErrNotExist
			}
			return manifest, nil
		},
	}

	// Create use case
	useCase := NewRegisterSBIsUseCase(sbiRepo, pbiRepo, approvalRepo)

	// Execute without Force flag
	opts := RegisterSBIsOptions{
		DryRun: false,
		Force:  false,
	}

	_, err := useCase.Execute(ctx, pbiID, opts)

	// Assert - should return error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegisterSBIsUseCase_Execute_PBINotFound(t *testing.T) {
	// Setup
	ctx := context.Background()
	pbiID := "NON-EXISTENT-PBI"

	// Create mock repositories
	pbiRepo := &mockPBIRepository{
		findByIDFunc: func(id string) (*pbi.PBI, error) {
			return nil, os.ErrNotExist
		},
	}
	sbiRepo := newMockSBIRepository()
	approvalRepo := &mockSBIApprovalRepository{}

	// Create use case
	useCase := NewRegisterSBIsUseCase(sbiRepo, pbiRepo, approvalRepo)

	// Execute
	opts := RegisterSBIsOptions{
		DryRun: false,
		Force:  false,
	}

	_, err := useCase.Execute(ctx, pbiID, opts)

	// Assert - should return error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find PBI")
}

func TestRegisterSBIsUseCase_Execute_PBIStatusUpdateError(t *testing.T) {
	// Setup
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()
	pbiID := "PBI-STATUS-ERROR"

	// Setup test PBI
	testPBI := &pbi.PBI{
		ID:        pbiID,
		Title:     "Test PBI",
		Status:    pbi.StatusPlanning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Create mock repositories - save will fail when saving PBI with updated status
	pbiRepo := &mockPBIRepository{
		findByIDFunc: func(id string) (*pbi.PBI, error) {
			if id == pbiID {
				return testPBI, nil
			}
			return nil, os.ErrNotExist
		},
		saveFunc: func(p *pbi.PBI, body string) error {
			// Fail when trying to save the PBI with updated status
			if p.Status == pbi.StatusPlaned {
				return fmt.Errorf("database connection lost")
			}
			return nil
		},
	}
	sbiRepo := newMockSBIRepository()

	manifests := make(map[string]*pbi.SBIApprovalManifest)
	approvalRepo := &mockSBIApprovalRepository{
		loadManifestFunc: func(ctx context.Context, id string) (*pbi.SBIApprovalManifest, error) {
			manifest, exists := manifests[id]
			if !exists {
				return nil, os.ErrNotExist
			}
			return manifest, nil
		},
		saveManifestFunc: func(ctx context.Context, manifest *pbi.SBIApprovalManifest) error {
			manifests[manifest.PBIID] = manifest
			return nil
		},
	}

	// Create test SBI file
	createTestSBIFile(t, tmpDir, pbiID, "sbi_01_test.md", "Test SBI", 1, 2.0)

	// Create approval manifest
	manifest := &pbi.SBIApprovalManifest{
		PBIID:       pbiID,
		GeneratedAt: time.Now(),
		TotalSBIs:   1,
		SBIs: []pbi.SBIApprovalRecord{
			{File: "sbi_01_test.md", Status: pbi.ApprovalStatusApproved},
		},
		Registered: false,
	}
	manifests[pbiID] = manifest

	// Create use case
	useCase := NewRegisterSBIsUseCase(sbiRepo, pbiRepo, approvalRepo)
	useCase.SetWorkingDir(tmpDir)

	// Execute
	opts := RegisterSBIsOptions{
		DryRun: false,
		Force:  false,
	}

	_, err := useCase.Execute(ctx, pbiID, opts)

	// Assert - should return error about PBI save failure
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to save PBI with updated status")
}
