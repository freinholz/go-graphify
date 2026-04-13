package extract

import (
	"regexp"
	"strings"

	"github.com/safishamsi/graphify/go/internal/core"
)

// GenericExtractor handles JVM and C-family languages via a shared pattern
// config: class/function regexes + import regex + call regex.
type GenericExtractor struct {
	Name        string
	Exts        []string
	ClassRe     *regexp.Regexp // submatch 1 = name
	FuncRe      *regexp.Regexp // submatch 1 = name
	ImportRe    *regexp.Regexp // submatch 1 = module
	CallRe      *regexp.Regexp // submatch 1 = callee (intra-file)
	FileType    core.FileType
	IgnoreNames map[string]bool
}

func (g *GenericExtractor) Extensions() []string { return g.Exts }

func (g *GenericExtractor) Extract(path string, src []byte) (core.Extraction, error) {
	ex := core.Extraction{}
	ft := g.FileType
	if ft == "" {
		ft = core.FileCode
	}
	fid := fileID(path)
	file := fileNode(path, ft)
	ex.Nodes = append(ex.Nodes, file)
	text := string(src)

	if g.ImportRe != nil {
		for _, m := range g.ImportRe.FindAllStringSubmatchIndex(text, -1) {
			mod := strings.TrimSpace(text[m[2]:m[3]])
			if mod == "" {
				continue
			}
			tgt := makeID("ext", mod)
			ex.Nodes = append(ex.Nodes, core.Node{
				ID: tgt, Label: mod, FileType: ft,
				SourceFile: path, SourceLocation: locStr(lineOf(src, m[0])),
			})
			ex.Edges = append(ex.Edges, core.Edge{
				Source: file.ID, Target: tgt, Relation: "imports",
				Confidence: core.ConfExtracted, SourceFile: path,
				SourceLocation: locStr(lineOf(src, m[0])), Weight: 1.0,
			})
		}
	}
	var spans []callerSpan
	add := func(re *regexp.Regexp) {
		if re == nil {
			return
		}
		for _, m := range re.FindAllStringSubmatchIndex(text, -1) {
			name := text[m[2]:m[3]]
			if g.IgnoreNames[name] {
				continue
			}
			line := lineOf(src, m[0])
			id := makeID(fid, name)
			ex.Nodes = append(ex.Nodes, core.Node{
				ID: id, Label: name, FileType: ft,
				SourceFile: path, SourceLocation: locStr(line),
			})
			ex.Edges = append(ex.Edges, core.Edge{
				Source: file.ID, Target: id, Relation: "contains",
				Confidence: core.ConfExtracted, SourceFile: path,
				SourceLocation: locStr(line), Weight: 1.0,
			})
			spans = append(spans, callerSpan{id: id, start: m[0]})
		}
	}
	add(g.ClassRe)
	add(g.FuncRe)

	if g.CallRe != nil {
		defined := map[string]string{}
		for _, n := range ex.Nodes {
			defined[n.Label] = n.ID
		}
		sortByStart(spans)
		for i := range spans {
			spans[i].end = len(text)
			if i+1 < len(spans) {
				spans[i].end = spans[i+1].start
			}
		}
		for _, c := range spans {
			body := text[c.start:c.end]
			for _, cm := range g.CallRe.FindAllStringSubmatchIndex(body, -1) {
				callee := body[cm[2]:cm[3]]
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
	}

	ex.Nodes = dedupeNodes(ex.Nodes)
	return ex, nil
}
