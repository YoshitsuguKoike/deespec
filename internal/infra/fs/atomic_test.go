package fs

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAppendNDJSONLine(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "atomic_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	jsonPath := filepath.Join(tmpDir, "test.ndjson")

	// Test appending a single record
	record := map[string]interface{}{
		"id":   1,
		"name": "test",
		"tags": []string{"a", "b"},
	}

	err = AppendNDJSONLine(jsonPath, record)
	if err != nil {
		t.Fatalf("AppendNDJSONLine failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Errorf("NDJSON file was not created")
	}

	// Read and verify content
	file, err := os.Open(jsonPath)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
		line := scanner.Text()

		var decoded map[string]interface{}
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Fatalf("Failed to decode JSON line: %v", err)
		}

		if decoded["id"].(float64) != 1 {
			t.Errorf("Expected id=1, got %v", decoded["id"])
		}
		if decoded["name"].(string) != "test" {
			t.Errorf("Expected name=test, got %v", decoded["name"])
		}
	}

	if lineCount != 1 {
		t.Errorf("Expected 1 line, got %d", lineCount)
	}
}

func TestAppendNDJSONLine_Multiple(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "atomic_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	jsonPath := filepath.Join(tmpDir, "test.ndjson")

	// Append multiple records
	for i := 1; i <= 5; i++ {
		record := map[string]interface{}{
			"id":    i,
			"value": i * 10,
		}

		err = AppendNDJSONLine(jsonPath, record)
		if err != nil {
			t.Fatalf("AppendNDJSONLine failed for record %d: %v", i, err)
		}
	}

	// Read and verify all lines
	file, err := os.Open(jsonPath)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	expectedID := 1
	for scanner.Scan() {
		lineCount++
		line := scanner.Text()

		var decoded map[string]interface{}
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Fatalf("Failed to decode JSON line %d: %v", lineCount, err)
		}

		if int(decoded["id"].(float64)) != expectedID {
			t.Errorf("Line %d: Expected id=%d, got %v", lineCount, expectedID, decoded["id"])
		}
		expectedID++
	}

	if lineCount != 5 {
		t.Errorf("Expected 5 lines, got %d", lineCount)
	}
}

func TestAppendNDJSONLine_CreateDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "atomic_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test with nested directory that doesn't exist
	jsonPath := filepath.Join(tmpDir, "nested", "dir", "test.ndjson")

	record := map[string]interface{}{
		"test": "value",
	}

	err = AppendNDJSONLine(jsonPath, record)
	if err != nil {
		t.Fatalf("AppendNDJSONLine failed: %v", err)
	}

	// Verify file exists in nested directory
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Errorf("NDJSON file was not created in nested directory")
	}

	// Verify content
	file, err := os.Open(jsonPath)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Error("Expected at least one line in file")
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(scanner.Bytes(), &decoded); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	if decoded["test"] != "value" {
		t.Errorf("Expected test=value, got %v", decoded["test"])
	}
}

func TestAppendNDJSONLine_InvalidJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "atomic_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	jsonPath := filepath.Join(tmpDir, "test.ndjson")

	// Try to append something that can't be marshaled to JSON
	// (channels cannot be marshaled)
	type InvalidRecord struct {
		Ch chan int
	}

	record := InvalidRecord{
		Ch: make(chan int),
	}

	err = AppendNDJSONLine(jsonPath, record)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestAppendNDJSONLine_EmptyRecord(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "atomic_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	jsonPath := filepath.Join(tmpDir, "test.ndjson")

	// Append empty record
	record := map[string]interface{}{}

	err = AppendNDJSONLine(jsonPath, record)
	if err != nil {
		t.Fatalf("AppendNDJSONLine failed: %v", err)
	}

	// Verify content
	content, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := "{}\n"
	if string(content) != expected {
		t.Errorf("Expected '%s', got '%s'", expected, string(content))
	}
}

func TestAppendNDJSONLine_NestedStructures(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "atomic_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	jsonPath := filepath.Join(tmpDir, "test.ndjson")

	// Test with nested structures
	record := map[string]interface{}{
		"id": 1,
		"metadata": map[string]interface{}{
			"tags":   []string{"a", "b", "c"},
			"nested": map[string]int{"x": 1, "y": 2},
		},
		"array": []interface{}{1, "two", true, nil},
	}

	err = AppendNDJSONLine(jsonPath, record)
	if err != nil {
		t.Fatalf("AppendNDJSONLine failed: %v", err)
	}

	// Read and verify
	file, err := os.Open(jsonPath)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("Expected at least one line in file")
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(scanner.Bytes(), &decoded); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	// Verify nested metadata
	metadata, ok := decoded["metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("Failed to decode metadata")
	}

	tags, ok := metadata["tags"].([]interface{})
	if !ok || len(tags) != 3 {
		t.Errorf("Expected tags array with 3 elements, got %v", tags)
	}
}

func TestAtomicWriteJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "atomic_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	jsonPath := filepath.Join(tmpDir, "test.json")

	// Test writing JSON
	data := map[string]interface{}{
		"id":   1,
		"name": "test",
		"tags": []string{"a", "b"},
	}

	err = AtomicWriteJSON(jsonPath, data)
	if err != nil {
		t.Fatalf("AtomicWriteJSON failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Errorf("JSON file was not created")
	}

	// Read and verify content
	content, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(content, &decoded); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	if decoded["id"].(float64) != 1 {
		t.Errorf("Expected id=1, got %v", decoded["id"])
	}
	if decoded["name"].(string) != "test" {
		t.Errorf("Expected name=test, got %v", decoded["name"])
	}
}

func TestAtomicWriteJSON_CreateDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "atomic_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test with nested directory that doesn't exist
	jsonPath := filepath.Join(tmpDir, "nested", "dir", "test.json")

	data := map[string]interface{}{
		"test": "value",
	}

	err = AtomicWriteJSON(jsonPath, data)
	if err != nil {
		t.Fatalf("AtomicWriteJSON failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Errorf("JSON file was not created in nested directory")
	}
}

func TestAtomicWriteJSON_Overwrite(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "atomic_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	jsonPath := filepath.Join(tmpDir, "test.json")

	// Write initial data
	data1 := map[string]interface{}{"version": 1}
	err = AtomicWriteJSON(jsonPath, data1)
	if err != nil {
		t.Fatalf("First AtomicWriteJSON failed: %v", err)
	}

	// Overwrite with new data
	data2 := map[string]interface{}{"version": 2}
	err = AtomicWriteJSON(jsonPath, data2)
	if err != nil {
		t.Fatalf("Second AtomicWriteJSON failed: %v", err)
	}

	// Verify new data
	content, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(content, &decoded); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	if decoded["version"].(float64) != 2 {
		t.Errorf("Expected version=2, got %v", decoded["version"])
	}
}
