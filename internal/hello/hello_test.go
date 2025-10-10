package hello

import "testing"

func TestHelloWorld(t *testing.T) {
	got := HelloWorld()
	want := "Hello, World!"

	if got != want {
		t.Errorf("HelloWorld() = %q, want %q", got, want)
	}
}

func TestHelloWithName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "with valid name",
			input:    "Alice",
			expected: "Hello, Alice!",
		},
		{
			name:     "with another name",
			input:    "Bob",
			expected: "Hello, Bob!",
		},
		{
			name:     "with empty string",
			input:    "",
			expected: "Hello, World!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HelloWithName(tt.input)
			if got != tt.expected {
				t.Errorf("HelloWithName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
