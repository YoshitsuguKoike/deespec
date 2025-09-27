package workflow

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/YoshitsuguKoike/deespec/internal/app"
	"github.com/YoshitsuguKoike/deespec/internal/app/state"
)

func TestValidatePlaceholders(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		allowed     []string
		wantUnknown []string
		wantUsed    []string
	}{
		{
			name:        "all allowed placeholders",
			text:        "turn={turn} id={task_id} pj={project_name} lang={language}",
			allowed:     Allowed,
			wantUnknown: nil,
			wantUsed:    []string{"turn", "task_id", "project_name", "language"},
		},
		{
			name:        "unknown placeholder",
			text:        "valid={turn} invalid={foo}",
			allowed:     Allowed,
			wantUnknown: []string{"foo"},
			wantUsed:    []string{"turn"},
		},
		{
			name:        "escaped placeholder ignored",
			text:        "escaped=\\{notvar} valid={turn}",
			allowed:     Allowed,
			wantUnknown: nil,
			wantUsed:    []string{"turn"},
		},
		{
			name:        "multiple unknown",
			text:        "{foo} {bar} {turn}",
			allowed:     Allowed,
			wantUnknown: []string{"foo", "bar"},
			wantUsed:    []string{"turn"},
		},
		{
			name:        "duplicate placeholders counted once",
			text:        "{turn} {turn} {foo} {foo}",
			allowed:     Allowed,
			wantUnknown: []string{"foo"},
			wantUsed:    []string{"turn"},
		},
		{
			name:        "no placeholders",
			text:        "plain text without placeholders",
			allowed:     Allowed,
			wantUnknown: nil,
			wantUsed:    nil,
		},
		{
			name:        "mixed valid and invalid names",
			text:        "{turn} {_invalid} {task_id} {123invalid}",
			allowed:     Allowed,
			wantUnknown: []string{"_invalid"},
			wantUsed:    []string{"turn", "task_id"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unknown, used := ValidatePlaceholders(tt.text, tt.allowed)

			// Sort for consistent comparison
			sort.Strings(unknown)
			sort.Strings(used)
			if tt.wantUnknown != nil {
				sort.Strings(tt.wantUnknown)
			}
			if tt.wantUsed != nil {
				sort.Strings(tt.wantUsed)
			}

			if !reflect.DeepEqual(unknown, tt.wantUnknown) {
				t.Errorf("ValidatePlaceholders() unknown = %v, want %v", unknown, tt.wantUnknown)
			}
			if !reflect.DeepEqual(used, tt.wantUsed) {
				t.Errorf("ValidatePlaceholders() used = %v, want %v", used, tt.wantUsed)
			}
		})
	}
}

func TestExpandPrompt(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		vars    map[string]string
		want    string
		wantErr bool
	}{
		{
			name: "successful expansion",
			text: "t={turn} id={task_id} pj={project_name} lang={language}",
			vars: map[string]string{
				"turn":         "1",
				"task_id":      "TEST-001",
				"project_name": "myproject",
				"language":     "en",
			},
			want:    "t=1 id=TEST-001 pj=myproject lang=en",
			wantErr: false,
		},
		{
			name: "unknown placeholder error",
			text: "valid={turn} invalid={foo}",
			vars: map[string]string{
				"turn": "1",
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "escaped braces preserved",
			text: "escaped \\{not_a_var} and {turn}",
			vars: map[string]string{
				"turn": "2",
			},
			want:    "escaped {not_a_var} and 2",
			wantErr: false,
		},
		{
			name: "empty values allowed",
			text: "task={task_id} turn={turn}",
			vars: map[string]string{
				"task_id": "",
				"turn":    "0",
			},
			want:    "task= turn=0",
			wantErr: false,
		},
		{
			name: "multiple same placeholder",
			text: "{turn} {turn} {turn}",
			vars: map[string]string{
				"turn": "3",
			},
			want:    "3 3 3",
			wantErr: false,
		},
		{
			name: "no placeholders",
			text: "plain text without placeholders",
			vars: map[string]string{
				"turn": "1",
			},
			want:    "plain text without placeholders",
			wantErr: false,
		},
		{
			name: "multiline text",
			text: "Line 1: {turn}\nLine 2: {task_id}\nLine 3: {language}",
			vars: map[string]string{
				"turn":     "1",
				"task_id":  "TEST",
				"language": "ja",
			},
			want:    "Line 1: 1\nLine 2: TEST\nLine 3: ja",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExpandPrompt(tt.text, tt.vars)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExpandPrompt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ExpandPrompt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildVarMap(t *testing.T) {
	// Environment variables are no longer used
	// Config should be passed through BuildVarMapWithConfig instead

	// Get the current working directory for project name
	wd, _ := os.Getwd()
	defaultProjectName := filepath.Base(wd)

	tests := []struct {
		name     string
		wfVars   map[string]string
		state    *state.State
		wantVars map[string]string
	}{
		{
			name:   "defaults only",
			wfVars: nil,
			state: &state.State{
				Turn: 5,
				Meta: map[string]interface{}{
					"task_id": "TASK-123",
				},
			},
			wantVars: map[string]string{
				"turn":         "5",
				"task_id":      "TASK-123",
				"project_name": defaultProjectName,
				"language":     "ja",
			},
		},
		{
			name: "workflow vars override defaults",
			wfVars: map[string]string{
				"project_name": "custom-project",
				"language":     "en",
			},
			state: &state.State{
				Turn: 3,
				Meta: map[string]interface{}{},
			},
			wantVars: map[string]string{
				"turn":         "3",
				"task_id":      "",
				"project_name": "custom-project",
				"language":     "en",
			},
		},
		{
			name:   "nil state handled",
			wfVars: nil,
			state:  nil,
			wantVars: map[string]string{
				"turn":         "0",
				"task_id":      "",
				"project_name": defaultProjectName,
				"language":     "ja",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Environment variables are no longer used
			// Config is passed through BuildVarMapWithConfig instead

			// Call BuildVarMap
			ctx := context.Background()
			paths := app.GetPaths() // Use default paths
			got := BuildVarMap(ctx, paths, tt.wfVars, tt.state)

			// Compare
			if !reflect.DeepEqual(got, tt.wantVars) {
				t.Errorf("BuildVarMap() = %v, want %v", got, tt.wantVars)
			}
		})
	}
}
