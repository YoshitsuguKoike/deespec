package runner

import (
	"regexp"
	"testing"
)

func TestParseDecision(t *testing.T) {
	// Default regex pattern
	defaultRegex := regexp.MustCompile(`^DECISION:\s+(OK|NEEDS_CHANGES)\s*$`)

	// Custom regex pattern
	customRegex := regexp.MustCompile(`^REVIEW_RESULT:\s*(OK|NEEDS_CHANGES)\s*$`)

	tests := []struct {
		name     string
		output   string
		regex    *regexp.Regexp
		want     DecisionType
	}{
		{
			name: "OK decision at end",
			output: `Some review output here
Analysis complete
DECISION: OK`,
			regex: defaultRegex,
			want:  DecisionOK,
		},
		{
			name: "NEEDS_CHANGES decision at end",
			output: `Review findings:
- Issue 1
- Issue 2
DECISION: NEEDS_CHANGES`,
			regex: defaultRegex,
			want:  DecisionNeedsChanges,
		},
		{
			name: "decision in middle, last one wins",
			output: `DECISION: OK
Some more text
DECISION: NEEDS_CHANGES`,
			regex: defaultRegex,
			want:  DecisionNeedsChanges,
		},
		{
			name: "no match returns pending",
			output: `Review complete
No decision line here`,
			regex: defaultRegex,
			want:  DecisionPending,
		},
		{
			name: "custom regex pattern OK",
			output: `Custom review format
REVIEW_RESULT: OK`,
			regex: customRegex,
			want:  DecisionOK,
		},
		{
			name: "custom regex pattern NEEDS_CHANGES",
			output: `Review output
REVIEW_RESULT: NEEDS_CHANGES`,
			regex: customRegex,
			want:  DecisionNeedsChanges,
		},
		{
			name: "whitespace handling",
			output: `Review done
DECISION:    OK   `,
			regex: defaultRegex,
			want:  DecisionOK,
		},
		{
			name: "nil regex returns pending",
			output: `DECISION: OK`,
			regex: nil,
			want:  DecisionPending,
		},
		{
			name: "empty output",
			output: ``,
			regex: defaultRegex,
			want:  DecisionPending,
		},
		{
			name: "multiline with decision at end",
			output: `First line
Second line
Third line
Fourth line
DECISION: OK`,
			regex: defaultRegex,
			want:  DecisionOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseDecision(tt.output, tt.regex)
			if got != tt.want {
				t.Errorf("ParseDecision() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractDecisionLine(t *testing.T) {
	defaultRegex := regexp.MustCompile(`^DECISION:\s+(OK|NEEDS_CHANGES)\s*$`)

	tests := []struct {
		name      string
		output    string
		regex     *regexp.Regexp
		wantLine  string
		wantFound bool
	}{
		{
			name: "extract OK line",
			output: `Review complete
DECISION: OK`,
			regex:     defaultRegex,
			wantLine:  "DECISION: OK",
			wantFound: true,
		},
		{
			name: "extract NEEDS_CHANGES line",
			output: `Issues found
DECISION: NEEDS_CHANGES`,
			regex:     defaultRegex,
			wantLine:  "DECISION: NEEDS_CHANGES",
			wantFound: true,
		},
		{
			name:      "no match",
			output:    `No decision here`,
			regex:     defaultRegex,
			wantLine:  "",
			wantFound: false,
		},
		{
			name:      "nil regex",
			output:    `DECISION: OK`,
			regex:     nil,
			wantLine:  "",
			wantFound: false,
		},
		{
			name: "extract last matching line",
			output: `DECISION: OK
Some text
DECISION: NEEDS_CHANGES`,
			regex:     defaultRegex,
			wantLine:  "DECISION: NEEDS_CHANGES",
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLine, gotFound := ExtractDecisionLine(tt.output, tt.regex)
			if gotLine != tt.wantLine {
				t.Errorf("ExtractDecisionLine() line = %v, want %v", gotLine, tt.wantLine)
			}
			if gotFound != tt.wantFound {
				t.Errorf("ExtractDecisionLine() found = %v, want %v", gotFound, tt.wantFound)
			}
		})
	}
}