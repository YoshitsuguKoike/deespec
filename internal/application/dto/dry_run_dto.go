package dto

// DryRunInput represents input for dry-run execution
type DryRunInput struct {
	UseStdin      bool
	FilePath      string
	OnCollision   string
	OutputFormat  string // json or yaml
	CompactOutput bool
}

// DryRunReport represents the complete dry-run output
type DryRunReport struct {
	Meta           DryRunMeta       `json:"meta" yaml:"meta"`
	Input          DryRunInputInfo  `json:"input" yaml:"input"`
	Validation     DryRunValidation `json:"validation" yaml:"validation"`
	Resolution     DryRunResolution `json:"resolution" yaml:"resolution"`
	JournalPreview DryRunJournal    `json:"journal_preview" yaml:"journal_preview"`
}

// DryRunMeta contains metadata about the dry-run execution
type DryRunMeta struct {
	SchemaVersion   int      `json:"schema_version" yaml:"schema_version"`
	TsUTC           string   `json:"ts_utc" yaml:"ts_utc"`
	Version         string   `json:"version" yaml:"version"`
	PolicyFileFound bool     `json:"policy_file_found" yaml:"policy_file_found"`
	PolicyPath      string   `json:"policy_path" yaml:"policy_path"`
	PolicySHA256    string   `json:"policy_sha256,omitempty" yaml:"policy_sha256,omitempty"`
	SourcePriority  []string `json:"source_priority" yaml:"source_priority"`
	DryRun          bool     `json:"dry_run" yaml:"dry_run"`
}

// DryRunInputInfo describes the input source
type DryRunInputInfo struct {
	Source string `json:"source" yaml:"source"`
	Bytes  int    `json:"bytes" yaml:"bytes"`
}

// DryRunValidation contains validation results
type DryRunValidation struct {
	OK       bool     `json:"ok" yaml:"ok"`
	Errors   []string `json:"errors" yaml:"errors"`
	Warnings []string `json:"warnings" yaml:"warnings"`
}

// DryRunResolution shows path resolution details
type DryRunResolution struct {
	ID                   string `json:"id" yaml:"id"`
	Title                string `json:"title" yaml:"title"`
	Slug                 string `json:"slug" yaml:"slug"`
	BaseDir              string `json:"base_dir" yaml:"base_dir"`
	SpecPath             string `json:"spec_path" yaml:"spec_path"`
	CollisionMode        string `json:"collision_mode" yaml:"collision_mode"`
	CollisionWouldHappen bool   `json:"collision_would_happen" yaml:"collision_would_happen"`
	CollisionResolution  string `json:"collision_resolution" yaml:"collision_resolution"`
	FinalPath            string `json:"final_path" yaml:"final_path"`
	PathSafe             bool   `json:"path_safe" yaml:"path_safe"`
	SymlinkSafe          bool   `json:"symlink_safe" yaml:"symlink_safe"`
	ContainedInBase      bool   `json:"contained_in_base" yaml:"contained_in_base"`
}

// DryRunJournal represents the journal entry that would be written
type DryRunJournal struct {
	Ts        string                   `json:"ts" yaml:"ts"`
	Turn      int                      `json:"turn" yaml:"turn"`
	Step      string                   `json:"step" yaml:"step"`
	Decision  string                   `json:"decision" yaml:"decision"`
	ElapsedMs int64                    `json:"elapsed_ms" yaml:"elapsed_ms"`
	Error     string                   `json:"error" yaml:"error"`
	Artifacts []map[string]interface{} `json:"artifacts" yaml:"artifacts"`
}
