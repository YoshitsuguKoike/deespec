package label

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/label"
)

// isOutsideProject checks if a file path is outside the current project
func isOutsideProject(filePath string) (bool, error) {
	// Get current working directory (project root)
	projectRoot, err := os.Getwd()
	if err != nil {
		return false, fmt.Errorf("failed to get working directory: %w", err)
	}

	// Convert both paths to absolute
	absProjectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return false, fmt.Errorf("failed to resolve project root: %w", err)
	}

	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to resolve file path: %w", err)
	}

	// Check if file is under project root
	relPath, err := filepath.Rel(absProjectRoot, absFilePath)
	if err != nil {
		return false, fmt.Errorf("failed to calculate relative path: %w", err)
	}

	// If relative path starts with "..", it's outside the project
	return strings.HasPrefix(relPath, ".."), nil
}

// promptCopyExternal prompts user to copy external file (default: Y)
func promptCopyExternal() (bool, error) {
	fmt.Print("プロジェクト外のパスが渡されました。ファイルをコピーしますか？ [Y/n]: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read input: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))

	// Default is Y (empty input or 'y')
	if response == "" || response == "y" {
		return true, nil
	}

	return false, nil
}

// copyExternalFile copies an external file to .deespec/labels/ preserving directory structure
func copyExternalFile(srcPath string) (string, error) {
	// Get project root
	projectRoot, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	// Convert to absolute paths
	absSrcPath, err := filepath.Abs(srcPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve source path: %w", err)
	}

	absProjectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", fmt.Errorf("failed to resolve project root: %w", err)
	}

	// Find common ancestor and calculate relative path
	relativePath, err := calculateRelativePathFromCommonAncestor(absSrcPath, absProjectRoot)
	if err != nil {
		return "", fmt.Errorf("failed to calculate relative path: %w", err)
	}

	// Destination: .deespec/labels/<relative-path>
	destPath := filepath.Join(projectRoot, ".deespec", "labels", relativePath)

	// Create destination directory
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Copy file
	if err := copyFile(absSrcPath, destPath); err != nil {
		return "", fmt.Errorf("failed to copy file: %w", err)
	}

	// Return absolute path (not relative) for compatibility with resolveTemplatePath
	absDestPath, err := filepath.Abs(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to calculate absolute destination path: %w", err)
	}

	return absDestPath, nil
}

// calculateRelativePathFromCommonAncestor calculates a relative path from common ancestor
func calculateRelativePathFromCommonAncestor(targetPath, projectRoot string) (string, error) {
	// Split paths into components
	targetParts := splitPath(targetPath)
	projectParts := splitPath(projectRoot)

	// Find common ancestor depth
	commonDepth := 0
	minLen := len(targetParts)
	if len(projectParts) < minLen {
		minLen = len(projectParts)
	}

	for i := 0; i < minLen; i++ {
		if targetParts[i] == projectParts[i] {
			commonDepth = i + 1
		} else {
			break
		}
	}

	// Get relative path from common ancestor
	if commonDepth == 0 {
		// No common ancestor, use full path (sanitized)
		return sanitizePath(targetPath), nil
	}

	// Join remaining parts
	relativeParts := targetParts[commonDepth:]
	if len(relativeParts) == 0 {
		return filepath.Base(targetPath), nil
	}

	return filepath.Join(relativeParts...), nil
}

// splitPath splits a file path into directory components
func splitPath(path string) []string {
	var parts []string
	path = filepath.Clean(path)

	for {
		dir, file := filepath.Split(path)
		if file != "" {
			parts = append([]string{file}, parts...)
		}
		if dir == "" || dir == "/" || dir == path {
			if dir == "/" {
				parts = append([]string{"/"}, parts...)
			}
			break
		}
		path = filepath.Clean(dir)
	}

	return parts
}

// sanitizePath creates a safe path by removing problematic characters
func sanitizePath(path string) string {
	// Remove leading slashes and drive letters
	path = strings.TrimPrefix(path, "/")
	if len(path) > 2 && path[1] == ':' {
		path = path[2:]
		path = strings.TrimPrefix(path, "\\")
		path = strings.TrimPrefix(path, "/")
	}

	// Replace path separators with underscores
	path = strings.ReplaceAll(path, string(filepath.Separator), "_")
	path = strings.ReplaceAll(path, "/", "_")
	path = strings.ReplaceAll(path, "\\", "_")

	return path
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	// Sync to ensure data is written to disk
	if err := dstFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync destination file: %w", err)
	}

	return nil
}

