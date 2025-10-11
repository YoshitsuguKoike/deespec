package pbi

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/pbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// RegisterSBIsOptions defines options for SBI registration
type RegisterSBIsOptions struct {
	DryRun bool // If true, only validate without persisting to DB
	Force  bool // If true, allow re-registration of already registered SBIs
}

// RegisterSBIsResult represents the result of SBI registration
type RegisterSBIsResult struct {
	RegisteredCount int      // Number of SBIs successfully registered
	SkippedCount    int      // Number of SBIs skipped (already registered)
	SBIIDs          []string // IDs of registered SBIs
	Errors          []string // List of errors encountered (partial failures)
}

// RegisteredSBIInfo holds information about a successfully registered SBI
type registeredSBIInfo struct {
	ID       string
	Sequence int
	FilePath string
}

// RegisterSBIsUseCase handles registration of approved SBIs to the database
type RegisterSBIsUseCase struct {
	sbiRepo      repository.SBIRepository
	pbiRepo      pbi.Repository
	approvalRepo repository.SBIApprovalRepository
	workingDir   string // Base working directory (default: ".")
}

// NewRegisterSBIsUseCase creates a new RegisterSBIsUseCase instance
func NewRegisterSBIsUseCase(
	sbiRepo repository.SBIRepository,
	pbiRepo pbi.Repository,
	approvalRepo repository.SBIApprovalRepository,
) *RegisterSBIsUseCase {
	return &RegisterSBIsUseCase{
		sbiRepo:      sbiRepo,
		pbiRepo:      pbiRepo,
		approvalRepo: approvalRepo,
		workingDir:   ".",
	}
}

// Execute registers approved SBIs from approval.yaml to the database
// This is the main entry point for the registration process
func (u *RegisterSBIsUseCase) Execute(
	ctx context.Context,
	pbiID string,
	opts RegisterSBIsOptions,
) (*RegisterSBIsResult, error) {
	// 1. Validate PBI exists
	_, err := u.pbiRepo.FindByID(pbiID)
	if err != nil {
		return nil, fmt.Errorf("failed to find PBI %s: %w", pbiID, err)
	}

	// 2. Load approval manifest
	manifest, err := u.approvalRepo.LoadManifest(ctx, repository.PBIID(pbiID))
	if err != nil {
		return nil, fmt.Errorf("failed to load approval manifest for PBI %s: %w", pbiID, err)
	}

	// 3. Check if already registered (unless Force flag is set)
	if manifest.Registered && !opts.Force {
		return nil, fmt.Errorf("SBIs for PBI %s are already registered (use --force to re-register)", pbiID)
	}

	// 4. Get approved SBI files
	approvedFiles := manifest.GetApprovedSBIs()
	if len(approvedFiles) == 0 {
		return nil, fmt.Errorf("no approved SBIs found in approval manifest for PBI %s", pbiID)
	}

	// 5. Parse and register each SBI
	result := &RegisterSBIsResult{
		RegisteredCount: 0,
		SkippedCount:    0,
		SBIIDs:          []string{},
		Errors:          []string{},
	}

	var registeredSBIs []registeredSBIInfo
	var previousSBIID string // Track previous SBI for dependency chain

	for _, sbiFile := range approvedFiles {
		sbiFilePath := u.buildSBIFilePath(pbiID, sbiFile)

		// Parse SBI file
		spec, err := ParseSBIFile(sbiFilePath)
		if err != nil {
			errMsg := fmt.Sprintf("failed to parse %s: %v", sbiFile, err)
			result.Errors = append(result.Errors, errMsg)
			continue
		}

		// Register single SBI
		sbiID, err := u.registerSingleSBI(ctx, pbiID, spec, previousSBIID, opts)
		if err != nil {
			errMsg := fmt.Sprintf("failed to register %s: %v", sbiFile, err)
			result.Errors = append(result.Errors, errMsg)
			continue
		}

		// Track registered SBI
		registeredSBIs = append(registeredSBIs, registeredSBIInfo{
			ID:       sbiID,
			Sequence: spec.Sequence,
			FilePath: sbiFile,
		})
		result.SBIIDs = append(result.SBIIDs, sbiID)
		result.RegisteredCount++

		// Update previous SBI ID for dependency chain
		previousSBIID = sbiID
	}

	// 6. Handle errors (partial success case)
	if len(result.Errors) > 0 && result.RegisteredCount == 0 {
		// Total failure
		return result, fmt.Errorf("all SBI registrations failed")
	}

	// 7. Skip DB updates in dry-run mode
	if opts.DryRun {
		return result, nil
	}

	// 8. Update PBI status to "planed" after successful SBI registration
	// Only update if we have at least one successfully registered SBI
	if result.RegisteredCount > 0 {
		// Retrieve the PBI entity again to ensure we have the latest state
		pbiEntity, err := u.pbiRepo.FindByID(pbiID)
		if err != nil {
			return result, fmt.Errorf("failed to retrieve PBI for status update: %w", err)
		}

		// Update PBI status to "planed" (decomposed and ready for execution)
		if err := pbiEntity.UpdateStatus(pbi.StatusPlaned); err != nil {
			return result, fmt.Errorf("failed to update PBI status: %w", err)
		}

		// Save the updated PBI (empty body string since we're only updating metadata)
		if err := u.pbiRepo.Save(pbiEntity, ""); err != nil {
			return result, fmt.Errorf("failed to save PBI with updated status: %w", err)
		}
	}

	// 9. Update approval manifest with registration information
	if err := u.updateApprovalManifest(ctx, pbiID, registeredSBIs); err != nil {
		return result, fmt.Errorf("failed to update approval manifest: %w", err)
	}

	return result, nil
}

