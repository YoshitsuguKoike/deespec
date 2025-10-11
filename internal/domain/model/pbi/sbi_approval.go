package pbi

import "time"

// SBIApprovalStatus represents the approval status of a generated SBI
type SBIApprovalStatus string

const (
	ApprovalStatusPending  SBIApprovalStatus = "pending"
	ApprovalStatusApproved SBIApprovalStatus = "approved"
	ApprovalStatusRejected SBIApprovalStatus = "rejected"
	ApprovalStatusEdited   SBIApprovalStatus = "edited"
)

// SBIApprovalRecord represents approval information for a single SBI file
type SBIApprovalRecord struct {
	File            string            `yaml:"file"`
	Status          SBIApprovalStatus `yaml:"status"`
	ReviewedBy      string            `yaml:"reviewed_by,omitempty"`
	ReviewedAt      *time.Time        `yaml:"reviewed_at,omitempty"`
	Notes           string            `yaml:"notes,omitempty"`
	RejectionReason string            `yaml:"rejection_reason,omitempty"`
}

// SBIApprovalManifest represents the approval manifest for all generated SBIs
type SBIApprovalManifest struct {
	PBIID          string              `yaml:"pbi_id"`
	GeneratedAt    time.Time           `yaml:"generated_at"`
	TotalSBIs      int                 `yaml:"total_sbis"`
	SBIs           []SBIApprovalRecord `yaml:"sbis"`
	Registered     bool                `yaml:"registered"`
	RegisteredAt   *time.Time          `yaml:"registered_at,omitempty"`
	RegisteredSBIs []string            `yaml:"registered_sbis,omitempty"`
}

// NewSBIApprovalManifest creates a new approval manifest
func NewSBIApprovalManifest(pbiID string, sbiFiles []string) *SBIApprovalManifest {
	now := time.Now()
	records := make([]SBIApprovalRecord, len(sbiFiles))

	for i, file := range sbiFiles {
		records[i] = SBIApprovalRecord{
			File:   file,
			Status: ApprovalStatusPending,
		}
	}

	return &SBIApprovalManifest{
		PBIID:       pbiID,
		GeneratedAt: now,
		TotalSBIs:   len(sbiFiles),
		SBIs:        records,
		Registered:  false,
	}
}

// GetApprovedSBIs returns list of approved SBI files
func (m *SBIApprovalManifest) GetApprovedSBIs() []string {
	var approved []string
	for _, sbi := range m.SBIs {
		if sbi.Status == ApprovalStatusApproved || sbi.Status == ApprovalStatusEdited {
			approved = append(approved, sbi.File)
		}
	}
	return approved
}

// GetPendingSBIs returns list of pending SBI files
func (m *SBIApprovalManifest) GetPendingSBIs() []string {
	var pending []string
	for _, sbi := range m.SBIs {
		if sbi.Status == ApprovalStatusPending {
			pending = append(pending, sbi.File)
		}
	}
	return pending
}

// ApprovedCount returns the number of approved SBIs
func (m *SBIApprovalManifest) ApprovedCount() int {
	count := 0
	for _, sbi := range m.SBIs {
		if sbi.Status == ApprovalStatusApproved || sbi.Status == ApprovalStatusEdited {
			count++
		}
	}
	return count
}

// PendingCount returns the number of pending SBIs
func (m *SBIApprovalManifest) PendingCount() int {
	count := 0
	for _, sbi := range m.SBIs {
		if sbi.Status == ApprovalStatusPending {
			count++
		}
	}
	return count
}

// RejectedCount returns the number of rejected SBIs
func (m *SBIApprovalManifest) RejectedCount() int {
	count := 0
	for _, sbi := range m.SBIs {
		if sbi.Status == ApprovalStatusRejected {
			count++
		}
	}
	return count
}
