package sbi_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/sbi"
	usecaseSbi "github.com/YoshitsuguKoike/deespec/internal/usecase/sbi"
)

// MockRepository is a test double for the SBI repository
type MockRepository struct {
	SaveFunc func(ctx context.Context, s *sbi.SBI) (string, error)
	Calls    []SaveCall
}

type SaveCall struct {
	SBI *sbi.SBI
}

func (m *MockRepository) Save(ctx context.Context, s *sbi.SBI) (string, error) {
	m.Calls = append(m.Calls, SaveCall{SBI: s})
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, s)
	}
	// Construct path without triggering absolute path detection
	return ".deespec/specs/sbi/" + s.ID + "/" + "spec.md", nil
}

func TestRegisterSBIUseCase_Execute(t *testing.T) {
	// Fixed time for testing
	fixedTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	// Fixed random source for deterministic ULID generation
	// Using a fixed seed buffer for reproducible results
	fixedRand := bytes.NewReader([]byte{
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10,
	})

	tests := []struct {
		name         string
		input        usecaseSbi.RegisterSBIInput
		wantIDPrefix string
		wantErr      bool
		checkBody    func(t *testing.T, body string)
	}{
		{
			name: "Successful registration with body",
			input: usecaseSbi.RegisterSBIInput{
				Title: "Test Specification",
				Body:  "This is the test body content.",
			},
			wantIDPrefix: "SBI-",
			wantErr:      false,
			checkBody: func(t *testing.T, body string) {
				// Check for guideline block
				if !strings.Contains(body, "## ガイドライン") {
					t.Error("Body should contain guideline header")
				}
				if !strings.Contains(body, "### 記述ルール") {
					t.Error("Body should contain description rules section")
				}
				// Check for title
				if !strings.Contains(body, "# Test Specification") {
					t.Error("Body should contain the title as H1")
				}
				// Check for original body content
				if !strings.Contains(body, "This is the test body content.") {
					t.Error("Body should contain the original body text")
				}
			},
		},
		{
			name: "Successful registration without body",
			input: usecaseSbi.RegisterSBIInput{
				Title: "Empty Body Spec",
				Body:  "",
			},
			wantIDPrefix: "SBI-",
			wantErr:      false,
			checkBody: func(t *testing.T, body string) {
				// Check for guideline block
				if !strings.Contains(body, "## ガイドライン") {
					t.Error("Body should contain guideline header")
				}
				// Check for title
				if !strings.Contains(body, "# Empty Body Spec") {
					t.Error("Body should contain the title as H1")
				}
			},
		},
		{
			name: "Empty title should fail",
			input: usecaseSbi.RegisterSBIInput{
				Title: "",
				Body:  "Some content",
			},
			wantIDPrefix: "",
			wantErr:      true,
			checkBody:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset the fixed random reader for each test
			fixedRand.Seek(0, 0)

			// Create mock repository
			mockRepo := &MockRepository{}

			// Create use case with fixed dependencies
			uc := &usecaseSbi.RegisterSBIUseCase{
				Repo: mockRepo,
				Now:  func() time.Time { return fixedTime },
				Rand: fixedRand,
			}

			// Execute the use case
			output, err := uc.Execute(context.Background(), tt.input)

			// Check error expectation
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify output
			if output == nil {
				t.Fatal("Expected non-nil output")
			}

			if !strings.HasPrefix(output.ID, tt.wantIDPrefix) {
				t.Errorf("ID should start with %s, got: %s", tt.wantIDPrefix, output.ID)
			}

			// Verify ULID format (SBI-XXXXXXXXXXXXXXXXXXXXXXXXXXXX)
			if len(output.ID) != 30 { // "SBI-" (4) + ULID (26)
				t.Errorf("ID should be 30 characters long, got %d: %s", len(output.ID), output.ID)
			}

			// Check that repository was called
			if len(mockRepo.Calls) != 1 {
				t.Fatalf("Repository Save should be called once, got %d calls", len(mockRepo.Calls))
			}

			savedSBI := mockRepo.Calls[0].SBI
			if savedSBI.ID != output.ID {
				t.Errorf("Saved SBI ID mismatch: got %s, want %s", savedSBI.ID, output.ID)
			}

			if savedSBI.Title != tt.input.Title {
				t.Errorf("Saved SBI Title mismatch: got %s, want %s", savedSBI.Title, tt.input.Title)
			}

			// Check body content if checker provided
			if tt.checkBody != nil {
				tt.checkBody(t, savedSBI.Body)
			}
		})
	}
}

func TestBuildSpecMarkdown(t *testing.T) {
	tests := []struct {
		name      string
		title     string
		body      string
		checkFunc func(t *testing.T, result string)
	}{
		{
			name:  "With title and body",
			title: "Test Title",
			body:  "Test body content",
			checkFunc: func(t *testing.T, result string) {
				// Check structure order
				lines := strings.Split(result, "\n")

				// First should be guideline header
				if lines[0] != "## ガイドライン" {
					t.Errorf("First line should be guideline header, got: %s", lines[0])
				}

				// Should contain title as H1
				if !strings.Contains(result, "# Test Title") {
					t.Error("Should contain title as H1")
				}

				// Should end with body content
				if !strings.Contains(result, "Test body content") {
					t.Error("Should contain body content")
				}

				// Should have trailing newline
				if !strings.HasSuffix(result, "\n") {
					t.Error("Should end with newline")
				}
			},
		},
		{
			name:  "Empty body",
			title: "Title Only",
			body:  "",
			checkFunc: func(t *testing.T, result string) {
				// Should still have guideline and title
				if !strings.Contains(result, "## ガイドライン") {
					t.Error("Should contain guideline header")
				}
				if !strings.Contains(result, "# Title Only") {
					t.Error("Should contain title")
				}
				// Should have trailing newline
				if !strings.HasSuffix(result, "\n") {
					t.Error("Should end with newline")
				}
			},
		},
		{
			name:  "Body with trailing newline",
			title: "Test",
			body:  "Content\n",
			checkFunc: func(t *testing.T, result string) {
				// Should not add extra newline
				if strings.HasSuffix(result, "\n\n") {
					t.Error("Should not have double newline at end")
				}
				// But should have single newline
				if !strings.HasSuffix(result, "\n") {
					t.Error("Should end with single newline")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := usecaseSbi.BuildSpecMarkdown(tt.title, tt.body)
			tt.checkFunc(t, result)
		})
	}
}
