package workflow

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/validator/agents"
	"gopkg.in/yaml.v3"
)

type Workflow struct {
	Name        string       `yaml:"name"`
	Steps       []Step       `yaml:"steps"`
	Constraints *Constraints `yaml:"constraints,omitempty"`
}

type Step struct {
	ID         string                 `yaml:"id"`
	Agent      string                 `yaml:"agent"`
	PromptPath string                 `yaml:"prompt_path"`
	Extra      map[string]interface{} `yaml:",inline"`
}

type Constraints struct {
	MaxPromptKB int `yaml:"max_prompt_kb,omitempty"`
}

type Issue struct {
	Type    string `json:"type"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

type FileResult struct {
	File   string  `json:"file"`
	Issues []Issue `json:"issues"`
}

type Summary struct {
	Files int `json:"files"`
	OK    int `json:"ok"`
	Warn  int `json:"warn"`
	Error int `json:"error"`
}

type ValidationResult struct {
	Version      int          `json:"version"`
	GeneratedAt  string       `json:"generated_at"`
	Files        []FileResult `json:"files"`
	AgentsSource string       `json:"agents_source,omitempty"`
	Summary      Summary      `json:"summary"`
}

type Validator struct {
	basePath string
}

func NewValidator(basePath string) *Validator {
	return &Validator{basePath: basePath}
}

func (v *Validator) Validate(path string) (*ValidationResult, error) {
	result := &ValidationResult{
		Version:     1,
		GeneratedAt: time.Now().UTC().Format("2006-01-02T15:04:05.000000Z"),
		Files:       []FileResult{},
		Summary:     Summary{Files: 1},
	}

	// Load agents configuration
	agentsPath := filepath.Join(v.basePath, "etc", "agents.yaml")
	agentsResult, err := agents.LoadAgents(agentsPath)
	if err != nil {
		// Treat loader error as validation error
		fileResult := FileResult{
			File: filepath.Base(path),
			Issues: []Issue{{
				Type:    "error",
				Field:   "/",
				Message: fmt.Sprintf("cannot load agents: %v", err),
			}},
		}
		result.Files = append(result.Files, fileResult)
		result.Summary.Error++
		return result, nil
	}

	// Set agents source in result
	result.AgentsSource = agentsResult.Source

	// Convert agents to map for validation
	validAgents := agents.ToMap(agentsResult.Agents)

	// Check for issues in agents.yaml
	if len(agentsResult.Issues) > 0 {
		fileResult := FileResult{
			File: "agents.yaml",
			Issues: []Issue{},
		}
		for _, issue := range agentsResult.Issues {
			fileResult.Issues = append(fileResult.Issues, Issue{
				Type:    issue.Type,
				Field:   issue.Field,
				Message: issue.Message,
			})
			if issue.Type == "error" {
				result.Summary.Error++
			}
		}
		result.Files = append(result.Files, fileResult)
		// Continue with validation even if agents.yaml has issues
	}

	relPath, err := filepath.Rel(".", path)
	if err != nil {
		relPath = path
	}

	fileResult := FileResult{
		File:   relPath,
		Issues: []Issue{},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		fileResult.Issues = append(fileResult.Issues, Issue{
			Type:    "error",
			Field:   "/",
			Message: fmt.Sprintf("cannot read file: %v", err),
		})
		result.Summary.Error++
		result.Files = append(result.Files, fileResult)
		return result, nil
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)

	var wf Workflow
	if err := dec.Decode(&wf); err != nil {
		if strings.Contains(err.Error(), "unknown field") {
			parts := strings.Split(err.Error(), "\"")
			if len(parts) >= 2 {
				fieldName := parts[1]
				fileResult.Issues = append(fileResult.Issues, Issue{
					Type:    "error",
					Field:   "/",
					Message: fmt.Sprintf("unknown field: %s", fieldName),
				})
			} else {
				fileResult.Issues = append(fileResult.Issues, Issue{
					Type:    "error",
					Field:   "/",
					Message: fmt.Sprintf("invalid yaml: %v", err),
				})
			}
		} else {
			fileResult.Issues = append(fileResult.Issues, Issue{
				Type:    "error",
				Field:   "/",
				Message: fmt.Sprintf("invalid yaml: %v", err),
			})
		}
		result.Summary.Error++
		result.Files = append(result.Files, fileResult)
		return result, nil
	}

	if len(wf.Steps) == 0 {
		fileResult.Issues = append(fileResult.Issues, Issue{
			Type:    "error",
			Field:   "/steps",
			Message: "steps array is required and cannot be empty",
		})
		result.Summary.Error++
	}

	ids := make(map[string]bool)
	for i, step := range wf.Steps {
		// Check for unknown fields
		if len(step.Extra) > 0 {
			for key := range step.Extra {
				fileResult.Issues = append(fileResult.Issues, Issue{
					Type:    "error",
					Field:   fmt.Sprintf("/steps/%d/%s", i, key),
					Message: fmt.Sprintf("unknown field: %s", key),
				})
				result.Summary.Error++
			}
		}
		if step.ID == "" {
			fileResult.Issues = append(fileResult.Issues, Issue{
				Type:    "error",
				Field:   fmt.Sprintf("/steps/%d/id", i),
				Message: "id is required",
			})
			result.Summary.Error++
		} else if ids[step.ID] {
			fileResult.Issues = append(fileResult.Issues, Issue{
				Type:    "error",
				Field:   fmt.Sprintf("/steps/%d/id", i),
				Message: fmt.Sprintf("duplicate id: %s", step.ID),
			})
			result.Summary.Error++
		} else {
			ids[step.ID] = true
		}

		if step.Agent == "" {
			fileResult.Issues = append(fileResult.Issues, Issue{
				Type:    "error",
				Field:   fmt.Sprintf("/steps/%d/agent", i),
				Message: "agent is required",
			})
			result.Summary.Error++
		} else if !validAgents[step.Agent] {
			fileResult.Issues = append(fileResult.Issues, Issue{
				Type:    "error",
				Field:   fmt.Sprintf("/steps/%d/agent", i),
				Message: fmt.Sprintf("unknown: %s (not in agents set)", step.Agent),
			})
			result.Summary.Error++
		}

		if step.PromptPath == "" {
			fileResult.Issues = append(fileResult.Issues, Issue{
				Type:    "error",
				Field:   fmt.Sprintf("/steps/%d/prompt_path", i),
				Message: "prompt_path is required",
			})
			result.Summary.Error++
		} else {
			if err := v.validatePromptPath(step.PromptPath); err != nil {
				fileResult.Issues = append(fileResult.Issues, Issue{
					Type:    "error",
					Field:   fmt.Sprintf("/steps/%d/prompt_path", i),
					Message: err.Error(),
				})
				result.Summary.Error++
			}
		}
	}

	if wf.Constraints != nil {
		if wf.Constraints.MaxPromptKB != 0 {
			if wf.Constraints.MaxPromptKB < 1 || wf.Constraints.MaxPromptKB > 512 {
				fileResult.Issues = append(fileResult.Issues, Issue{
					Type:    "error",
					Field:   "/constraints/max_prompt_kb",
					Message: fmt.Sprintf("must be between 1 and 512, got %d", wf.Constraints.MaxPromptKB),
				})
				result.Summary.Error++
			}
		}
	}

	if result.Summary.Error == 0 && result.Summary.Warn == 0 {
		result.Summary.OK = 1
	}

	result.Files = append(result.Files, fileResult)
	return result, nil
}

func (v *Validator) validatePromptPath(path string) error {
	if filepath.IsAbs(path) {
		return fmt.Errorf("absolute path not allowed: %s", path)
	}

	if strings.Contains(path, "..") {
		return fmt.Errorf("parent directory reference not allowed: %s", path)
	}

	fullPath := filepath.Join(v.basePath, path)

	info, err := os.Lstat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", path)
		}
		return fmt.Errorf("cannot access file: %s", path)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		target, err := filepath.EvalSymlinks(fullPath)
		if err != nil {
			return fmt.Errorf("cannot resolve symlink: %s", path)
		}

		// Get absolute paths for comparison
		absBase, err := filepath.Abs(v.basePath)
		if err != nil {
			return fmt.Errorf("cannot resolve base path: %s", path)
		}

		absTarget, err := filepath.Abs(target)
		if err != nil {
			return fmt.Errorf("cannot resolve target path: %s", path)
		}

		// Check if target is within basePath by checking if absTarget starts with absBase
		if !strings.HasPrefix(absTarget, absBase+string(filepath.Separator)) && absTarget != absBase {
			return fmt.Errorf("symlink points outside .deespec/: %s", path)
		}
	}

	return nil
}