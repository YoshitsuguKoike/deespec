package service

import (
	"fmt"
	"regexp"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
)

// Validation constants
const (
	MaxIDLength    = 64
	MaxTitleLength = 200
	MaxLabelCount  = 32
)

// RegisterValidationService handles validation logic for SBI registration
type RegisterValidationService struct {
	// Configurable validation rules
	idPattern              *regexp.Regexp
	idMaxLen               int
	titleMaxLen            int
	titleDenyEmpty         bool
	labelsPattern          *regexp.Regexp
	labelsMaxCount         int
	labelsWarnOnDuplicates bool
}

// NewRegisterValidationService creates a new validation service with default rules
func NewRegisterValidationService() *RegisterValidationService {
	return &RegisterValidationService{
		idPattern:              regexp.MustCompile(`^[A-Z0-9-]{1,64}$`),
		idMaxLen:               MaxIDLength,
		titleMaxLen:            MaxTitleLength,
		titleDenyEmpty:         true,
		labelsPattern:          regexp.MustCompile(`^[a-z0-9-]+$`),
		labelsMaxCount:         MaxLabelCount,
		labelsWarnOnDuplicates: true,
	}
}

// ValidateSpec validates a RegisterSpec and returns validation result
func (s *RegisterValidationService) ValidateSpec(spec *dto.RegisterSpec) dto.ValidationResult {
	result := dto.ValidationResult{
		Warnings: []string{},
	}

	// Validate ID
	if err := s.ValidateID(spec.ID); err != nil {
		result.Err = err
		return result
	}

	// Validate Title
	if err := s.ValidateTitle(spec.Title); err != nil {
		result.Err = err
		return result
	}

	// Validate Labels
	labelWarnings, err := s.ValidateLabels(spec.Labels)
	if err != nil {
		result.Err = err
		return result
	}
	result.Warnings = append(result.Warnings, labelWarnings...)

	return result
}

// ValidateID validates the ID field
func (s *RegisterValidationService) ValidateID(id string) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}

	if s.idPattern != nil && !s.idPattern.MatchString(id) {
		return fmt.Errorf("invalid id format: must match %s", s.idPattern.String())
	}

	if s.idMaxLen > 0 && len(id) > s.idMaxLen {
		return fmt.Errorf("id length exceeds maximum of %d characters", s.idMaxLen)
	}

	return nil
}

// ValidateTitle validates the Title field
func (s *RegisterValidationService) ValidateTitle(title string) error {
	if title == "" && s.titleDenyEmpty {
		return fmt.Errorf("title is required and cannot be empty")
	}

	if s.titleMaxLen > 0 && len(title) > s.titleMaxLen {
		return fmt.Errorf("title length exceeds maximum of %d characters", s.titleMaxLen)
	}

	return nil
}

// ValidateLabels validates the Labels field
func (s *RegisterValidationService) ValidateLabels(labels []string) (warnings []string, err error) {
	// Labels are optional
	if labels == nil {
		return nil, nil
	}

	labelMap := make(map[string]bool)
	for _, label := range labels {
		if s.labelsPattern != nil && !s.labelsPattern.MatchString(label) {
			return nil, fmt.Errorf("invalid label format '%s': must match %s", label, s.labelsPattern.String())
		}

		// Check for duplicates
		if labelMap[label] && s.labelsWarnOnDuplicates {
			warnings = append(warnings, fmt.Sprintf("duplicate label: %s", label))
		}
		labelMap[label] = true
	}

	// Check count limit
	if s.labelsMaxCount > 0 && len(labels) > s.labelsMaxCount {
		warnings = append(warnings, fmt.Sprintf("labels count exceeds %d (%d)", s.labelsMaxCount, len(labels)))
	}

	return warnings, nil
}