// CleanupStatus represents the result of file cleanup
type CleanupStatus int

const (
	CleanupSkipped    CleanupStatus = iota // Not an internal copied file
	CleanupStillInUse                      // File is still used by other labels
	CleanupDeleted                         // File was successfully deleted
	CleanupNotFound                        // File was already deleted
	CleanupError                           // Error during cleanup
)

// CleanupResult represents the result of a file cleanup operation
type CleanupResult struct {
	Status CleanupStatus
	Error  error
}

// deleteInternalCopiedFiles deletes files in .deespec/labels/ that are no longer referenced
func deleteInternalCopiedFiles(templatePath string, labelRepo interface {
	FindAll(ctx context.Context) ([]*label.Label, error)
}) CleanupResult {
	// Only delete files in .deespec/labels/
	if !strings.Contains(templatePath, ".deespec/labels/") {
		return CleanupResult{Status: CleanupSkipped}
	}

	// Get project root
	projectRoot, err := os.Getwd()
	if err != nil {
		return CleanupResult{Status: CleanupError, Error: fmt.Errorf("failed to get working directory: %w", err)}
	}

	// Convert to absolute path
	absTemplatePath, err := filepath.Abs(templatePath)
	if err != nil {
		return CleanupResult{Status: CleanupError, Error: fmt.Errorf("failed to resolve template path: %w", err)}
	}

	// Check if any other label is using this file
	allLabels, err := labelRepo.FindAll(context.Background())
	if err != nil {
		return CleanupResult{Status: CleanupError, Error: fmt.Errorf("failed to list all labels: %w", err)}
	}

	for _, lbl := range allLabels {
		for _, path := range lbl.TemplatePaths() {
			absPath, err := filepath.Abs(path)
			if err != nil {
				continue
			}
			if absPath == absTemplatePath {
				// File is still being used by another label
				return CleanupResult{Status: CleanupStillInUse}
			}
		}
	}

	// Delete the file
	fileNotFound := false
	if err := os.Remove(absTemplatePath); err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, which is fine
			fileNotFound = true
		} else {
			return CleanupResult{Status: CleanupError, Error: fmt.Errorf("failed to delete file: %w", err)}
		}
	}

	// Clean up empty parent directories up to .deespec/labels/
	labelsDir := filepath.Join(projectRoot, ".deespec", "labels")
	cleanupEmptyDirs(filepath.Dir(absTemplatePath), labelsDir)

	if fileNotFound {
		return CleanupResult{Status: CleanupNotFound}
	}
	return CleanupResult{Status: CleanupDeleted}
}

// cleanupEmptyDirs removes empty directories up to the base directory
func cleanupEmptyDirs(dirPath, baseDir string) {
	// Don't delete the base directory itself
	if dirPath == baseDir {
		return
	}

	// Check if directory is empty
	entries, err := os.ReadDir(dirPath)
	if err != nil || len(entries) > 0 {
		return
	}

	// Remove empty directory
	os.Remove(dirPath)

	// Recursively clean up parent
	cleanupEmptyDirs(filepath.Dir(dirPath), baseDir)
}

// displayTemplatePreview displays the first N lines of a file and returns total line count
func displayTemplatePreview(filePath string, previewLines int) (totalLines int, err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	fmt.Printf("\n--- Preview (first %d lines) ---\n", previewLines)

	// Display preview
	for scanner.Scan() && lineNum < previewLines {
		fmt.Println(scanner.Text())
		lineNum++
	}

	// Count remaining lines
	for scanner.Scan() {
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return lineNum, fmt.Errorf("error reading file: %w", err)
	}

	return lineNum, nil
}

// promptViewFullContent prompts user to view full content or exit
func promptViewFullContent() bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nPress 'f' then Enter to view full content, or just Enter to exit: ")

	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response := strings.TrimSpace(strings.ToLower(input))
	return response == "f"
}

// viewFileWithPager opens a file with less pager
func viewFileWithPager(filePath string) error {
	// Check if less is available
	lessBin, err := exec.LookPath("less")
	if err != nil {
		// Fallback to more if less is not available
		moreBin, err := exec.LookPath("more")
		if err != nil {
			return fmt.Errorf("neither 'less' nor 'more' command found")
		}
		lessBin = moreBin
	}

	cmd := exec.Command(lessBin, "-N", filePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
