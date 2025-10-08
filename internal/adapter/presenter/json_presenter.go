package presenter

import (
	"encoding/json"
	"io"

	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
)

// JSONPresenter implements output.Presenter for JSON output
// Formats all output as JSON for programmatic consumption
type JSONPresenter struct {
	output io.Writer
}

// NewJSONPresenter creates a new JSON presenter
func NewJSONPresenter(output io.Writer) output.Presenter {
	return &JSONPresenter{output: output}
}

// PresentSuccess presents a successful result as JSON
func (p *JSONPresenter) PresentSuccess(message string, data interface{}) error {
	result := map[string]interface{}{
		"success": true,
		"message": message,
		"data":    data,
	}
	return json.NewEncoder(p.output).Encode(result)
}

// PresentError presents an error as JSON
func (p *JSONPresenter) PresentError(err error) error {
	result := map[string]interface{}{
		"success": false,
		"error":   err.Error(),
	}
	return json.NewEncoder(p.output).Encode(result)
}

// PresentProgress presents progress information as JSON
func (p *JSONPresenter) PresentProgress(message string, progress int, total int) error {
	result := map[string]interface{}{
		"type":     "progress",
		"message":  message,
		"progress": progress,
		"total":    total,
		"percent":  float64(progress) / float64(total) * 100,
	}
	return json.NewEncoder(p.output).Encode(result)
}
