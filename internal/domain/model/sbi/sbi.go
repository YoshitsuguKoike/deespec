package sbi

import (
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/task"
)

// SBI represents a Small Backlog Item (implementation unit)
// SBI can optionally belong to a PBI and is the smallest unit of work
// SBI is an aggregate root in DDD terms
type SBI struct {
	base      *task.BaseTask
	metadata  SBIMetadata
	execution *ExecutionState
}

// SBIMetadata contains SBI-specific metadata
type SBIMetadata struct {
	EstimatedHours float64
	Priority       int        // 0=通常, 1=高, 2=緊急
	Sequence       int        // 登録順序番号 (自動採番)
	RegisteredAt   time.Time  // 明示的な登録タイムスタンプ
	StartedAt      *time.Time // 作業開始時刻 (PENDING→PICKED時に記録)
	CompletedAt    *time.Time // 作業完了時刻 (DONE/FAILED時に記録)
	Labels         []string
	AssignedAgent  string   // e.g., "claude-code", "gemini-cli", "codex"
	FilePaths      []string // Files to be modified/created
	DependsOn      []string // IDs of SBIs that must be completed before this SBI
	OnlyImplement  bool     // false=実装→レビュー（デフォルト）, true=実装のみ
}

// ExecutionState tracks the execution state of an SBI
type ExecutionState struct {
	CurrentTurn    model.Turn
	CurrentAttempt model.Attempt
	MaxTurns       int
	MaxAttempts    int
	LastError      string
	ArtifactPaths  []string
}

// NewSBI creates a new SBI
func NewSBI(title, description string, parentPBIID *model.TaskID, metadata SBIMetadata) (*SBI, error) {
	baseTask, err := task.NewBaseTask(
		model.TaskTypeSBI,
		title,
		description,
		parentPBIID, // SBI can optionally have a PBI as parent
	)
	if err != nil {
		return nil, err
	}

	// Initialize execution state with default limits
	executionState := &ExecutionState{
		CurrentTurn:    model.NewTurn(),
		CurrentAttempt: model.NewAttempt(),
		MaxTurns:       10, // Default max turns
		MaxAttempts:    3,  // Default max attempts
		LastError:      "",
		ArtifactPaths:  []string{},
	}

	return &SBI{
		base:      baseTask,
		metadata:  metadata,
		execution: executionState,
	}, nil
}

// ReconstructSBI reconstructs an SBI from stored data
func ReconstructSBI(
	id model.TaskID,
	title string,
	description string,
	status model.Status,
	currentStep model.Step,
	parentPBIID *model.TaskID,
	metadata SBIMetadata,
	execution *ExecutionState,
	createdAt time.Time,
	updatedAt time.Time,
) *SBI {
	baseTask := task.ReconstructBaseTask(
		id,
		model.TaskTypeSBI,
		title,
		description,
		status,
		currentStep,
		parentPBIID,
		createdAt,
		updatedAt,
	)

	return &SBI{
		base:      baseTask,
		metadata:  metadata,
		execution: execution,
	}
}

// Implement Task interface
func (s *SBI) ID() model.TaskID {
	return s.base.ID()
}

func (s *SBI) Type() model.TaskType {
	return s.base.Type()
}

func (s *SBI) Title() string {
	return s.base.Title()
}

func (s *SBI) Description() string {
	return s.base.Description()
}

func (s *SBI) Status() model.Status {
	return s.base.Status()
}

func (s *SBI) CurrentStep() model.Step {
	return s.base.CurrentStep()
}

func (s *SBI) ParentTaskID() *model.TaskID {
	return s.base.ParentTaskID()
}

func (s *SBI) CreatedAt() model.Timestamp {
	return s.base.CreatedAt()
}

func (s *SBI) UpdatedAt() model.Timestamp {
	return s.base.UpdatedAt()
}

func (s *SBI) UpdateStatus(newStatus model.Status) error {
	return s.base.UpdateStatus(newStatus)
}

func (s *SBI) UpdateStep(newStep model.Step) error {
	return s.base.UpdateStep(newStep)
}

// SBI-specific methods

// Metadata returns the SBI metadata
func (s *SBI) Metadata() SBIMetadata {
	return s.metadata
}

// UpdateMetadata updates the SBI metadata
func (s *SBI) UpdateMetadata(metadata SBIMetadata) {
	s.metadata = metadata
}

// ExecutionState returns the execution state
func (s *SBI) ExecutionState() *ExecutionState {
	return s.execution
}

// IncrementTurn increments the turn counter
func (s *SBI) IncrementTurn() {
	s.execution.CurrentTurn = s.execution.CurrentTurn.Increment()
	s.execution.CurrentAttempt = model.NewAttempt() // Reset attempt counter
}

// IncrementAttempt increments the attempt counter
func (s *SBI) IncrementAttempt() {
	s.execution.CurrentAttempt = s.execution.CurrentAttempt.Increment()
}

