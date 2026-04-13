package extract

import (
	"strings"

	"github.com/safishamsi/graphify/go/internal/core"
)

// GoExtractor handles .go files.
type GoExtractor struct{}

func (GoExtractor) Extensions() []string { return []string{".go"} }

var (
	goImportSingle = compile(`(?m)^import\s+"([^"]+)"`)
	goImportBlock  = compile(`(?ms)^import\s*\(([^)]*)\)`)
	goFunc         = compile(`(?m)^func\s+(?:\(\s*\w+\s+[\w\*]+\s*\)\s+)?([A-Za-z_]\w*)\s*\(`)
	goType         = compile(`(?m)^type\s+([A-Za-z_]\w*)\s+(struct|interface|\w+)`)
	goCall         = compile(`\b([A-Za-z_]\w*)\s*\(`)
)

func (GoExtractor) Extract(path string, src []byte) (core.Extraction, error) {
	ex := core.Extraction{}
	fid := fileID(path)
	file := fileNode(path, core.FileCode)
	ex.Nodes = append(ex.Nodes, file)

	text := string(src)

	// Imports.
	addImport := func(mod string, off int) {
		mod = strings.Trim(strings.TrimSpace(mod), `"`)
		if mod == "" {
			return
		}
		tgt := makeID("ext", mod)
		ex.Nodes = append(ex.Nodes, core.Node{
			ID: tgt, Label: mod, FileType: core.FileCode,
			SourceFile: path, SourceLocation: locStr(lineOf(src, off)),
		})
		ex.Edges = append(ex.Edges, core.Edge{
			Source: file.ID, Target: tgt, Relation: "imports",
			Confidence: core.ConfExtracted, SourceFile: path,
			SourceLocation: locStr(lineOf(src, off)), Weight: 1.0,
		})
	}
	for _, m := range goImportSingle.FindAllStringSubmatchIndex(text, -1) {
		addImport(text[m[2]:m[3]], m[0])
	}
	for _, m := range goImportBlock.FindAllStringSubmatchIndex(text, -1) {
		for _, line := range strings.Split(text[m[2]:m[3]], "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if idx := strings.Index(line, `"`); idx >= 0 {
				rest := line[idx+1:]
				if end := strings.Index(rest, `"`); end > 0 {
					addImport(rest[:end], m[0])
				}
			}
		}
	}

	// Types.
	for _, m := range goType.FindAllStringSubmatchIndex(text, -1) {
		name := text[m[2]:m[3]]
		line := lineOf(src, m[0])
		id := makeID(fid, name)
		ex.Nodes = append(ex.Nodes, core.Node{
			ID: id, Label: name, FileType: core.FileCode,
			SourceFile: path, SourceLocation: locStr(line),
		})
		ex.Edges = append(ex.Edges, core.Edge{
			Source: file.ID, Target: id, Relation: "contains",
			Confidence: core.ConfExtracted, SourceFile: path,
			SourceLocation: locStr(line), Weight: 1.0,
		})
	}

	// Functions.
	funcStarts := goFunc.FindAllStringSubmatchIndex(text, -1)
	for _, m := range funcStarts {
		name := text[m[2]:m[3]]
		line := lineOf(src, m[0])
		id := makeID(fid, name)
		ex.Nodes = append(ex.Nodes, core.Node{
			ID: id, Label: name, FileType: core.FileCode,
			SourceFile: path, SourceLocation: locStr(line),
		})
		ex.Edges = append(ex.Edges, core.Edge{
			Source: file.ID, Target: id, Relation: "contains",
			Confidence: core.ConfExtracted, SourceFile: path,
			SourceLocation: locStr(line), Weight: 1.0,
		})
	}

	// Calls (INFERRED).
	defined := map[string]string{}
	for _, n := range ex.Nodes {
		defined[n.Label] = n.ID
	}
	for i, m := range funcStarts {
		name := text[m[2]:m[3]]
		srcID := makeID(fid, name)
		end := len(text)
		if i+1 < len(funcStarts) {
			end = funcStarts[i+1][0]
		}
		body := text[m[0]:end]
		for _, cm := range goCall.FindAllStringSubmatchIndex(body, -1) {
			callee := body[cm[2]:cm[3]]
			if callee == name {
				continue
			}
			tgt, ok := defined[callee]
			if !ok {
				continue
			}
			ex.Edges = append(ex.Edges, core.Edge{
				Source: srcID, Target: tgt, Relation: "calls",
				Confidence: core.ConfInferred, SourceFile: path,
				SourceLocation: locStr(lineOf(src, m[0]+cm[0])), Weight: 1.0,
			})
		}
	}

	ex.Nodes = dedupeNodes(ex.Nodes)
	return ex, nil
}
