package storage_test

import (
	"context"
	"testing"

	"github.com/YoshitsuguKoike/deespec/internal/adapter/gateway/storage"
	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
)

func TestMockStorageGateway_SaveAndLoad(t *testing.T) {
	gateway := storage.NewMockStorageGateway()

	// Test SaveArtifact
	req := output.SaveArtifactRequest{
		TaskID:       "task-123",
		ArtifactType: output.ArtifactTypeCode,
		Content:      []byte("package main\n\nfunc main() {}\n"),
		ContentType:  "text/x-go",
		Metadata: map[string]string{
			"filename": "main.go",
		},
	}

	metadata, err := gateway.SaveArtifact(context.Background(), req)
	if err != nil {
		t.Fatalf("SaveArtifact() error = %v", err)
	}

	if metadata.ID == "" {
		t.Error("Artifact ID should not be empty")
	}

	if metadata.TaskID != "task-123" {
		t.Errorf("TaskID = %s, want task-123", metadata.TaskID)
	}

	if metadata.Type != output.ArtifactTypeCode {
		t.Errorf("Type = %s, want %s", metadata.Type, output.ArtifactTypeCode)
	}

	// Test LoadArtifact
	artifact, err := gateway.LoadArtifact(context.Background(), metadata.ID)
	if err != nil {
		t.Fatalf("LoadArtifact() error = %v", err)
	}

	if artifact.ID != metadata.ID {
		t.Errorf("ID = %s, want %s", artifact.ID, metadata.ID)
	}

	if string(artifact.Content) != string(req.Content) {
		t.Errorf("Content mismatch")
	}
}

func TestMockStorageGateway_LoadInstruction(t *testing.T) {
	gateway := storage.NewMockStorageGateway()

	// Test default mock instruction
	instruction, err := gateway.LoadInstruction(context.Background(), "path/to/spec.md")
	if err != nil {
		t.Fatalf("LoadInstruction() error = %v", err)
	}

	if instruction == "" {
		t.Error("Instruction should not be empty")
	}

	// Test custom instruction
	customInstr := "Custom instruction content"
	gateway.SetInstruction("custom/path", customInstr)

	instruction, err = gateway.LoadInstruction(context.Background(), "custom/path")
	if err != nil {
		t.Fatalf("LoadInstruction() error = %v", err)
	}

	if instruction != customInstr {
		t.Errorf("Instruction = %s, want %s", instruction, customInstr)
	}
}

func TestMockStorageGateway_ListArtifacts(t *testing.T) {
	gateway := storage.NewMockStorageGateway()

	// Save multiple artifacts for same task
	taskID := "task-456"
	for i := 0; i < 3; i++ {
		req := output.SaveArtifactRequest{
			TaskID:       taskID,
			ArtifactType: output.ArtifactTypeCode,
			Content:      []byte("content"),
		}
		_, err := gateway.SaveArtifact(context.Background(), req)
		if err != nil {
			t.Fatalf("SaveArtifact() error = %v", err)
		}
	}

	// Save artifact for different task
	req := output.SaveArtifactRequest{
		TaskID:       "task-789",
		ArtifactType: output.ArtifactTypeCode,
		Content:      []byte("content"),
	}
	_, err := gateway.SaveArtifact(context.Background(), req)
	if err != nil {
		t.Fatalf("SaveArtifact() error = %v", err)
	}

	// List artifacts for task-456
	artifacts, err := gateway.ListArtifacts(context.Background(), taskID)
	if err != nil {
		t.Fatalf("ListArtifacts() error = %v", err)
	}

	if len(artifacts) != 3 {
		t.Errorf("ListArtifacts() returned %d artifacts, want 3", len(artifacts))
	}

	for _, a := range artifacts {
		if a.TaskID != taskID {
			t.Errorf("Artifact TaskID = %s, want %s", a.TaskID, taskID)
		}
	}
}

func TestMockStorageGateway_LoadArtifact_NotFound(t *testing.T) {
	gateway := storage.NewMockStorageGateway()

	_, err := gateway.LoadArtifact(context.Background(), "non-existent")
	if err == nil {
		t.Error("LoadArtifact() should return error for non-existent artifact")
	}
}

func TestMockStorageGateway_Clear(t *testing.T) {
	gateway := storage.NewMockStorageGateway()

	// Save some artifacts
	req := output.SaveArtifactRequest{
		TaskID:       "task-123",
		ArtifactType: output.ArtifactTypeCode,
		Content:      []byte("content"),
	}
	gateway.SaveArtifact(context.Background(), req)

	if gateway.GetArtifactCount() != 1 {
		t.Errorf("ArtifactCount = %d, want 1", gateway.GetArtifactCount())
	}

	// Clear
	gateway.Clear()

	if gateway.GetArtifactCount() != 0 {
		t.Errorf("ArtifactCount after Clear() = %d, want 0", gateway.GetArtifactCount())
	}
}
