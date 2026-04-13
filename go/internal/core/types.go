// Package core defines the domain types shared across the graphify pipeline.
//
// These types intentionally avoid any I/O, storage, or transport concerns so
// that the same structures can be used by a local CLI driver today and by a
// hosted backend + remote client in the future.
package core

// Confidence classifies how certain a relation is.
//
// Extracted: explicitly stated in source (import, direct call).
// Inferred:  reasonable deduction (call-graph second pass, co-occurrence).
// Ambiguous: uncertain; surfaced for human review in the report.
type Confidence string

const (
	ConfExtracted Confidence = "EXTRACTED"
	ConfInferred  Confidence = "INFERRED"
	ConfAmbiguous Confidence = "AMBIGUOUS"
)

// FileType classifies the origin of a node for filtering and analysis.
type FileType string

const (
	FileCode      FileType = "code"
	FileDocument  FileType = "document"
	FilePaper     FileType = "paper"
	FileImage     FileType = "image"
	FileRationale FileType = "rationale"
)

// Node represents a single entity in the knowledge graph.
type Node struct {
	ID             string   `json:"id"`
	Label          string   `json:"label"`
	FileType       FileType `json:"file_type"`
	SourceFile     string   `json:"source_file"`
	SourceLocation string   `json:"source_location,omitempty"`
	// Community is populated after clustering; -1 means unassigned.
	Community int `json:"community,omitempty"`
}

// Edge represents a directed relationship between two nodes.
type Edge struct {
	Source         string     `json:"source"`
	Target         string     `json:"target"`
	Relation       string     `json:"relation"`
	Confidence     Confidence `json:"confidence"`
	SourceFile     string     `json:"source_file,omitempty"`
	SourceLocation string     `json:"source_location,omitempty"`
	Weight         float64    `json:"weight,omitempty"`
}

// Extraction is the output of a single file's extraction pass.
type Extraction struct {
	Nodes        []Node `json:"nodes"`
	Edges        []Edge `json:"edges"`
	InputTokens  int    `json:"input_tokens,omitempty"`
	OutputTokens int    `json:"output_tokens,omitempty"`
}

// Stats summarises the contents of a graph.
type Stats struct {
	Nodes       int            `json:"nodes"`
	Edges       int            `json:"edges"`
	Communities int            `json:"communities"`
	Confidence  map[string]int `json:"confidence"`
}
