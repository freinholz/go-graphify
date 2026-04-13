// Package validate enforces the extraction schema before a graph is built.
package validate

import (
	"fmt"

	"github.com/safishamsi/graphify/go/internal/core"
)

// ValidFileTypes enumerates acceptable file_type values.
var ValidFileTypes = map[core.FileType]bool{
	core.FileCode: true, core.FileDocument: true, core.FilePaper: true,
	core.FileImage: true, core.FileRationale: true,
}

// ValidConfidences enumerates acceptable confidence values.
var ValidConfidences = map[core.Confidence]bool{
	core.ConfExtracted: true, core.ConfInferred: true, core.ConfAmbiguous: true,
}

// Extraction validates a single extraction's structural correctness and returns
// an accumulated list of errors (empty if valid). Dangling edges are not
// treated as errors since external imports are expected.
func Extraction(ex *core.Extraction) []string {
	var errs []string
	seen := map[string]bool{}
	for i, n := range ex.Nodes {
		if n.ID == "" {
			errs = append(errs, fmt.Sprintf("node[%d]: missing id", i))
		}
		if n.Label == "" {
			errs = append(errs, fmt.Sprintf("node[%d]: missing label", i))
		}
		if n.FileType != "" && !ValidFileTypes[n.FileType] {
			errs = append(errs, fmt.Sprintf("node[%d]: invalid file_type %q", i, n.FileType))
		}
		if seen[n.ID] {
			errs = append(errs, fmt.Sprintf("node[%d]: duplicate id %q", i, n.ID))
		}
		seen[n.ID] = true
	}
	for i, e := range ex.Edges {
		if e.Source == "" || e.Target == "" {
			errs = append(errs, fmt.Sprintf("edge[%d]: missing endpoints", i))
		}
		if e.Relation == "" {
			errs = append(errs, fmt.Sprintf("edge[%d]: missing relation", i))
		}
		if !ValidConfidences[e.Confidence] {
			errs = append(errs, fmt.Sprintf("edge[%d]: invalid confidence %q", i, e.Confidence))
		}
	}
	return errs
}