// RecordError records an execution error
func (s *SBI) RecordError(errorMsg string) {
	s.execution.LastError = errorMsg
}

// ClearError clears the last error
func (s *SBI) ClearError() {
	s.execution.LastError = ""
}

// AddArtifact adds an artifact path to the execution state
func (s *SBI) AddArtifact(artifactPath string) {
	s.execution.ArtifactPaths = append(s.execution.ArtifactPaths, artifactPath)
}

// HasExceededMaxTurns checks if max turns have been exceeded
func (s *SBI) HasExceededMaxTurns() bool {
	return s.execution.CurrentTurn.Value() > s.execution.MaxTurns
}

// HasExceededMaxAttempts checks if max attempts have been exceeded
func (s *SBI) HasExceededMaxAttempts() bool {
	return s.execution.CurrentAttempt.Value() > s.execution.MaxAttempts
}

// UpdateTitle updates the SBI title
func (s *SBI) UpdateTitle(title string) error {
	return s.base.UpdateTitle(title)
}

// UpdateDescription updates the SBI description
func (s *SBI) UpdateDescription(description string) {
	s.base.UpdateDescription(description)
}

// HasParentPBI checks if this SBI belongs to a PBI
func (s *SBI) HasParentPBI() bool {
	return s.base.ParentTaskID() != nil
}

// CanDelete checks if the SBI can be deleted
func (s *SBI) CanDelete() bool {
	// SBI can be deleted if it's not currently being executed
	return s.base.Status() != model.StatusImplementing
}

// IsCompleted checks if the SBI is completed
func (s *SBI) IsCompleted() bool {
	return s.base.Status() == model.StatusDone
}

// IsFailed checks if the SBI has failed
func (s *SBI) IsFailed() bool {
	return s.base.Status() == model.StatusFailed
}

// SetMaxTurns sets the maximum number of turns
func (s *SBI) SetMaxTurns(maxTurns int) {
	s.execution.MaxTurns = maxTurns
}

// SetMaxAttempts sets the maximum number of attempts
func (s *SBI) SetMaxAttempts(maxAttempts int) {
	s.execution.MaxAttempts = maxAttempts
}

// SetSequence sets the sequence number (registration order)
func (s *SBI) SetSequence(sequence int) {
	s.metadata.Sequence = sequence
}

// Sequence returns the sequence number
func (s *SBI) Sequence() int {
	return s.metadata.Sequence
}

// SetRegisteredAt sets the registration timestamp
func (s *SBI) SetRegisteredAt(registeredAt time.Time) {
	s.metadata.RegisteredAt = registeredAt
}

// RegisteredAt returns the registration timestamp
func (s *SBI) RegisteredAt() time.Time {
	return s.metadata.RegisteredAt
}

// Priority returns the priority level
func (s *SBI) Priority() int {
	return s.metadata.Priority
}

// SetPriority sets the priority level
func (s *SBI) SetPriority(priority int) {
	s.metadata.Priority = priority
}

// DependsOn returns the list of SBI IDs this SBI depends on
func (s *SBI) DependsOn() []string {
	return s.metadata.DependsOn
}

// SetDependsOn sets the list of SBI IDs this SBI depends on
func (s *SBI) SetDependsOn(dependsOn []string) {
	s.metadata.DependsOn = dependsOn
}

// AddDependency adds a single dependency to this SBI
func (s *SBI) AddDependency(sbiID string) {
	s.metadata.DependsOn = append(s.metadata.DependsOn, sbiID)
}

// HasDependencies checks if this SBI has any dependencies
func (s *SBI) HasDependencies() bool {
	return len(s.metadata.DependsOn) > 0
}

// MarkAsStarted records the work start time
func (s *SBI) MarkAsStarted() {
	now := time.Now()
	s.metadata.StartedAt = &now
}

// MarkAsCompleted records the work completion time
func (s *SBI) MarkAsCompleted() {
	now := time.Now()
	s.metadata.CompletedAt = &now
}

// StartedAt returns the work start time
func (s *SBI) StartedAt() *time.Time {
	return s.metadata.StartedAt
}

// CompletedAt returns the work completion time
func (s *SBI) CompletedAt() *time.Time {
	return s.metadata.CompletedAt
}

// WorkDuration calculates the duration between start and completion
// Returns nil if either timestamp is missing
func (s *SBI) WorkDuration() *time.Duration {
	if s.metadata.StartedAt == nil || s.metadata.CompletedAt == nil {
		return nil
	}
	duration := s.metadata.CompletedAt.Sub(*s.metadata.StartedAt)
	return &duration
}

// === Workflow Control Methods ===

// OnlyImplement checks if this SBI should only do implementation (no review)
func (s *SBI) OnlyImplement() bool {
	return s.metadata.OnlyImplement
}

// SetOnlyImplement sets the only_implement flag
func (s *SBI) SetOnlyImplement(onlyImplement bool) {
	s.metadata.OnlyImplement = onlyImplement
}
