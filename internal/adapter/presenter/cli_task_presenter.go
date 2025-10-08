package presenter

import (
	"fmt"
	"io"
	"strings"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
)

// CLITaskPresenter implements output.Presenter for CLI output
// Formats output in a human-readable text format similar to existing CLI
type CLITaskPresenter struct {
	output io.Writer
}

// NewCLITaskPresenter creates a new CLI task presenter
func NewCLITaskPresenter(output io.Writer) output.Presenter {
	return &CLITaskPresenter{output: output}
}

// PresentSuccess presents a successful result
func (p *CLITaskPresenter) PresentSuccess(message string, data interface{}) error {
	fmt.Fprintf(p.output, "✓ %s\n\n", message)

	switch v := data.(type) {
	case *dto.SBIDTO:
		return p.presentSBI(v)
	case *dto.PBIDTO:
		return p.presentPBI(v)
	case *dto.EPICDTO:
		return p.presentEPIC(v)
	case *dto.ListTasksResponse:
		return p.presentTaskList(v)
	case *dto.ImplementTaskResponse:
		return p.presentImplementResult(v)
	case *dto.ReviewTaskResponse:
		return p.presentReviewResult(v)
	default:
		// Fallback for unknown types
		fmt.Fprintf(p.output, "%+v\n", data)
	}

	return nil
}

// PresentError presents an error
func (p *CLITaskPresenter) PresentError(err error) error {
	fmt.Fprintf(p.output, "✗ Error: %v\n", err)
	return err
}

// PresentProgress presents progress information
func (p *CLITaskPresenter) PresentProgress(message string, progress int, total int) error {
	percentage := float64(progress) / float64(total) * 100
	bar := strings.Repeat("█", progress) + strings.Repeat("░", total-progress)
	fmt.Fprintf(p.output, "\r%s [%s] %.1f%%", message, bar, percentage)
	return nil
}

// presentSBI presents SBI details (similar to existing CLI format)
func (p *CLITaskPresenter) presentSBI(sbi *dto.SBIDTO) error {
	fmt.Fprintf(p.output, "SBI: %s\n", sbi.Title)
	fmt.Fprintf(p.output, "ID: %s\n", sbi.ID)
	fmt.Fprintf(p.output, "Status: %s\n", sbi.Status)
	fmt.Fprintf(p.output, "Step: %s\n", sbi.CurrentStep)

	if sbi.ParentID != nil {
		fmt.Fprintf(p.output, "Parent PBI: %s\n", *sbi.ParentID)
	}

	fmt.Fprintf(p.output, "Turn: %d/%d\n", sbi.CurrentTurn, sbi.MaxTurns)
	fmt.Fprintf(p.output, "Attempt: %d/%d\n", sbi.CurrentAttempt, sbi.MaxAttempts)

	if sbi.EstimatedHours > 0 {
		fmt.Fprintf(p.output, "Estimated Hours: %.1f\n", sbi.EstimatedHours)
	}

	if sbi.Priority > 0 {
		fmt.Fprintf(p.output, "Priority: %d\n", sbi.Priority)
	}

	if len(sbi.Labels) > 0 {
		fmt.Fprintf(p.output, "Labels: %s\n", strings.Join(sbi.Labels, ", "))
	}

	if sbi.AssignedAgent != "" {
		fmt.Fprintf(p.output, "Assigned Agent: %s\n", sbi.AssignedAgent)
	}

	if sbi.Description != "" {
		fmt.Fprintf(p.output, "\nDescription:\n%s\n", sbi.Description)
	}

	if len(sbi.FilePaths) > 0 {
		fmt.Fprintf(p.output, "\nFile Paths:\n")
		for _, path := range sbi.FilePaths {
			fmt.Fprintf(p.output, "  - %s\n", path)
		}
	}

	if len(sbi.ArtifactPaths) > 0 {
		fmt.Fprintf(p.output, "\nArtifacts:\n")
		for _, artifact := range sbi.ArtifactPaths {
			fmt.Fprintf(p.output, "  - %s\n", artifact)
		}
	}

	if sbi.LastError != "" {
		fmt.Fprintf(p.output, "\nLast Error: %s\n", sbi.LastError)
	}

	return nil
}