// registerSingleSBI parses and registers a single SBI to the database
// Returns the generated SBI ID
func (u *RegisterSBIsUseCase) registerSingleSBI(
	ctx context.Context,
	pbiID string,
	spec *SBISpec,
	previousSBIID string,
	opts RegisterSBIsOptions,
) (string, error) {
	// 1. Validate parent PBI ID matches
	if spec.ParentPBIID != pbiID {
		return "", fmt.Errorf(
			"parent PBI ID mismatch: expected %s, got %s in spec",
			pbiID,
			spec.ParentPBIID,
		)
	}

	// 2. Create SBI domain model
	taskID, err := model.NewTaskIDFromString(pbiID)
	if err != nil {
		return "", fmt.Errorf("invalid PBI ID: %w", err)
	}
	metadata := sbi.SBIMetadata{
		EstimatedHours: spec.EstimatedHours,
		Priority:       0, // Default priority
		Sequence:       spec.Sequence,
		RegisteredAt:   time.Now(),
		Labels:         []string{},
		AssignedAgent:  "claude-code", // Default agent
		FilePaths:      []string{},
		DependsOn:      []string{},
	}

	sbiEntity, err := sbi.NewSBI(spec.Title, spec.Body, &taskID, metadata)
	if err != nil {
		return "", fmt.Errorf("failed to create SBI entity: %w", err)
	}

	// 3. Set dependency on previous SBI (if exists)
	// This creates a sequential dependency chain: SBI N depends on SBI N-1
	if previousSBIID != "" {
		sbiEntity.AddDependency(previousSBIID)
	}

	// 4. Skip database save in dry-run mode
	if opts.DryRun {
		return sbiEntity.ID().String(), nil
	}

	// 5. Save SBI to database
	if err := u.sbiRepo.Save(ctx, sbiEntity); err != nil {
		return "", fmt.Errorf("failed to save SBI to database: %w", err)
	}

	// 6. Save dependencies separately (if repository supports it)
	if len(sbiEntity.DependsOn()) > 0 {
		sbiID := repository.SBIID(sbiEntity.ID().String())
		if err := u.sbiRepo.SaveDependencies(ctx, sbiID, sbiEntity.DependsOn()); err != nil {
			return "", fmt.Errorf("failed to save SBI dependencies: %w", err)
		}
	}

	return sbiEntity.ID().String(), nil
}

// updateApprovalManifest updates the approval.yaml with registration information
func (u *RegisterSBIsUseCase) updateApprovalManifest(
	ctx context.Context,
	pbiID string,
	registeredSBIs []registeredSBIInfo,
) error {
	// 1. Load current manifest
	manifest, err := u.approvalRepo.LoadManifest(ctx, repository.PBIID(pbiID))
	if err != nil {
		return fmt.Errorf("failed to load approval manifest: %w", err)
	}

	// 2. Update registration fields
	manifest.Registered = true
	now := time.Now()
	manifest.RegisteredAt = &now

	// 3. Collect registered SBI IDs
	manifest.RegisteredSBIs = []string{}
	for _, info := range registeredSBIs {
		manifest.RegisteredSBIs = append(manifest.RegisteredSBIs, info.ID)
	}

	// 4. Save updated manifest
	if err := u.approvalRepo.SaveManifest(ctx, manifest); err != nil {
		return fmt.Errorf("failed to save updated approval manifest: %w", err)
	}

	return nil
}

// buildSBIFilePath constructs the full path to an SBI file
func (u *RegisterSBIsUseCase) buildSBIFilePath(pbiID, sbiFile string) string {
	return filepath.Join(u.workingDir, ".deespec", "specs", "pbi", pbiID, sbiFile)
}

// SetWorkingDir sets the working directory (useful for testing)
func (u *RegisterSBIsUseCase) SetWorkingDir(dir string) {
	u.workingDir = dir
}
