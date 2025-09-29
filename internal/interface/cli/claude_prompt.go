package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ClaudeCodePromptBuilder builds prompts specifically for Claude Code
type ClaudeCodePromptBuilder struct {
	WorkDir string
	SBIDir  string
	SBIID   string
	Turn    int
	Step    string
}

// LoadExternalPrompt loads a prompt template from external file
func (b *ClaudeCodePromptBuilder) LoadExternalPrompt(status string, taskDescription string) (string, error) {
	// Determine prompt file name based on status
	var promptFile string
	switch status {
	case "READY", "WIP":
		promptFile = "WIP.md"
	case "REVIEW":
		promptFile = "REVIEW.md"
	case "REVIEW&WIP":
		promptFile = "REVIEW_AND_WIP.md"
	default:
		return "", fmt.Errorf("unknown status: %s", status)
	}

	// Try to load from .deespec/prompts/ directory
	promptPath := filepath.Join(".deespec", "prompts", promptFile)
	template, err := os.ReadFile(promptPath)
	if err != nil {
		// File doesn't exist or can't be read
		return "", err
	}

	// Replace placeholders in template
	prompt := string(template)
	prompt = strings.ReplaceAll(prompt, "{{.WorkDir}}", b.WorkDir)
	prompt = strings.ReplaceAll(prompt, "{{.SBIID}}", b.SBIID)
	prompt = strings.ReplaceAll(prompt, "{{.Turn}}", fmt.Sprintf("%d", b.Turn))
	prompt = strings.ReplaceAll(prompt, "{{.Step}}", b.Step)
	prompt = strings.ReplaceAll(prompt, "{{.SBIDir}}", b.SBIDir)
	prompt = strings.ReplaceAll(prompt, "{{.TaskDescription}}", taskDescription)

	// Add timestamp for context
	prompt = strings.ReplaceAll(prompt, "{{.Timestamp}}", fmt.Sprintf("%s", time.Now().Format("2006-01-02 15:04:05")))

	return prompt, nil
}

// BuildImplementPrompt creates an implementation prompt for Claude Code
func (b *ClaudeCodePromptBuilder) BuildImplementPrompt(taskDescription string) string {
	var sb strings.Builder

	sb.WriteString("# Implementation Task\n\n")
	sb.WriteString("## Context\n")
	sb.WriteString(fmt.Sprintf("- Working Directory: `%s`\n", b.WorkDir))
	sb.WriteString(fmt.Sprintf("- SBI ID: %s\n", b.SBIID))
	sb.WriteString(fmt.Sprintf("- Turn: %d\n", b.Turn))
	sb.WriteString(fmt.Sprintf("- Step: %s\n", b.Step))
	sb.WriteString(fmt.Sprintf("- Artifacts Directory: `%s`\n", b.SBIDir))
	sb.WriteString("\n")

	sb.WriteString("## Task Description\n")
	sb.WriteString(taskDescription)
	sb.WriteString("\n\n")

	sb.WriteString("## Instructions\n")
	sb.WriteString("1. Analyze the task requirements and existing code structure\n")
	sb.WriteString("2. Use Read/Grep/Glob tools to understand the codebase\n")
	sb.WriteString("3. Implement required changes using Edit/MultiEdit/Write tools\n")
	sb.WriteString("4. Follow existing code patterns and conventions\n")
	sb.WriteString("5. Ensure changes are atomic and don't break existing functionality\n")
	sb.WriteString("6. Add appropriate error handling and validation\n")
	sb.WriteString("\n")

	sb.WriteString("## Available Tools\n")
	sb.WriteString("You have access to all Claude Code tools for implementation.\n")
	sb.WriteString("\n")

	sb.WriteString("## Output Requirements\n")
	sb.WriteString("At the end of your implementation, provide:\n")
	sb.WriteString("1. **Summary of Changes**: List all files modified and what was changed\n")
	sb.WriteString("2. **Key Decisions**: Explain important implementation choices\n")
	sb.WriteString("3. **Testing Recommendations**: Suggest how to verify the changes\n")
	sb.WriteString("\n")
	sb.WriteString("## Implementation Note\n")
	sb.WriteString("End with a section '## Implementation Note' containing a 2-3 sentence summary.\n")

	return sb.String()
}

// BuildTestPrompt creates a test prompt for Claude Code
func (b *ClaudeCodePromptBuilder) BuildTestPrompt(previousArtifact string) string {
	var sb strings.Builder

	sb.WriteString("# Test Verification Task\n\n")
	sb.WriteString("## Context\n")
	sb.WriteString(fmt.Sprintf("- Working Directory: `%s`\n", b.WorkDir))
	sb.WriteString(fmt.Sprintf("- SBI ID: %s\n", b.SBIID))
	sb.WriteString(fmt.Sprintf("- Turn: %d\n", b.Turn))
	sb.WriteString(fmt.Sprintf("- Previous Step: implement\n"))
	sb.WriteString("\n")

	if previousArtifact != "" {
		sb.WriteString("## Previous Implementation\n")
		sb.WriteString(fmt.Sprintf("Implementation artifact location: `%s`\n", previousArtifact))
		sb.WriteString("First read this artifact to understand what was implemented.\n")
		sb.WriteString("\n")
	}

	sb.WriteString("## Instructions\n")
	sb.WriteString("1. Read the implementation artifact to understand changes\n")
	sb.WriteString("2. Identify the test strategy based on the project type\n")
	sb.WriteString("3. Run appropriate tests using Bash tool:\n")
	sb.WriteString("   - For Go: `go test ./...` or specific package tests\n")
	sb.WriteString("   - For Node: `npm test` or `yarn test`\n")
	sb.WriteString("   - For Python: `pytest` or `python -m unittest`\n")
	sb.WriteString("4. If no tests exist, create simple verification commands\n")
	sb.WriteString("5. Document any failures or warnings\n")
	sb.WriteString("\n")

	sb.WriteString("## Available Tools\n")
	sb.WriteString("You have access to all Claude Code tools for testing.\n")
	sb.WriteString("\n")

	sb.WriteString("## Test Output Format\n")
	sb.WriteString("Provide:\n")
	sb.WriteString("1. **Test Commands Run**: List all test commands executed\n")
	sb.WriteString("2. **Results Summary**: Pass/Fail status and counts\n")
	sb.WriteString("3. **Issues Found**: Any failures or warnings\n")
	sb.WriteString("4. **Coverage**: What was tested and what wasn't\n")
	sb.WriteString("\n")
	sb.WriteString("## Test Note\n")
	sb.WriteString("End with a '## Test Note' section summarizing the test results.\n")

	return sb.String()
}

