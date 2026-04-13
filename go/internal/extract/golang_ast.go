package extract

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"github.com/safishamsi/graphify/go/internal/core"
)

// GoASTExtractor uses Go's standard library parser (go/parser + go/ast) for
// precise, structural extraction of .go files.
//
// Advantages over the regex path (used for every other language):
//   - Correctly distinguishes function calls from keywords (if/for/switch/…)
//     and statement-level uses.
//   - Honors real block scoping, so a call inside a nested function is
//     attributed to the nested function, not to the outer one.
//   - Ignores content inside string literals and comments.
//   - Handles generics, methods on pointer/star receivers, embedded types,
//     interface method declarations.
//
// Zero external dependencies — go/parser ships with the Go toolchain, so the
// binary remains static and cgo-free.
type GoASTExtractor struct{}

func (GoASTExtractor) Extensions() []string { return []string{".go"} }

func (GoASTExtractor) Extract(path string, src []byte) (core.Extraction, error) {
	fset := token.NewFileSet()
	// SkipObjectResolution keeps us fast; we do our own name resolution below.
	f, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
	if err != nil && f == nil {
		// Total parse failure (extremely rare: syntactically garbage input).
		// Return an empty extraction rather than inventing structure.
		return core.Extraction{}, nil
	}

	fid := fileID(path)
	file := fileNode(path, core.FileCode)
	ex := core.Extraction{Nodes: []core.Node{file}}

	// defined maps every lookup key -> node id. For methods we register both
	// "Recv.Method" and "Method" so intra-file calls via either a direct
	// identifier (helper()) or a selector (x.Method()) resolve.
	defined := map[string]string{}

	addNode := func(label string, line int) string {
		id := makeID(fid, label)
		ex.Nodes = append(ex.Nodes, core.Node{
			ID: id, Label: label, FileType: core.FileCode,
			SourceFile: path, SourceLocation: locStr(line),
		})
		ex.Edges = append(ex.Edges, core.Edge{
			Source: file.ID, Target: id, Relation: "contains",
			Confidence: core.ConfExtracted, SourceFile: path,
			SourceLocation: locStr(line), Weight: 1,
		})
		defined[label] = id
		return id
	}

	type funcEntry struct {
		id   string
		body *ast.BlockStmt
	}
	var funcs []funcEntry

	for _, decl := range f.Decls {
		switch d := decl.(type) {

		case *ast.GenDecl:
			switch d.Tok {
			case token.IMPORT:
				for _, spec := range d.Specs {
					is := spec.(*ast.ImportSpec)
					imp := strings.Trim(is.Path.Value, `"`)
					line := fset.Position(is.Pos()).Line
					tgt := makeID("ext", imp)
					ex.Nodes = append(ex.Nodes, core.Node{
						ID: tgt, Label: imp, FileType: core.FileCode,
						SourceFile: path, SourceLocation: locStr(line),
					})
					ex.Edges = append(ex.Edges, core.Edge{
						Source: file.ID, Target: tgt, Relation: "imports",
						Confidence: core.ConfExtracted, SourceFile: path,
						SourceLocation: locStr(line), Weight: 1,
					})
				}

			case token.TYPE:
				for _, spec := range d.Specs {
					ts := spec.(*ast.TypeSpec)
					name := ts.Name.Name
					line := fset.Position(ts.Pos()).Line
					id := addNode(name, line)

					switch t := ts.Type.(type) {
					case *ast.StructType:
						if t.Fields != nil {
							for _, field := range t.Fields.List {
								// Embedded type (no field names) -> "embeds" edge.
								if len(field.Names) == 0 {
									if name := exprName(field.Type); name != "" {
										ex.Edges = append(ex.Edges, core.Edge{
											Source: id, Target: ensureRef(&ex, defined, name, path),
											Relation: "embeds", Confidence: core.ConfExtracted,
											SourceFile: path, SourceLocation: locStr(line), Weight: 1,
										})
									}
								}
							}
						}
					case *ast.InterfaceType:
						if t.Methods != nil {
							for _, m := range t.Methods.List {
								for _, mn := range m.Names {
									mline := fset.Position(m.Pos()).Line
									methodLabel := name + "." + mn.Name
									mid := addNode(methodLabel, mline)
									defined[mn.Name] = mid
									ex.Edges = append(ex.Edges, core.Edge{
										Source: id, Target: mid, Relation: "declares",
										Confidence: core.ConfExtracted, SourceFile: path,
										SourceLocation: locStr(mline), Weight: 1,
									})
								}
							}
						}
					}
				}
			}

		case *ast.FuncDecl:
			if d.Name == nil {
				continue
			}
			line := fset.Position(d.Pos()).Line
			label := d.Name.Name
			var recvType string
			if d.Recv != nil && len(d.Recv.List) > 0 {
				if rt := exprName(d.Recv.List[0].Type); rt != "" {
					recvType = rt
					label = rt + "." + d.Name.Name
				}
			}
			id := addNode(label, line)
			// Register short name too so x.Method() calls resolve.
			if recvType != "" {
				defined[d.Name.Name] = id
				// Type → method via has_method, if the receiver type is in this file.
				if tid, ok := defined[recvType]; ok {
					ex.Edges = append(ex.Edges, core.Edge{
						Source: tid, Target: id, Relation: "has_method",
						Confidence: core.ConfExtracted, SourceFile: path,
						SourceLocation: locStr(line), Weight: 1,
					})
				}
			}
			funcs = append(funcs, funcEntry{id: id, body: d.Body})
		}
	}

	// Second pass: intra-file call resolution. ast.Inspect walks correctly
	// into nested closures, so each call gets attributed to the enclosing
	// FuncDecl — not "whatever function the regex last saw".
	for _, fn := range funcs {
		if fn.body == nil {
			continue
		}
		ast.Inspect(fn.body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			callee := callName(call.Fun)
			if callee == "" {
				return true
			}
			tgt, ok := defined[callee]
			if !ok || tgt == fn.id {
				return true
			}
			line := fset.Position(call.Pos()).Line
			ex.Edges = append(ex.Edges, core.Edge{
				Source: fn.id, Target: tgt, Relation: "calls",
				Confidence: core.ConfInferred, SourceFile: path,
				SourceLocation: locStr(line), Weight: 1,
			})
			return true
		})
	}

	ex.Nodes = dedupeNodes(ex.Nodes)
	return ex, nil
}

// exprName extracts a bare type name from an ast.Expr used in a field or
// receiver position. Handles *T, pkg.T, T[X] (generic), and plain identifiers.
func exprName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.StarExpr:
		return exprName(x.X)
	case *ast.SelectorExpr:
		return x.Sel.Name
	case *ast.IndexExpr:
		return exprName(x.X)
	case *ast.IndexListExpr:
		return exprName(x.X)
	}
	return ""
}

// callName returns the tail identifier of a call expression:
//
//	f()            -> "f"
//	x.Method()     -> "Method"
//	pkg.Fn()       -> "Fn"
//
// The caller uses this as a lookup key into a defined-symbol table.
func callName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return x.Sel.Name
	}
	return ""
}

// ensureRef returns the node id for name, creating an external-reference node
// if it isn't defined locally. Used by embeds edges so we always have a
// target.
func ensureRef(ex *core.Extraction, defined map[string]string, name, path string) string {
	if id, ok := defined[name]; ok {
		return id
	}
	id := makeID("ext", name)
	ex.Nodes = append(ex.Nodes, core.Node{
		ID: id, Label: name, FileType: core.FileCode, SourceFile: path,
	})
	return id
}
