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
				SlugNFKC:       true,
				SlugLowercase:  true,
				SlugAllowChars: "a-z0-9-",
				SlugMaxRunes:   60,
			},
			want: ".deespec/specs/sbi/TEST-001_test-specification",
		},
		{
			name:  "special characters in title",
			id:    "TEST-002",
			title: "Test@#$%Spec!!!",
			cfg: ResolvedConfig{
				PathBaseDir:    ".deespec/specs/sbi",
				SlugNFKC:       true,
				SlugLowercase:  true,
				SlugAllowChars: "a-z0-9-",
				SlugMaxRunes:   60,
			},
			want: ".deespec/specs/sbi/TEST-002_test-spec",
		},
		{
			name:  "unicode normalization",
			id:    "TEST-003",
			title: "Café Naïve",
			cfg: ResolvedConfig{
				PathBaseDir:    ".deespec/specs/sbi",
				SlugNFKC:       true,
				SlugLowercase:  true,
				SlugAllowChars: "a-z0-9-",
				SlugMaxRunes:   60,
			},
			want: ".deespec/specs/sbi/TEST-003_cafe-naive",
		},
		{
			name:  "consecutive hyphens",
			id:    "TEST-004",
			title: "Test   ---   Spec",
			cfg: ResolvedConfig{
				PathBaseDir:    ".deespec/specs/sbi",
				SlugNFKC:       true,
				SlugLowercase:  true,
				SlugAllowChars: "a-z0-9-",
				SlugMaxRunes:   60,
			},
			want: ".deespec/specs/sbi/TEST-004_test-spec",
		},
		{
			name:  "empty title becomes untitled",
			id:    "TEST-005",
			title: "@#$%^&*()",
			cfg: ResolvedConfig{
				PathBaseDir:    ".deespec/specs/sbi",
				SlugNFKC:       true,
				SlugLowercase:  true,
				SlugAllowChars: "a-z0-9-",
				SlugMaxRunes:   60,
				SlugFallback:   "untitled",
			},
			want: ".deespec/specs/sbi/TEST-005_untitled",
		},
		{
			name:  "reserved name handling",
			id:    "TEST-006",
			title: "CON",
			cfg: ResolvedConfig{
				PathBaseDir:               ".deespec/specs/sbi",
				SlugNFKC:                  true,
				SlugLowercase:             true,
				SlugAllowChars:            "a-z0-9-",
				SlugMaxRunes:              60,
				SlugWindowsReservedSuffix: "-spec",
			},
			want: ".deespec/specs/sbi/TEST-006_con-spec",
		},
		{
			name:  "length limit applied",
			id:    "TEST-007",
			title: strings.Repeat("very-long-title-", 20),
			cfg: ResolvedConfig{
				PathBaseDir:    ".deespec/specs/sbi",
				SlugNFKC:       true,
				SlugLowercase:  true,
				SlugAllowChars: "a-z0-9-",
				SlugMaxRunes:   30,
			},
			want: ".deespec/specs/sbi/TEST-007_very-long-title-very-long-titl",
		},
		{
			name:  "custom base directory",
			id:    "TEST-008",
			title: "Custom Base",
			cfg: ResolvedConfig{
				PathBaseDir:    "custom/path",
				SlugNFKC:       true,
				SlugLowercase:  true,
				SlugAllowChars: "a-z0-9-",
				SlugMaxRunes:   60,
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
				SlugNFKC:       true,
				SlugLowercase:  true,
				SlugAllowChars: "a-z0-9-",
				SlugMaxRunes:   60,
			},
			want: "hello-world-123",
		},
		{
			name:  "uppercase to lowercase",
			title: "UPPERCASE",
			cfg: ResolvedConfig{
				SlugNFKC:       true,
				SlugLowercase:  true,
				SlugAllowChars: "a-z0-9-",
				SlugMaxRunes:   60,
			},
			want: "uppercase",
		},
		{
			name:  "special characters replaced",
			title: "foo@bar#baz$qux",
			cfg: ResolvedConfig{
				SlugNFKC:       true,
				SlugLowercase:  true,
				SlugAllowChars: "a-z0-9-",
				SlugMaxRunes:   60,
			},
			want: "foo-bar-baz-qux",
		},
		{
			name:  "leading and trailing hyphens trimmed",
			title: "---test---",
			cfg: ResolvedConfig{
				SlugNFKC:       true,
				SlugLowercase:  true,
				SlugAllowChars: "a-z0-9-",
				SlugMaxRunes:   60,
			},
			want: "test",
		},
		{
			name:  "unicode normalized",
			title: "naïve café",
			cfg: ResolvedConfig{
				SlugNFKC:       true,
				SlugLowercase:  true,
				SlugAllowChars: "a-z0-9-",
				SlugMaxRunes:   60,
			},
			want: "naive-cafe",
		},
		{
			name:  "japanese becomes hyphens",
			title: "テスト",
			cfg: ResolvedConfig{
				SlugNFKC:       true,
				SlugLowercase:  true,
				SlugAllowChars: "a-z0-9-",
				SlugMaxRunes:   60,
				SlugFallback:   "spec",
			},
			want: "spec",
		},
		{
			name:  "mixed content",
			title: "Test 123 テスト ABC",
			cfg: ResolvedConfig{
				SlugNFKC:       true,
				SlugLowercase:  true,
				SlugAllowChars: "a-z0-9-",
				SlugMaxRunes:   60,
			},
			want: "test-123-abc",
		},
		{
			name:  "windows reserved name with suffix",
			title: "CON",
			cfg: ResolvedConfig{
				SlugNFKC:                  true,
				SlugLowercase:             true,
				SlugAllowChars:            "a-z0-9-",
				SlugMaxRunes:              60,
				SlugWindowsReservedSuffix: "-x",
			},
			want: "con-x",
		},
		{
			name:  "trailing dots and spaces removed",
			title: "Test...  ",
			cfg: ResolvedConfig{
				SlugNFKC:                 true,
				SlugLowercase:            true,
				SlugAllowChars:           "a-z0-9-",
				SlugMaxRunes:             60,
				SlugTrimTrailingDotSpace: true,
			},
			want: "test",
		},
		{
			name:  "fallback when empty",
			title: "@#$%",
			cfg: ResolvedConfig{
				SlugNFKC:       true,
				SlugLowercase:  true,
				SlugAllowChars: "a-z0-9-",
				SlugMaxRunes:   60,
				SlugFallback:   "default-slug",
			},
			want: "default-slug",
		},
		{
			name:  "max runes limit",
			title: "This is a very long title that should be truncated",
			cfg: ResolvedConfig{
				SlugNFKC:       true,
				SlugLowercase:  true,
				SlugAllowChars: "a-z0-9-",
				SlugMaxRunes:   20,
			},
			want: "this-is-a-very-long",
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