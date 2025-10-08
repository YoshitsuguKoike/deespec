package output

// Presenter defines the interface for presenting output to users
// Different implementations can format output for CLI, JSON, or other formats
type Presenter interface {
	// PresentSuccess presents a successful result
	PresentSuccess(message string, data interface{}) error

	// PresentError presents an error
	PresentError(err error) error

	// PresentProgress presents progress information
	PresentProgress(message string, progress int, total int) error
}

// ExecutionPresenter specifically handles execution result presentation
type ExecutionPresenter interface {
	Presenter

	// PresentExecutionResult presents the result of a task execution
	PresentExecutionResult(result ExecutionResult) error
}

// ExecutionResult represents the result of a task execution
type ExecutionResult struct {
	TaskID      string
	TaskType    string
	Step        string
	Status      string
	Message     string
	Artifacts   []string
	Duration    string
	ErrorDetail string
}
