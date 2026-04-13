package extract

import (
	"strings"

	"github.com/safishamsi/graphify/go/internal/core"
)

// PythonExtractor handles .py files using regex scanning. The output schema is
// compatible with the Python graphify tree-sitter extractor.
type PythonExtractor struct{}

func (PythonExtractor) Extensions() []string { return []string{".py"} }

var (
	pyClass  = compile(`(?m)^class\s+([A-Za-z_]\w*)\s*(?:\(([^)]*)\))?\s*:`)
	pyFunc   = compile(`(?m)^(?P<indent>\s*)(?:async\s+)?def\s+([A-Za-z_]\w*)\s*\(`)
	pyImport = compile(`(?m)^\s*import\s+([\w.]+(?:\s*,\s*[\w.]+)*)`)
	pyFrom   = compile(`(?m)^\s*from\s+([.\w]+)\s+import\s+([\w,\s*]+)`)
	pyCall   = compile(`(?:\b([A-Za-z_]\w*)\s*\()`)
)

func (PythonExtractor) Extract(path string, src []byte) (core.Extraction, error) {
	ex := core.Extraction{}
	fid := fileID(path)
	file := fileNode(path, core.FileCode)
	ex.Nodes = append(ex.Nodes, file)

	text := string(src)

	// Imports (file-level).
	for _, m := range pyImport.FindAllStringSubmatchIndex(text, -1) {
		mod := text[m[2]:m[3]]
		for _, name := range splitCommas(mod) {
			tgt := makeID("ext", name)
			ex.Nodes = append(ex.Nodes, core.Node{
				ID: tgt, Label: name, FileType: core.FileCode,
				SourceFile: path, SourceLocation: locStr(lineOf(src, m[0])),
			})
			ex.Edges = append(ex.Edges, core.Edge{
				Source: file.ID, Target: tgt, Relation: "imports",
				Confidence: core.ConfExtracted, SourceFile: path,
				SourceLocation: locStr(lineOf(src, m[0])), Weight: 1.0,
			})
		}
	}
	for _, m := range pyFrom.FindAllStringSubmatchIndex(text, -1) {
		mod := text[m[2]:m[3]]
		for _, name := range splitCommas(text[m[4]:m[5]]) {
			full := mod + "." + strings.TrimSpace(name)
			tgt := makeID("ext", full)
			ex.Nodes = append(ex.Nodes, core.Node{
				ID: tgt, Label: full, FileType: core.FileCode,
				SourceFile: path, SourceLocation: locStr(lineOf(src, m[0])),
			})
			ex.Edges = append(ex.Edges, core.Edge{
				Source: file.ID, Target: tgt, Relation: "imports",
				Confidence: core.ConfExtracted, SourceFile: path,
				SourceLocation: locStr(lineOf(src, m[0])), Weight: 1.0,
			})
		}
	}

	// Classes.
	for _, m := range pyClass.FindAllStringSubmatchIndex(text, -1) {
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
		if m[4] != -1 {
			for _, base := range splitCommas(text[m[4]:m[5]]) {
				tgt := makeID("ext", base)
				ex.Nodes = append(ex.Nodes, core.Node{
					ID: tgt, Label: base, FileType: core.FileCode, SourceFile: path,
				})
				ex.Edges = append(ex.Edges, core.Edge{
					Source: id, Target: tgt, Relation: "inherits",
					Confidence: core.ConfExtracted, SourceFile: path,
					SourceLocation: locStr(line), Weight: 1.0,
				})
			}
		}
	}

	// Functions / methods. Indent tells us whether it's a method of the
	// enclosing class: for v1 we treat all as contained by the file.
	for _, m := range pyFunc.FindAllStringSubmatchIndex(text, -1) {
		name := text[m[4]:m[5]]
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

	// Calls — INFERRED second pass: any bare identifier followed by "(" that
	// matches a symbol we defined in this file becomes a calls edge.
	defined := map[string]string{}
	for _, n := range ex.Nodes {
		defined[n.Label] = n.ID
	}
	callers := collectCallerSpans(text, fid)
	for _, c := range callers {
		for _, m := range pyCall.FindAllStringSubmatchIndex(text[c.start:c.end], -1) {
			callee := text[c.start+m[2] : c.start+m[3]]
			tgt, ok := defined[callee]
			if !ok || tgt == c.id {
				continue
			}
			ex.Edges = append(ex.Edges, core.Edge{
				Source: c.id, Target: tgt, Relation: "calls",
				Confidence: core.ConfInferred, SourceFile: path,
				SourceLocation: locStr(lineOf(src, c.start+m[0])), Weight: 1.0,
			})
		}
	}

	ex.Nodes = dedupeNodes(ex.Nodes)
	return ex, nil
}

type callerSpan struct {
	id         string
	start, end int
}

// collectCallerSpans returns byte-ranges of function bodies (heuristic: body
// runs from def-line to the next def). v1 is coarse; good enough to establish
// intra-file calls.
func collectCallerSpans(text, fid string) []callerSpan {
	var out []callerSpan
	starts := pyFunc.FindAllStringSubmatchIndex(text, -1)
	for i, m := range starts {
		end := len(text)
		if i+1 < len(starts) {
			end = starts[i+1][0]
		}
		name := text[m[4]:m[5]]
		out = append(out, callerSpan{id: makeID(fid, name), start: m[0], end: end})
	}
	return out
}

func splitCommas(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == "*" {
			continue
		}
		out = append(out, p)
	}
	return out
}
