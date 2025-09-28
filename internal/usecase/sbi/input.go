package sbi

// RegisterSBIInput represents the input data for registering a new SBI
type RegisterSBIInput struct {
	Title  string   // Required title of the specification
	Body   string   // Optional body content (raw text)
	Labels []string // Optional labels for categorization
}
