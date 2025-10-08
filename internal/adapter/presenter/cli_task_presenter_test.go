package presenter_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/YoshitsuguKoike/deespec/internal/adapter/presenter"
	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
)

func TestCLITaskPresenter_PresentSuccess(t *testing.T) {
	tests := []struct {
		name        string
		message     string
		data        interface{}
		wantContain []string
	}{
		{
			name:    "present SBI",
			message: "SBI created",
			data: &dto.SBIDTO{
				TaskDTO: dto.TaskDTO{
					ID:     "sbi-123",
					Type:   "SBI",
					Title:  "Test SBI",
					Status: "pending",
				},
				CurrentTurn:    1,
				MaxTurns:       10,
				CurrentAttempt: 1,
				MaxAttempts:    3,
			},
			wantContain: []string{"✓ SBI created", "SBI: Test SBI", "ID: sbi-123", "Turn: 1/10", "Attempt: 1/3"},
		},
		{
			name:    "present task list",
			message: "Tasks listed",
			data: &dto.ListTasksResponse{
				Tasks: []dto.TaskDTO{
					{ID: "task-1", Type: "EPIC", Title: "Epic 1", Status: "pending"},
					{ID: "task-2", Type: "PBI", Title: "PBI 1", Status: "in_progress"},
				},
				TotalCount: 2,
			},
			wantContain: []string{"✓ Tasks listed", "Total: 2 tasks", "[EPIC] Epic 1", "[PBI] PBI 1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			p := presenter.NewCLITaskPresenter(buf)

			err := p.PresentSuccess(tt.message, tt.data)
			if err != nil {
				t.Fatalf("PresentSuccess() error = %v", err)
			}

			output := buf.String()
			for _, want := range tt.wantContain {
				if !strings.Contains(output, want) {
					t.Errorf("Output does not contain %q\nGot: %s", want, output)
				}
			}
		})
	}
}

func TestCLITaskPresenter_PresentError(t *testing.T) {
	buf := &bytes.Buffer{}
	p := presenter.NewCLITaskPresenter(buf)

	err := p.PresentError(nil)
	if err != nil {
		t.Fatalf("PresentError() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "✗ Error:") {
		t.Errorf("Output should contain error marker\nGot: %s", output)
	}
}

func TestCLITaskPresenter_PresentProgress(t *testing.T) {
	buf := &bytes.Buffer{}
	p := presenter.NewCLITaskPresenter(buf)

	err := p.PresentProgress("Processing", 5, 10)
	if err != nil {
		t.Fatalf("PresentProgress() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Processing") {
		t.Errorf("Output should contain message\nGot: %s", output)
	}
	if !strings.Contains(output, "50.0%") {
		t.Errorf("Output should contain percentage\nGot: %s", output)
	}
}
