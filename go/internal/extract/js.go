package extract

import (
	"strings"

	"github.com/safishamsi/graphify/go/internal/core"
)

// JSExtractor handles .js, .jsx, .ts, .tsx, .mjs, .cjs.
type JSExtractor struct{}

func (JSExtractor) Extensions() []string {
	return []string{".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs"}
}

var (
	jsImport  = compile(`(?m)^\s*import\s+(?:[\w*\s{},]+)\s+from\s+['"]([^'"]+)['"]`)
	jsRequire = compile(`require\(\s*['"]([^'"]+)['"]\s*\)`)
	jsExport  = compile(`(?m)^\s*export\s+(?:default\s+)?(?:(?:async\s+)?function|class|const|let)\s+([A-Za-z_]\w*)`)
	jsClass   = compile(`(?m)^\s*(?:export\s+)?class\s+([A-Za-z_]\w*)(?:\s+extends\s+([A-Za-z_][\w.]*))?`)
	jsFunc    = compile(`(?m)^\s*(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_]\w*)\s*\(`)
	jsArrow   = compile(`(?m)^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_]\w*)\s*=\s*(?:async\s*)?\(`)
	jsCall    = compile(`\b([A-Za-z_]\w*)\s*\(`)
)

func (JSExtractor) Extract(path string, src []byte) (core.Extraction, error) {
	ex := core.Extraction{}
	fid := fileID(path)
	file := fileNode(path, core.FileCode)
	ex.Nodes = append(ex.Nodes, file)
	text := string(src)

	addImport := func(mod string, off int) {
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
	for _, m := range jsImport.FindAllStringSubmatchIndex(text, -1) {
		addImport(text[m[2]:m[3]], m[0])
	}
	for _, m := range jsRequire.FindAllStringSubmatchIndex(text, -1) {
		addImport(text[m[2]:m[3]], m[0])
	}

	addDef := func(name string, off int, baseExtend string) {
		line := lineOf(src, off)
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
		if baseExtend != "" {
			tgt := makeID("ext", baseExtend)
			ex.Nodes = append(ex.Nodes, core.Node{
				ID: tgt, Label: baseExtend, FileType: core.FileCode, SourceFile: path,
			})
			ex.Edges = append(ex.Edges, core.Edge{
				Source: id, Target: tgt, Relation: "inherits",
				Confidence: core.ConfExtracted, SourceFile: path,
				SourceLocation: locStr(line), Weight: 1.0,
			})
		}
	}
	var defSpans []callerSpan
	for _, m := range jsClass.FindAllStringSubmatchIndex(text, -1) {
		name := text[m[2]:m[3]]
		base := ""
		if m[4] != -1 {
			base = text[m[4]:m[5]]
		}
		addDef(name, m[0], base)
		defSpans = append(defSpans, callerSpan{id: makeID(fid, name), start: m[0]})
	}
	for _, m := range jsFunc.FindAllStringSubmatchIndex(text, -1) {
		name := text[m[2]:m[3]]
		addDef(name, m[0], "")
		defSpans = append(defSpans, callerSpan{id: makeID(fid, name), start: m[0]})
	}
	for _, m := range jsArrow.FindAllStringSubmatchIndex(text, -1) {
		name := text[m[2]:m[3]]
		addDef(name, m[0], "")
		defSpans = append(defSpans, callerSpan{id: makeID(fid, name), start: m[0]})
	}
	for _, m := range jsExport.FindAllStringSubmatchIndex(text, -1) {
		name := text[m[2]:m[3]]
		addDef(name, m[0], "")
	}

	// Intra-file calls.
	defined := map[string]string{}
	for _, n := range ex.Nodes {
		defined[n.Label] = n.ID
	}
	sortByStart(defSpans)
	for i := range defSpans {
		defSpans[i].end = len(text)
		if i+1 < len(defSpans) {
			defSpans[i].end = defSpans[i+1].start
		}
	}
	for _, c := range defSpans {
		body := text[c.start:c.end]
		for _, cm := range jsCall.FindAllStringSubmatchIndex(body, -1) {
			callee := body[cm[2]:cm[3]]
			if isJSKeyword(callee) {
				continue
			}
			tgt, ok := defined[callee]
			if !ok || tgt == c.id {
				continue
			}
			ex.Edges = append(ex.Edges, core.Edge{
				Source: c.id, Target: tgt, Relation: "calls",
				Confidence: core.ConfInferred, SourceFile: path,
				SourceLocation: locStr(lineOf(src, c.start+cm[0])), Weight: 1.0,
			})
		}
	}

	ex.Nodes = dedupeNodes(ex.Nodes)
	return ex, nil
}

func sortByStart(cs []callerSpan) {
	// Insertion sort; span counts are small.
	for i := 1; i < len(cs); i++ {
		for j := i; j > 0 && cs[j-1].start > cs[j].start; j-- {
			cs[j-1], cs[j] = cs[j], cs[j-1]
		}
	}
}

var jsKeywordSet = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "catch": true,
	"return": true, "typeof": true, "new": true, "await": true, "function": true,
	"super": true, "throw": true, "yield": true, "void": true, "delete": true,
}

func isJSKeyword(s string) bool {
	return jsKeywordSet[strings.ToLower(s)]
}