// presentPBI presents PBI details
func (p *CLITaskPresenter) presentPBI(pbi *dto.PBIDTO) error {
	fmt.Fprintf(p.output, "PBI: %s\n", pbi.Title)
	fmt.Fprintf(p.output, "ID: %s\n", pbi.ID)
	fmt.Fprintf(p.output, "Status: %s\n", pbi.Status)
	fmt.Fprintf(p.output, "Step: %s\n", pbi.CurrentStep)

	if pbi.ParentID != nil {
		fmt.Fprintf(p.output, "Parent EPIC: %s\n", *pbi.ParentID)
	}

	if pbi.StoryPoints > 0 {
		fmt.Fprintf(p.output, "Story Points: %d\n", pbi.StoryPoints)
	}

	if pbi.Priority > 0 {
		fmt.Fprintf(p.output, "Priority: %d\n", pbi.Priority)
	}

	if len(pbi.Labels) > 0 {
		fmt.Fprintf(p.output, "Labels: %s\n", strings.Join(pbi.Labels, ", "))
	}

	if pbi.AssignedAgent != "" {
		fmt.Fprintf(p.output, "Assigned Agent: %s\n", pbi.AssignedAgent)
	}

	fmt.Fprintf(p.output, "SBIs: %d\n", pbi.SBICount)

	if pbi.Description != "" {
		fmt.Fprintf(p.output, "\nDescription:\n%s\n", pbi.Description)
	}

	if len(pbi.AcceptanceCriteria) > 0 {
		fmt.Fprintf(p.output, "\nAcceptance Criteria:\n")
		for i, criteria := range pbi.AcceptanceCriteria {
			fmt.Fprintf(p.output, "  %d. %s\n", i+1, criteria)
		}
	}

	if len(pbi.SBIIDs) > 0 {
		fmt.Fprintf(p.output, "\nChild SBIs:\n")
		for _, sbiID := range pbi.SBIIDs {
			fmt.Fprintf(p.output, "  - %s\n", sbiID)
		}
	}

	return nil
}

// presentEPIC presents EPIC details
func (p *CLITaskPresenter) presentEPIC(epic *dto.EPICDTO) error {
	fmt.Fprintf(p.output, "EPIC: %s\n", epic.Title)
	fmt.Fprintf(p.output, "ID: %s\n", epic.ID)
	fmt.Fprintf(p.output, "Status: %s\n", epic.Status)
	fmt.Fprintf(p.output, "Step: %s\n", epic.CurrentStep)

	if epic.EstimatedStoryPoints > 0 {
		fmt.Fprintf(p.output, "Estimated Story Points: %d\n", epic.EstimatedStoryPoints)
	}

	if epic.Priority > 0 {
		fmt.Fprintf(p.output, "Priority: %d\n", epic.Priority)
	}

	if len(epic.Labels) > 0 {
		fmt.Fprintf(p.output, "Labels: %s\n", strings.Join(epic.Labels, ", "))
	}

	if epic.AssignedAgent != "" {
		fmt.Fprintf(p.output, "Assigned Agent: %s\n", epic.AssignedAgent)
	}

	fmt.Fprintf(p.output, "PBIs: %d\n", epic.PBICount)

	if epic.Description != "" {
		fmt.Fprintf(p.output, "\nDescription:\n%s\n", epic.Description)
	}

	if len(epic.PBIIDs) > 0 {
		fmt.Fprintf(p.output, "\nChild PBIs:\n")
		for _, pbiID := range epic.PBIIDs {
			fmt.Fprintf(p.output, "  - %s\n", pbiID)
		}
	}

	return nil
}

// presentTaskList presents a list of tasks
func (p *CLITaskPresenter) presentTaskList(list *dto.ListTasksResponse) error {
	fmt.Fprintf(p.output, "Total: %d tasks", list.TotalCount)

	if list.Limit > 0 {
		fmt.Fprintf(p.output, " (showing %d-%d)", list.Offset+1, list.Offset+len(list.Tasks))
	}
	fmt.Fprintf(p.output, "\n\n")

	for i, task := range list.Tasks {
		status := task.Status
		if task.CurrentStep != "" && task.CurrentStep != task.Status {
			status = fmt.Sprintf("%s/%s", task.Status, task.CurrentStep)
		}

		fmt.Fprintf(p.output, "%d. [%s] %s (%s)\n", i+1, task.Type, task.Title, status)
		fmt.Fprintf(p.output, "   ID: %s\n", task.ID)

		if task.ParentID != nil {
			fmt.Fprintf(p.output, "   Parent: %s\n", *task.ParentID)
		}
	}

	return nil
}

// presentImplementResult presents implementation result
func (p *CLITaskPresenter) presentImplementResult(result *dto.ImplementTaskResponse) error {
	if result.Success {
		fmt.Fprintf(p.output, "Implementation successful!\n")
	} else {
		fmt.Fprintf(p.output, "Implementation failed: %s\n", result.Message)
	}

	fmt.Fprintf(p.output, "Task ID: %s\n", result.TaskID)
	fmt.Fprintf(p.output, "Next Step: %s\n", result.NextStep)

	if len(result.Artifacts) > 0 {
		fmt.Fprintf(p.output, "\nArtifacts generated:\n")
		for _, artifact := range result.Artifacts {
			fmt.Fprintf(p.output, "  - %s\n", artifact)
		}
	}

	if len(result.ChildTaskIDs) > 0 {
		fmt.Fprintf(p.output, "\nChild tasks created:\n")
		for _, childID := range result.ChildTaskIDs {
			fmt.Fprintf(p.output, "  - %s\n", childID)
		}
	}

	return nil
}

// presentReviewResult presents review result
func (p *CLITaskPresenter) presentReviewResult(result *dto.ReviewTaskResponse) error {
	if result.Success {
		fmt.Fprintf(p.output, "Review completed!\n")
	} else {
		fmt.Fprintf(p.output, "Review failed\n")
	}

	fmt.Fprintf(p.output, "Task ID: %s\n", result.TaskID)
	fmt.Fprintf(p.output, "Message: %s\n", result.Message)
	fmt.Fprintf(p.output, "Next Step: %s\n", result.NextStep)

	return nil
}
