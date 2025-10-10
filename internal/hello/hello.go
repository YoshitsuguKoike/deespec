// Package hello provides a simple greeting functionality for testing Claude Code CLI integration.
package hello

import "fmt"

// HelloWorld returns a friendly greeting message.
// This function demonstrates basic Go functionality for integration testing.
func HelloWorld() string {
	return "Hello, World!"
}

// HelloWithName returns a personalized greeting message with the given name.
// If the name is empty, it returns a generic greeting.
func HelloWithName(name string) string {
	if name == "" {
		return HelloWorld()
	}
	return fmt.Sprintf("Hello, %s!", name)
}
