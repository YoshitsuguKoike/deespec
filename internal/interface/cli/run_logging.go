package cli

// replaceLoggingInRun demonstrates the pattern for replacing fmt.Fprintf with logger calls
func replaceLoggingInRun() {
	// Example 1: INFO level
	// Before: Info("another instance active")
	// After:
	Info("another instance active")

	// Example 2: ERROR level
	// Before: Error("failed to read state: %v\n", err)
	// After:
	// Error("failed to read state: %v", err)

	// Example 3: WARN level
	// Before: Warn("Failed to auto-register FB drafts: %v\n", err)
	// After:
	// Warn("Failed to auto-register FB drafts: %v", err)

	// Example 4: DEBUG level
	// Before: Debug("state.json and journal saved atomically via TX")
	// After:
	Debug("state.json and journal saved atomically via TX")

	// Example 5: Plain messages (usually INFO)
	// Before: fmt.Fprintf(os.Stderr, "Warning: failed to write journal: %v\n", err)
	// After:
	// Warn("failed to write journal: %v", err)
}
