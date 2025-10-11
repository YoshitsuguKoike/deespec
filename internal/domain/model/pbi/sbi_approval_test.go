package pbi

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestNewSBIApprovalManifest(t *testing.T) {
	tests := []struct {
		name     string
		pbiID    string
		sbiFiles []string
		wantLen  int
	}{
		{
			name:     "create manifest with 3 SBI files",
			pbiID:    "PBI-001",
			sbiFiles: []string{"sbi_1.md", "sbi_2.md", "sbi_3.md"},
			wantLen:  3,
		},
		{
			name:     "create manifest with 1 SBI file",
			pbiID:    "PBI-002",
			sbiFiles: []string{"sbi_1.md"},
			wantLen:  1,
		},
		{
			name:     "create manifest with empty SBI files",
			pbiID:    "PBI-003",
			sbiFiles: []string{},
			wantLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := NewSBIApprovalManifest(tt.pbiID, tt.sbiFiles)

			if manifest.PBIID != tt.pbiID {
				t.Errorf("PBIID = %v, want %v", manifest.PBIID, tt.pbiID)
			}

			if manifest.TotalSBIs != tt.wantLen {
				t.Errorf("TotalSBIs = %v, want %v", manifest.TotalSBIs, tt.wantLen)
			}

			if len(manifest.SBIs) != tt.wantLen {
				t.Errorf("len(SBIs) = %v, want %v", len(manifest.SBIs), tt.wantLen)
			}

			if manifest.Registered != false {
				t.Errorf("Registered = %v, want false", manifest.Registered)
			}

			// Verify all SBIs are initialized with pending status
			for i, sbi := range manifest.SBIs {
				if sbi.File != tt.sbiFiles[i] {
					t.Errorf("SBIs[%d].File = %v, want %v", i, sbi.File, tt.sbiFiles[i])
				}
				if sbi.Status != ApprovalStatusPending {
					t.Errorf("SBIs[%d].Status = %v, want %v", i, sbi.Status, ApprovalStatusPending)
				}
				if sbi.ReviewedBy != "" {
					t.Errorf("SBIs[%d].ReviewedBy should be empty", i)
				}
				if sbi.ReviewedAt != nil {
					t.Errorf("SBIs[%d].ReviewedAt should be nil", i)
				}
			}
		})
	}
}

func TestGetApprovedSBIs(t *testing.T) {
	manifest := &SBIApprovalManifest{
		PBIID: "PBI-001",
		SBIs: []SBIApprovalRecord{
			{File: "sbi_1.md", Status: ApprovalStatusApproved},
			{File: "sbi_2.md", Status: ApprovalStatusPending},
			{File: "sbi_3.md", Status: ApprovalStatusEdited},
			{File: "sbi_4.md", Status: ApprovalStatusRejected},
			{File: "sbi_5.md", Status: ApprovalStatusApproved},
		},
	}

	approved := manifest.GetApprovedSBIs()

	expectedLen := 3 // approved + edited
	if len(approved) != expectedLen {
		t.Errorf("len(approved) = %v, want %v", len(approved), expectedLen)
	}

	// Verify approved list contains correct files
	expectedFiles := []string{"sbi_1.md", "sbi_3.md", "sbi_5.md"}
	for i, file := range expectedFiles {
		if approved[i] != file {
			t.Errorf("approved[%d] = %v, want %v", i, approved[i], file)
		}
	}
}

func TestGetApprovedSBIs_EmptyWhenNoneApproved(t *testing.T) {
	manifest := &SBIApprovalManifest{
		PBIID: "PBI-001",
		SBIs: []SBIApprovalRecord{
			{File: "sbi_1.md", Status: ApprovalStatusPending},
			{File: "sbi_2.md", Status: ApprovalStatusRejected},
		},
	}

	approved := manifest.GetApprovedSBIs()

	if len(approved) != 0 {
		t.Errorf("len(approved) = %v, want 0", len(approved))
	}
}

func TestGetPendingSBIs(t *testing.T) {
	manifest := &SBIApprovalManifest{
		PBIID: "PBI-001",
		SBIs: []SBIApprovalRecord{
			{File: "sbi_1.md", Status: ApprovalStatusPending},
			{File: "sbi_2.md", Status: ApprovalStatusApproved},
			{File: "sbi_3.md", Status: ApprovalStatusPending},
			{File: "sbi_4.md", Status: ApprovalStatusRejected},
		},
	}

	pending := manifest.GetPendingSBIs()

	expectedLen := 2
	if len(pending) != expectedLen {
		t.Errorf("len(pending) = %v, want %v", len(pending), expectedLen)
	}

	// Verify pending list contains correct files
	expectedFiles := []string{"sbi_1.md", "sbi_3.md"}
	for i, file := range expectedFiles {
		if pending[i] != file {
			t.Errorf("pending[%d] = %v, want %v", i, pending[i], file)
		}
	}
}

