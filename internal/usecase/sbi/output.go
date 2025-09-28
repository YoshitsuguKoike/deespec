package sbi

// RegisterSBIOutput represents the output data after registering an SBI
type RegisterSBIOutput struct {
	ID       string // Generated SBI-<ULID> format ID
	SpecPath string // Path where the spec.md file was saved
}