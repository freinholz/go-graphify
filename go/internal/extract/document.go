package extract

import (
	"regexp"
	"strings"

	"github.com/safishamsi/graphify/go/internal/core"
)

var (
	mdHeading  = regexp.MustCompile(`(?m)^(#+)\s+(.+)$`)
	mdWikilink = regexp.MustCompile(`\[\[([^\]|]+)(?:\|[^\]]+)?\]\]`)
	mdMdLink   = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
)

// extractDocument handles .md, .txt, .rst: the file itself becomes a document
// node, each heading becomes a contained section node, and wiki/markdown
// links become `references` edges to other labels.
func extractDocument(path string, src []byte) (core.Extraction, error) {
	ex := core.Extraction{}
	fid := fileID(path)
	file := fileNode(path, core.FileDocument)
	ex.Nodes = append(ex.Nodes, file)
	text := string(src)

	for _, m := range mdHeading.FindAllStringSubmatchIndex(text, -1) {
		name := strings.TrimSpace(text[m[4]:m[5]])
		if name == "" {
			continue
		}
		line := lineOf(src, m[0])
		id := makeID(fid, name)
		ex.Nodes = append(ex.Nodes, core.Node{
			ID: id, Label: name, FileType: core.FileDocument,
			SourceFile: path, SourceLocation: locStr(line),
		})
		ex.Edges = append(ex.Edges, core.Edge{
			Source: file.ID, Target: id, Relation: "contains",
			Confidence: core.ConfExtracted, SourceFile: path,
			SourceLocation: locStr(line), Weight: 1.0,
		})
	}
	for _, m := range mdWikilink.FindAllStringSubmatchIndex(text, -1) {
		target := strings.TrimSpace(text[m[2]:m[3]])
		if target == "" {
			continue
		}
		tgt := makeID("ref", target)
		ex.Nodes = append(ex.Nodes, core.Node{
			ID: tgt, Label: target, FileType: core.FileDocument, SourceFile: path,
		})
		ex.Edges = append(ex.Edges, core.Edge{
			Source: file.ID, Target: tgt, Relation: "references",
			Confidence: core.ConfExtracted, SourceFile: path,
			SourceLocation: locStr(lineOf(src, m[0])), Weight: 1.0,
		})
	}
	for _, m := range mdMdLink.FindAllStringSubmatchIndex(text, -1) {
		label := strings.TrimSpace(text[m[2]:m[3]])
		dest := strings.TrimSpace(text[m[4]:m[5]])
		if strings.HasPrefix(dest, "http://") || strings.HasPrefix(dest, "https://") {
			continue
		}
		tgt := makeID("ref", label)
		ex.Nodes = append(ex.Nodes, core.Node{
			ID: tgt, Label: label, FileType: core.FileDocument, SourceFile: path,
		})
		ex.Edges = append(ex.Edges, core.Edge{
			Source: file.ID, Target: tgt, Relation: "references",
			Confidence: core.ConfExtracted, SourceFile: path,
			SourceLocation: locStr(lineOf(src, m[0])), Weight: 1.0,
		})
	}
	ex.Nodes = dedupeNodes(ex.Nodes)
	return ex, nil
}