// BuildReviewPrompt creates a review prompt for Claude Code
func (b *ClaudeCodePromptBuilder) BuildReviewPrompt(implementArtifact, testArtifact string) string {
	var sb strings.Builder

	sb.WriteString("# Code Review Task\n\n")
	sb.WriteString("## Context\n")
	sb.WriteString(fmt.Sprintf("- Working Directory: `%s`\n", b.WorkDir))
	sb.WriteString(fmt.Sprintf("- SBI ID: %s\n", b.SBIID))
	sb.WriteString(fmt.Sprintf("- Turn: %d\n", b.Turn))
	sb.WriteString(fmt.Sprintf("- Reviewing: implementation and test results\n"))
	sb.WriteString("\n")

	sb.WriteString("## Artifacts to Review\n")
	sb.WriteString("Read these artifacts to understand what was done:\n")
	if implementArtifact != "" {
		sb.WriteString(fmt.Sprintf("1. Implementation: `%s`\n", implementArtifact))
	}
	if testArtifact != "" {
		sb.WriteString(fmt.Sprintf("2. Test Results: `%s`\n", testArtifact))
	}
	sb.WriteString("\n")

	sb.WriteString("## Review Process\n")
	sb.WriteString("1. Read both artifacts carefully\n")
	sb.WriteString("2. Use Read/Grep tools to verify actual code changes\n")
	sb.WriteString("3. Check if implementation matches requirements\n")
	sb.WriteString("4. Verify test results are satisfactory\n")
	sb.WriteString("5. Look for potential issues or improvements\n")
	sb.WriteString("\n")

	sb.WriteString("## Available Tools\n")
	sb.WriteString("You have access to all Claude Code tools for review.\n")
	sb.WriteString("\n")

	sb.WriteString("## Review Criteria\n")
	sb.WriteString("Evaluate based on:\n")
	sb.WriteString("1. **Functionality**: Does it solve the intended problem?\n")
	sb.WriteString("2. **Code Quality**: Is it well-structured and maintainable?\n")
	sb.WriteString("3. **Testing**: Are tests comprehensive and passing?\n")
	sb.WriteString("4. **Standards**: Does it follow project conventions?\n")
	sb.WriteString("5. **Edge Cases**: Are error cases handled properly?\n")
	sb.WriteString("\n")

	sb.WriteString("## Decision Guidelines\n")
	sb.WriteString("Make your decision based on the review:\n")
	sb.WriteString("- `DECISION: SUCCEEDED` - Implementation is correct and tests pass\n")
	sb.WriteString("- `DECISION: NEEDS_CHANGES` - Issues found that need fixing\n")
	sb.WriteString("- `DECISION: FAILED` - Critical issues or unable to complete\n")
	sb.WriteString("\n")
	sb.WriteString("## Review Note\n")
	sb.WriteString("End with a '## Review Note' section explaining your decision with specific details.\n")

	return sb.String()
}

// BuildPlanPrompt creates a planning prompt for Claude Code
func (b *ClaudeCodePromptBuilder) BuildPlanPrompt(todo string) string {
	var sb strings.Builder

	sb.WriteString("# Planning Task\n\n")
	sb.WriteString("## Context\n")
	sb.WriteString(fmt.Sprintf("- Working Directory: `%s`\n", b.WorkDir))
	sb.WriteString(fmt.Sprintf("- SBI ID: %s\n", b.SBIID))
	sb.WriteString(fmt.Sprintf("- Starting Turn: %d\n", b.Turn))
	sb.WriteString("\n")

	sb.WriteString("## TODO\n")
	sb.WriteString(todo)
	sb.WriteString("\n\n")

	sb.WriteString("## Instructions\n")
	sb.WriteString("Create a detailed plan for implementing this task:\n")
	sb.WriteString("1. Analyze the requirements\n")
	sb.WriteString("2. Identify files that need to be modified\n")
	sb.WriteString("3. Break down the implementation into clear steps\n")
	sb.WriteString("4. Note any dependencies or prerequisites\n")
	sb.WriteString("5. Highlight potential challenges\n")
	sb.WriteString("\n")

	sb.WriteString("## Expected Output\n")
	sb.WriteString("Provide a structured plan with:\n")
	sb.WriteString("- Overview (200 characters max)\n")
	sb.WriteString("- Step-by-step implementation approach\n")
	sb.WriteString("- Files to be modified\n")
	sb.WriteString("- Testing strategy\n")

	return sb.String()
}

// GetLastArtifact returns the path to the last artifact for a given step
func (b *ClaudeCodePromptBuilder) GetLastArtifact(step string, turn int) string {
	if b.SBIDir == "" || step == "" || turn <= 0 {
		return ""
	}
	return filepath.Join(b.SBIDir, fmt.Sprintf("%s_%d.md", step, turn))
}
