package dto

// PromptContextDTO contains context information for building prompts
type PromptContextDTO struct {
	WorkDir         string
	SBIDir          string
	SBIID           string
	Turn            int
	Step            string
	TaskDescription string
}

// PromptResultDTO contains the built prompt and any warnings
type PromptResultDTO struct {
	Content  string
	Warnings []string
}

// ArtifactLocationDTO specifies artifact locations for review prompts
type ArtifactLocationDTO struct {
	ImplementArtifact string
	TestArtifact      string
}
