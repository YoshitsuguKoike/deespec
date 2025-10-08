package presenter_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/YoshitsuguKoike/deespec/internal/adapter/presenter"
	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
)

func TestJSONPresenter_PresentSuccess(t *testing.T) {
	buf := &bytes.Buffer{}
	p := presenter.NewJSONPresenter(buf)

	data := &dto.TaskDTO{
		ID:    "task-123",
		Type:  "SBI",
		Title: "Test Task",
	}

	err := p.PresentSuccess("Task created", data)
	if err != nil {
		t.Fatalf("PresentSuccess() error = %v", err)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(buf).Decode(&result); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	if result["success"] != true {
		t.Errorf("Expected success=true, got %v", result["success"])
	}

	if result["message"] != "Task created" {
		t.Errorf("Expected message='Task created', got %v", result["message"])
	}

	if result["data"] == nil {
		t.Error("Expected data to be present")
	}
}

func TestJSONPresenter_PresentError(t *testing.T) {
	buf := &bytes.Buffer{}
	p := presenter.NewJSONPresenter(buf)

	testErr := errors.New("test error")
	err := p.PresentError(testErr)
	if err != nil {
		t.Fatalf("PresentError() error = %v", err)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(buf).Decode(&result); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	if result["success"] != false {
		t.Errorf("Expected success=false, got %v", result["success"])
	}

	if result["error"] != "test error" {
		t.Errorf("Expected error='test error', got %v", result["error"])
	}
}

func TestJSONPresenter_PresentProgress(t *testing.T) {
	buf := &bytes.Buffer{}
	p := presenter.NewJSONPresenter(buf)

	err := p.PresentProgress("Processing", 3, 10)
	if err != nil {
		t.Fatalf("PresentProgress() error = %v", err)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(buf).Decode(&result); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	if result["type"] != "progress" {
		t.Errorf("Expected type='progress', got %v", result["type"])
	}

	if result["progress"] != float64(3) {
		t.Errorf("Expected progress=3, got %v", result["progress"])
	}

	if result["total"] != float64(10) {
		t.Errorf("Expected total=10, got %v", result["total"])
	}

	if result["percent"] != 30.0 {
		t.Errorf("Expected percent=30.0, got %v", result["percent"])
	}
}
