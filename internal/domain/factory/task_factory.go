package factory

import (
	"errors"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/epic"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/pbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/task"
)

// Factory creates task instances
type Factory struct{}

// NewFactory creates a new task factory
func NewFactory() *Factory {
	return &Factory{}
}

// CreateEPIC creates a new EPIC task
func (f *Factory) CreateEPIC(title, description string, metadata epic.EPICMetadata) (*epic.EPIC, error) {
	if title == "" {
		return nil, errors.New("EPIC title cannot be empty")
	}

	return epic.NewEPIC(title, description, metadata)
}

// CreatePBI creates a new PBI task
func (f *Factory) CreatePBI(title, description string, parentEPICID *model.TaskID, metadata pbi.PBIMetadata) (*pbi.PBI, error) {
	if title == "" {
		return nil, errors.New("PBI title cannot be empty")
	}

	return pbi.NewPBI(title, description, parentEPICID, metadata)
}

// CreateSBI creates a new SBI task
func (f *Factory) CreateSBI(title, description string, parentPBIID *model.TaskID, metadata sbi.SBIMetadata) (*sbi.SBI, error) {
	if title == "" {
		return nil, errors.New("SBI title cannot be empty")
	}

	return sbi.NewSBI(title, description, parentPBIID, metadata)
}

// CreateTaskFromType creates a task based on type (polymorphic creation)
func (f *Factory) CreateTaskFromType(
	taskType model.TaskType,
	title, description string,
	parentID *model.TaskID,
) (task.Task, error) {
	switch taskType {
	case model.TaskTypeEPIC:
		// EPIC cannot have a parent
		if parentID != nil {
			return nil, errors.New("EPIC cannot have a parent task")
		}
		return epic.NewEPIC(title, description, epic.EPICMetadata{})

	case model.TaskTypePBI:
		return pbi.NewPBI(title, description, parentID, pbi.PBIMetadata{})

	case model.TaskTypeSBI:
		return sbi.NewSBI(title, description, parentID, sbi.SBIMetadata{})

	default:
		return nil, &InvalidTaskTypeError{TaskType: taskType}
	}
}

// InvalidTaskTypeError is returned when an invalid task type is provided
type InvalidTaskTypeError struct {
	TaskType model.TaskType
}

func (e *InvalidTaskTypeError) Error() string {
	return "invalid task type: " + e.TaskType.String()
}

// ValidateTaskHierarchy validates parent-child relationships
func (f *Factory) ValidateTaskHierarchy(childType model.TaskType, parentType model.TaskType) error {
	validRelationships := map[model.TaskType][]model.TaskType{
		model.TaskTypeEPIC: {},                   // EPIC has no parent
		model.TaskTypePBI:  {model.TaskTypeEPIC}, // PBI can have EPIC parent (or no parent)
		model.TaskTypeSBI:  {model.TaskTypePBI},  // SBI can have PBI parent (or no parent)
	}

	allowedParents, exists := validRelationships[childType]
	if !exists {
		return &InvalidTaskTypeError{TaskType: childType}
	}

	// No parent is allowed for PBI and SBI
	if parentType == "" {
		return nil
	}

	// Check if parent type is valid
	for _, allowed := range allowedParents {
		if allowed == parentType {
			return nil
		}
	}

	return &InvalidHierarchyError{
		ChildType:  childType,
		ParentType: parentType,
	}
}

// InvalidHierarchyError is returned when task hierarchy is invalid
type InvalidHierarchyError struct {
	ChildType  model.TaskType
	ParentType model.TaskType
}

func (e *InvalidHierarchyError) Error() string {
	return "invalid hierarchy: " + e.ChildType.String() + " cannot be child of " + e.ParentType.String()
}
