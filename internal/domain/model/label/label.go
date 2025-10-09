package label

import (
	"time"
)

// Label represents a label entity in the domain
// Labels are used to categorize and organize tasks (SBI, PBI, EPIC)
type Label struct {
	id            int
	name          string
	description   string
	templatePaths []string          // Relative paths to template files
	contentHashes map[string]string // File path -> SHA256 hash
	parentLabelID *int              // For hierarchical labels
	color         string
	priority      int // Higher value = higher priority when merging instructions
	isActive      bool
	lineCount     int       // Total line count for all templates
	lastSyncedAt  time.Time // Last time templates were synced
	metadata      string    // JSON metadata for future extensions
	createdAt     time.Time
	updatedAt     time.Time
}

// NewLabel creates a new Label
func NewLabel(name, description string, templatePaths []string, priority int) *Label {
	now := time.Now()
	return &Label{
		id:            0, // Will be set by repository
		name:          name,
		description:   description,
		templatePaths: templatePaths,
		contentHashes: make(map[string]string),
		parentLabelID: nil,
		color:         "",
		priority:      priority,
		isActive:      true,
		lineCount:     0,
		lastSyncedAt:  now,
		metadata:      "",
		createdAt:     now,
		updatedAt:     now,
	}
}

// ReconstructLabel reconstructs a Label from stored data
func ReconstructLabel(
	id int,
	name string,
	description string,
	templatePaths []string,
	contentHashes map[string]string,
	parentLabelID *int,
	color string,
	priority int,
	isActive bool,
	lineCount int,
	lastSyncedAt time.Time,
	metadata string,
	createdAt time.Time,
	updatedAt time.Time,
) *Label {
	if contentHashes == nil {
		contentHashes = make(map[string]string)
	}
	return &Label{
		id:            id,
		name:          name,
		description:   description,
		templatePaths: templatePaths,
		contentHashes: contentHashes,
		parentLabelID: parentLabelID,
		color:         color,
		priority:      priority,
		isActive:      isActive,
		lineCount:     lineCount,
		lastSyncedAt:  lastSyncedAt,
		metadata:      metadata,
		createdAt:     createdAt,
		updatedAt:     updatedAt,
	}
}

// Getters
func (l *Label) ID() int                          { return l.id }
func (l *Label) Name() string                     { return l.name }
func (l *Label) Description() string              { return l.description }
func (l *Label) TemplatePaths() []string          { return l.templatePaths }
func (l *Label) ContentHashes() map[string]string { return l.contentHashes }
func (l *Label) ParentLabelID() *int              { return l.parentLabelID }
func (l *Label) Color() string                    { return l.color }
func (l *Label) Priority() int                    { return l.priority }
func (l *Label) IsActive() bool                   { return l.isActive }
func (l *Label) LineCount() int                   { return l.lineCount }
func (l *Label) LastSyncedAt() time.Time          { return l.lastSyncedAt }
func (l *Label) Metadata() string                 { return l.metadata }
func (l *Label) CreatedAt() time.Time             { return l.createdAt }
func (l *Label) UpdatedAt() time.Time             { return l.updatedAt }

// Setters with business logic

// SetID sets the label ID (called by repository after persistence)
func (l *Label) SetID(id int) {
	l.id = id
}

// SetContentHash sets the hash for a specific template file
func (l *Label) SetContentHash(path string, hash string) {
	l.contentHashes[path] = hash
	l.updatedAt = time.Now()
}

// SetLineCount sets the total line count for all templates
func (l *Label) SetLineCount(count int) {
	l.lineCount = count
	l.updatedAt = time.Now()
}

// UpdateSyncTime updates the last synchronized time
func (l *Label) UpdateSyncTime() {
	l.lastSyncedAt = time.Now()
	l.updatedAt = time.Now()
}

// SetDescription updates the label description
func (l *Label) SetDescription(description string) {
	l.description = description
	l.updatedAt = time.Now()
}

// SetColor sets the UI display color
func (l *Label) SetColor(color string) {
	l.color = color
	l.updatedAt = time.Now()
}

// SetPriority sets the merge priority
func (l *Label) SetPriority(priority int) {
	l.priority = priority
	l.updatedAt = time.Now()
}

// Activate activates the label
func (l *Label) Activate() {
	l.isActive = true
	l.updatedAt = time.Now()
}

// Deactivate deactivates the label
func (l *Label) Deactivate() {
	l.isActive = false
	l.updatedAt = time.Now()
}

// SetParentLabelID sets the parent label ID for hierarchical structure
func (l *Label) SetParentLabelID(parentID *int) {
	l.parentLabelID = parentID
	l.updatedAt = time.Now()
}

// AddTemplatePath adds a new template path
func (l *Label) AddTemplatePath(path string) {
	for _, p := range l.templatePaths {
		if p == path {
			return // Already exists
		}
	}
	l.templatePaths = append(l.templatePaths, path)
	l.updatedAt = time.Now()
}

// RemoveTemplatePath removes a template path
func (l *Label) RemoveTemplatePath(path string) {
	newPaths := make([]string, 0, len(l.templatePaths))
	for _, p := range l.templatePaths {
		if p != path {
			newPaths = append(newPaths, p)
		}
	}
	l.templatePaths = newPaths
	delete(l.contentHashes, path)
	l.updatedAt = time.Now()
}

// GetContentHash returns the hash for a specific path
func (l *Label) GetContentHash(path string) (string, bool) {
	hash, exists := l.contentHashes[path]
	return hash, exists
}

// ClearContentHashes clears all content hashes
func (l *Label) ClearContentHashes() {
	l.contentHashes = make(map[string]string)
	l.updatedAt = time.Now()
}
