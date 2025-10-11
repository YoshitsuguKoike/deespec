package pbi

import (
	"fmt"
	"regexp"
	"strconv"
)

// GenerateID generates the next PBI ID based on existing PBIs
// Format: PBI-001, PBI-002, etc.
func GenerateID(repo Repository) (string, error) {
	pbis, err := repo.FindAll()
	if err != nil {
		return "", fmt.Errorf("failed to find all PBIs: %w", err)
	}

	maxNum := 0
	re := regexp.MustCompile(`PBI-(\d+)`)

	for _, p := range pbis {
		matches := re.FindStringSubmatch(p.ID)
		if len(matches) == 2 {
			num, err := strconv.Atoi(matches[1])
			if err != nil {
				continue
			}
			if num > maxNum {
				maxNum = num
			}
		}
	}

	nextNum := maxNum + 1
	return fmt.Sprintf("PBI-%03d", nextNum), nil
}