func TestApprovedCount(t *testing.T) {
	tests := []struct {
		name string
		sbis []SBIApprovalRecord
		want int
	}{
		{
			name: "count approved and edited",
			sbis: []SBIApprovalRecord{
				{File: "sbi_1.md", Status: ApprovalStatusApproved},
				{File: "sbi_2.md", Status: ApprovalStatusEdited},
				{File: "sbi_3.md", Status: ApprovalStatusPending},
				{File: "sbi_4.md", Status: ApprovalStatusRejected},
			},
			want: 2,
		},
		{
			name: "all approved",
			sbis: []SBIApprovalRecord{
				{File: "sbi_1.md", Status: ApprovalStatusApproved},
				{File: "sbi_2.md", Status: ApprovalStatusApproved},
			},
			want: 2,
		},
		{
			name: "none approved",
			sbis: []SBIApprovalRecord{
				{File: "sbi_1.md", Status: ApprovalStatusPending},
				{File: "sbi_2.md", Status: ApprovalStatusRejected},
			},
			want: 0,
		},
		{
			name: "empty list",
			sbis: []SBIApprovalRecord{},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := &SBIApprovalManifest{
				PBIID: "PBI-001",
				SBIs:  tt.sbis,
			}

			got := manifest.ApprovedCount()
			if got != tt.want {
				t.Errorf("ApprovedCount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPendingCount(t *testing.T) {
	tests := []struct {
		name string
		sbis []SBIApprovalRecord
		want int
	}{
		{
			name: "mixed statuses",
			sbis: []SBIApprovalRecord{
				{File: "sbi_1.md", Status: ApprovalStatusPending},
				{File: "sbi_2.md", Status: ApprovalStatusApproved},
				{File: "sbi_3.md", Status: ApprovalStatusPending},
				{File: "sbi_4.md", Status: ApprovalStatusPending},
			},
			want: 3,
		},
		{
			name: "all pending",
			sbis: []SBIApprovalRecord{
				{File: "sbi_1.md", Status: ApprovalStatusPending},
				{File: "sbi_2.md", Status: ApprovalStatusPending},
			},
			want: 2,
		},
		{
			name: "none pending",
			sbis: []SBIApprovalRecord{
				{File: "sbi_1.md", Status: ApprovalStatusApproved},
				{File: "sbi_2.md", Status: ApprovalStatusRejected},
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := &SBIApprovalManifest{
				PBIID: "PBI-001",
				SBIs:  tt.sbis,
			}

			got := manifest.PendingCount()
			if got != tt.want {
				t.Errorf("PendingCount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRejectedCount(t *testing.T) {
	tests := []struct {
		name string
		sbis []SBIApprovalRecord
		want int
	}{
		{
			name: "mixed statuses",
			sbis: []SBIApprovalRecord{
				{File: "sbi_1.md", Status: ApprovalStatusRejected},
				{File: "sbi_2.md", Status: ApprovalStatusApproved},
				{File: "sbi_3.md", Status: ApprovalStatusRejected},
			},
			want: 2,
		},
		{
			name: "all rejected",
			sbis: []SBIApprovalRecord{
				{File: "sbi_1.md", Status: ApprovalStatusRejected},
			},
			want: 1,
		},
		{
			name: "none rejected",
			sbis: []SBIApprovalRecord{
				{File: "sbi_1.md", Status: ApprovalStatusPending},
				{File: "sbi_2.md", Status: ApprovalStatusApproved},
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := &SBIApprovalManifest{
				PBIID: "PBI-001",
				SBIs:  tt.sbis,
			}

			got := manifest.RejectedCount()
			if got != tt.want {
				t.Errorf("RejectedCount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSBIApprovalManifest_YAMLMarshal(t *testing.T) {
	now := time.Date(2025, 10, 12, 10, 0, 0, 0, time.UTC)
	reviewTime := time.Date(2025, 10, 12, 10, 5, 0, 0, time.UTC)

	manifest := &SBIApprovalManifest{
		PBIID:       "PBI-001",
		GeneratedAt: now,
		TotalSBIs:   2,
		SBIs: []SBIApprovalRecord{
			{
				File:       "sbi_1.md",
				Status:     ApprovalStatusApproved,
				ReviewedBy: "testuser",
				ReviewedAt: &reviewTime,
				Notes:      "LGTM",
			},
			{
				File:   "sbi_2.md",
				Status: ApprovalStatusPending,
			},
		},
		Registered: false,
	}

	data, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	// Unmarshal to verify round-trip
	var decoded SBIApprovalManifest
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if decoded.PBIID != manifest.PBIID {
		t.Errorf("PBIID = %v, want %v", decoded.PBIID, manifest.PBIID)
	}

	if decoded.TotalSBIs != manifest.TotalSBIs {
		t.Errorf("TotalSBIs = %v, want %v", decoded.TotalSBIs, manifest.TotalSBIs)
	}

	if len(decoded.SBIs) != len(manifest.SBIs) {
		t.Errorf("len(SBIs) = %v, want %v", len(decoded.SBIs), len(manifest.SBIs))
	}

	// Verify first SBI with review data
	if decoded.SBIs[0].Status != ApprovalStatusApproved {
		t.Errorf("SBIs[0].Status = %v, want %v", decoded.SBIs[0].Status, ApprovalStatusApproved)
	}
	if decoded.SBIs[0].ReviewedBy != "testuser" {
		t.Errorf("SBIs[0].ReviewedBy = %v, want testuser", decoded.SBIs[0].ReviewedBy)
	}
	if decoded.SBIs[0].ReviewedAt == nil {
		t.Error("SBIs[0].ReviewedAt should not be nil")
	}

	// Verify second SBI without review data
	if decoded.SBIs[1].Status != ApprovalStatusPending {
		t.Errorf("SBIs[1].Status = %v, want %v", decoded.SBIs[1].Status, ApprovalStatusPending)
	}
	if decoded.SBIs[1].ReviewedBy != "" {
		t.Errorf("SBIs[1].ReviewedBy should be empty")
	}
	if decoded.SBIs[1].ReviewedAt != nil {
		t.Error("SBIs[1].ReviewedAt should be nil")
	}
}

func TestSBIApprovalManifest_YAMLUnmarshal(t *testing.T) {
	yamlData := `
pbi_id: PBI-001
generated_at: 2025-10-12T10:00:00Z
total_sbis: 3
sbis:
  - file: sbi_1.md
    status: approved
    reviewed_by: yoshitsugukoike
    reviewed_at: 2025-10-12T10:05:00Z
    notes: "LGTM"
  - file: sbi_2.md
    status: edited
    reviewed_by: yoshitsugukoike
    reviewed_at: 2025-10-12T10:07:00Z
    notes: "推定工数を修正"
  - file: sbi_3.md
    status: rejected
    reviewed_by: yoshitsugukoike
    reviewed_at: 2025-10-12T10:08:00Z
    rejection_reason: "要件が不明確"
registered: false
`

	var manifest SBIApprovalManifest
	if err := yaml.Unmarshal([]byte(yamlData), &manifest); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if manifest.PBIID != "PBI-001" {
		t.Errorf("PBIID = %v, want PBI-001", manifest.PBIID)
	}

	if manifest.TotalSBIs != 3 {
		t.Errorf("TotalSBIs = %v, want 3", manifest.TotalSBIs)
	}

	if len(manifest.SBIs) != 3 {
		t.Fatalf("len(SBIs) = %v, want 3", len(manifest.SBIs))
	}

	// Verify first SBI (approved)
	if manifest.SBIs[0].File != "sbi_1.md" {
		t.Errorf("SBIs[0].File = %v, want sbi_1.md", manifest.SBIs[0].File)
	}
	if manifest.SBIs[0].Status != ApprovalStatusApproved {
		t.Errorf("SBIs[0].Status = %v, want approved", manifest.SBIs[0].Status)
	}
	if manifest.SBIs[0].Notes != "LGTM" {
		t.Errorf("SBIs[0].Notes = %v, want LGTM", manifest.SBIs[0].Notes)
	}

	// Verify second SBI (edited)
	if manifest.SBIs[1].Status != ApprovalStatusEdited {
		t.Errorf("SBIs[1].Status = %v, want edited", manifest.SBIs[1].Status)
	}

	// Verify third SBI (rejected)
	if manifest.SBIs[2].Status != ApprovalStatusRejected {
		t.Errorf("SBIs[2].Status = %v, want rejected", manifest.SBIs[2].Status)
	}
	if manifest.SBIs[2].RejectionReason != "要件が不明確" {
		t.Errorf("SBIs[2].RejectionReason = %v, want 要件が不明確", manifest.SBIs[2].RejectionReason)
	}

	if manifest.Registered != false {
		t.Errorf("Registered = %v, want false", manifest.Registered)
	}

	// Verify counts
	if manifest.ApprovedCount() != 2 {
		t.Errorf("ApprovedCount() = %v, want 2", manifest.ApprovedCount())
	}
	if manifest.PendingCount() != 0 {
		t.Errorf("PendingCount() = %v, want 0", manifest.PendingCount())
	}
	if manifest.RejectedCount() != 1 {
		t.Errorf("RejectedCount() = %v, want 1", manifest.RejectedCount())
	}
}

func TestSBIApprovalStatus_Constants(t *testing.T) {
	// Verify status constant values
	if ApprovalStatusPending != "pending" {
		t.Errorf("ApprovalStatusPending = %v, want pending", ApprovalStatusPending)
	}
	if ApprovalStatusApproved != "approved" {
		t.Errorf("ApprovalStatusApproved = %v, want approved", ApprovalStatusApproved)
	}
	if ApprovalStatusRejected != "rejected" {
		t.Errorf("ApprovalStatusRejected = %v, want rejected", ApprovalStatusRejected)
	}
	if ApprovalStatusEdited != "edited" {
		t.Errorf("ApprovalStatusEdited = %v, want edited", ApprovalStatusEdited)
	}
}
