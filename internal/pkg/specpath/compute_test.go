package specpath

import (
	"strings"
	"testing"
)

func TestComputeSpecPath(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		title     string
		cfg       ResolvedConfig
		want      string
		wantErr   bool
	}{
		{
			name:  "basic path construction",
			id:    "TEST-001",
			title: "Test Specification",
			cfg: ResolvedConfig{
				PathBaseDir:    ".deespec/specs/sbi",
				SlugAllowChars: "a-z0-9-",
				SlugMaxLength:  60,
			},
			want: ".deespec/specs/sbi/TEST-001_test-specification",
		},
		{
			name:  "special characters in title",
			id:    "TEST-002",
			title: "Test@#$%Spec!!!",
			cfg: ResolvedConfig{
				PathBaseDir:    ".deespec/specs/sbi",
				SlugAllowChars: "a-z0-9-",
				SlugMaxLength:  60,
			},
			want: ".deespec/specs/sbi/TEST-002_test-spec",
		},
		{
			name:  "unicode normalization",
			id:    "TEST-003",
			title: "Café Naïve",
			cfg: ResolvedConfig{
				PathBaseDir:    ".deespec/specs/sbi",
				SlugAllowChars: "a-z0-9-",
				SlugMaxLength:  60,
			},
			want: ".deespec/specs/sbi/TEST-003_cafe-naive",
		},
		{
			name:  "consecutive hyphens",
			id:    "TEST-004",
			title: "Test   ---   Spec",
			cfg: ResolvedConfig{
				PathBaseDir:    ".deespec/specs/sbi",
				SlugAllowChars: "a-z0-9-",
				SlugMaxLength:  60,
			},
			want: ".deespec/specs/sbi/TEST-004_test-spec",
		},
		{
			name:  "empty title becomes untitled",
			id:    "TEST-005",
			title: "@#$%^&*()",
			cfg: ResolvedConfig{
				PathBaseDir:    ".deespec/specs/sbi",
				SlugAllowChars: "a-z0-9-",
				SlugMaxLength:  60,
			},
			want: ".deespec/specs/sbi/TEST-005_untitled",
		},
		{
			name:  "reserved name handling",
			id:    "TEST-006",
			title: "CON",
			cfg: ResolvedConfig{
				PathBaseDir:    ".deespec/specs/sbi",
				SlugAllowChars: "a-z0-9-",
				SlugMaxLength:  60,
			},
			want: ".deespec/specs/sbi/TEST-006_con-spec",
		},
		{
			name:  "length limit applied",
			id:    "TEST-007",
			title: strings.Repeat("very-long-title-", 20),
			cfg: ResolvedConfig{
				PathBaseDir:    ".deespec/specs/sbi",
				SlugAllowChars: "a-z0-9-",
				SlugMaxLength:  30,
			},
			want: ".deespec/specs/sbi/TEST-007_very-long-title-very-long-titl",
		},
		{
			name:  "custom base directory",
			id:    "TEST-008",
			title: "Custom Base",
			cfg: ResolvedConfig{
				PathBaseDir:    "custom/path",
				SlugAllowChars: "a-z0-9-",
				SlugMaxLength:  60,
			},
			want: "custom/path/TEST-008_custom-base",
		},
		{
			name:    "missing ID",
			id:      "",
			title:   "Test",
			cfg:     ResolvedConfig{},
			wantErr: true,
		},
		{
			name:    "missing title",
			id:      "TEST-010",
			title:   "",
			cfg:     ResolvedConfig{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ComputeSpecPath(tt.id, tt.title, tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ComputeSpecPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ComputeSpecPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSlugifyTitle(t *testing.T) {
	tests := []struct {
		name  string
		title string
		cfg   ResolvedConfig
		want  string
	}{
		{
			name:  "basic alphanumeric",
			title: "Hello World 123",
			cfg: ResolvedConfig{
				SlugAllowChars: "a-z0-9-",
				SlugMaxLength:  60,
			},
			want: "hello-world-123",
		},
		{
			name:  "uppercase to lowercase",
			title: "UPPERCASE",
			cfg: ResolvedConfig{
				SlugAllowChars: "a-z0-9-",
				SlugMaxLength:  60,
			},
			want: "uppercase",
		},
		{
			name:  "special characters replaced",
			title: "foo@bar#baz$qux",
			cfg: ResolvedConfig{
				SlugAllowChars: "a-z0-9-",
				SlugMaxLength:  60,
			},
			want: "foo-bar-baz-qux",
		},
		{
			name:  "leading and trailing hyphens trimmed",
			title: "---test---",
			cfg: ResolvedConfig{
				SlugAllowChars: "a-z0-9-",
				SlugMaxLength:  60,
			},
			want: "test",
		},
		{
			name:  "unicode normalized",
			title: "naïve café",
			cfg: ResolvedConfig{
				SlugAllowChars: "a-z0-9-",
				SlugMaxLength:  60,
			},
			want: "naive-cafe",
		},
		{
			name:  "japanese becomes hyphens",
			title: "テスト",
			cfg: ResolvedConfig{
				SlugAllowChars: "a-z0-9-",
				SlugMaxLength:  60,
			},
			want: "untitled",
		},
		{
			name:  "mixed content",
			title: "Test 123 テスト ABC",
			cfg: ResolvedConfig{
				SlugAllowChars: "a-z0-9-",
				SlugMaxLength:  60,
			},
			want: "test-123-abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SlugifyTitle(tt.title, tt.cfg)
			if got != tt.want {
				t.Errorf("slugifyTitle() = %v, want %v", got, tt.want)
			}
		})
	}
}