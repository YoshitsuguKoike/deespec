package domain

type State struct {
	Version       int               `json:"version"`
	Current       string            `json:"current"`
	Turn          int               `json:"turn"`
	Inputs        map[string]string `json:"inputs"`
	LastArtifacts map[string]string `json:"last_artifacts"`
	Meta          struct {
		UpdatedAt string `json:"updated_at"`
	} `json:"meta"`
	WIP string `json:"wip"` // Work In Progress - current SBI ID
}

// 次ステップ（最小直進）
func NextStep(cur string, reviewDecision string) string {
	switch cur {
	case "plan":
		return "implement"
	case "implement":
		return "test"
	case "test":
		return "review"
	case "review":
		if reviewDecision == "OK" {
			return "done"
		}
		return "implement" // ブーメラン
	case "done":
		return "done"
	default:
		return "plan"
	}
}
