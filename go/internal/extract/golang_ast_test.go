package extract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/safishamsi/graphify/go/internal/core"
)

// writeGo stages a .go file and returns its path.
func writeGo(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "m.go")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func nodesByLabel(ex core.Extraction) map[string]core.Node {
	m := map[string]core.Node{}
	for _, n := range ex.Nodes {
		m[n.Label] = n
	}
	return m
}

func edgeExists(ex core.Extraction, srcLabel, tgtLabel, rel string) bool {
	idOf := map[string]string{}
	for _, n := range ex.Nodes {
		idOf[n.Label] = n.ID
	}
	src, tgt := idOf[srcLabel], idOf[tgtLabel]
	for _, e := range ex.Edges {
		if e.Source == src && e.Target == tgt && e.Relation == rel {
			return true
		}
	}
	return false
}

// TestCommentKeywordsDoNotMatchCalls is the canonical case the regex extractor
// cannot get right: the word "function" or a function-looking token inside a
// comment must not become a node or an edge.
func TestGoAST_IgnoresCommentsAndStrings(t *testing.T) {
	p := writeGo(t, `package demo

// This helper() is described in the docs.
// Look for notes about foo() below.
func real() {
	s := "bar() is just a string"
	_ = s
}
`)
	ex, err := Extract(p)
	if err != nil {
		t.Fatal(err)
	}
	labels := nodesByLabel(ex)
	if _, ok := labels["helper"]; ok {
		t.Fatal("comment token 'helper' leaked into graph")
	}
	if _, ok := labels["foo"]; ok {
		t.Fatal("comment token 'foo' leaked into graph")
	}
	if _, ok := labels["bar"]; ok {
		t.Fatal("string token 'bar' leaked into graph")
	}
	if _, ok := labels["real"]; !ok {
		t.Fatal("real function should be extracted")
	}
}

// TestControlKeywordsNotCalls verifies that if/for/switch parens are not
// confused with function calls — another regex failure mode.
func TestGoAST_ControlKeywordsAreNotCalls(t *testing.T) {
	p := writeGo(t, `package demo

func outer() {
	if ready() {
		for i := 0; i < 10; i++ {
			_ = i
		}
	}
}

func ready() bool { return true }
`)
	ex, err := Extract(p)
	if err != nil {
		t.Fatal(err)
	}
	// Must have exactly one edge "outer -> ready (calls)"; no edges for if/for.
	if !edgeExists(ex, "outer", "ready", "calls") {
		t.Fatal("expected outer -> ready calls edge")
	}
	for _, e := range ex.Edges {
		if e.Relation != "calls" {
			continue
		}
		for _, n := range ex.Nodes {
			if n.ID == e.Target && (n.Label == "if" || n.Label == "for" || n.Label == "switch") {
				t.Fatalf("control keyword leaked as call target: %s", n.Label)
			}
		}
	}
}

// TestNestedFunctionScoping — the failure case we observed earlier with the
// regex extractor (where _make_id accumulated 51 bogus edges to walk).
func TestGoAST_NestedClosureScoping(t *testing.T) {
	p := writeGo(t, `package demo

func outer() {
	inner := func() {
		target()
	}
	inner()
}

func target() {}
`)
	ex, err := Extract(p)
	if err != nil {
		t.Fatal(err)
	}
	// target() is called from inside the closure assigned in outer; in our
	// v1 model we attribute it to the enclosing FuncDecl (outer), exactly
	// once — not to whatever decl the regex scanner last saw.
	count := 0
	for _, e := range ex.Edges {
		if e.Relation == "calls" {
			src := ""
			tgt := ""
			for _, n := range ex.Nodes {
				if n.ID == e.Source {
					src = n.Label
				}
				if n.ID == e.Target {
					tgt = n.Label
				}
			}
			if src == "outer" && tgt == "target" {
				count++
			}
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly one outer->target call edge, got %d", count)
	}
}

// TestMethodAndReceiver covers pointer-receiver methods and the has_method
// edge to the owning type.
func TestGoAST_MethodOnPointerReceiver(t *testing.T) {
	p := writeGo(t, `package demo

type Server struct{}

func (s *Server) Run() {
	s.Prepare()
}

func (s *Server) Prepare() {}
`)
	ex, err := Extract(p)
	if err != nil {
		t.Fatal(err)
	}
	labels := nodesByLabel(ex)
	for _, want := range []string{"Server", "Server.Run", "Server.Prepare"} {
		if _, ok := labels[want]; !ok {
			t.Fatalf("missing node %q", want)
		}
	}
	if !edgeExists(ex, "Server", "Server.Run", "has_method") {
		t.Fatal("expected Server has_method Server.Run")
	}
	if !edgeExists(ex, "Server.Run", "Server.Prepare", "calls") {
		t.Fatal("expected Server.Run -> Server.Prepare call edge")
	}
}

// TestEmbeddedStruct -> embeds edge.
func TestGoAST_EmbedsEdge(t *testing.T) {
	p := writeGo(t, `package demo

type Base struct{}
type Child struct {
	Base
	name string
}
`)
	ex, err := Extract(p)
	if err != nil {
		t.Fatal(err)
	}
	if !edgeExists(ex, "Child", "Base", "embeds") {
		t.Fatal("expected Child embeds Base")
	}
}

// TestInterfaceMethodsDeclared -> declares edges.
func TestGoAST_InterfaceDeclares(t *testing.T) {
	p := writeGo(t, `package demo

type Store interface {
	Get(k string) ([]byte, error)
	Put(k string, v []byte) error
}
`)
	ex, err := Extract(p)
	if err != nil {
		t.Fatal(err)
	}
	if !edgeExists(ex, "Store", "Store.Get", "declares") {
		t.Fatal("expected Store declares Store.Get")
	}
	if !edgeExists(ex, "Store", "Store.Put", "declares") {
		t.Fatal("expected Store declares Store.Put")
	}
}

// TestImportPathsExact.
func TestGoAST_Imports(t *testing.T) {
	p := writeGo(t, `package demo

import (
	"fmt"
	os "os/exec"
	_ "embed"
)

func run() { fmt.Println("hi") }
`)
	ex, err := Extract(p)
	if err != nil {
		t.Fatal(err)
	}
	labels := nodesByLabel(ex)
	for _, want := range []string{"fmt", "os/exec", "embed"} {
		if _, ok := labels[want]; !ok {
			t.Fatalf("missing import node %q", want)
		}
	}
}

// TestPartialParseTolerance — a syntactically broken file should not crash
// the extractor; whatever did parse should still be emitted.
func TestGoAST_ToleratesPartialFile(t *testing.T) {
	p := writeGo(t, `package demo

func Complete() {}

func Broken( // missing body
`)
	ex, err := Extract(p)
	if err != nil {
		t.Fatal(err)
	}
	labels := nodesByLabel(ex)
	if _, ok := labels["Complete"]; !ok {
		t.Fatal("well-formed function should still appear in partial file")
	}
	// Broken's label should not leak unrelated nodes.
	var brokenish []string
	for lbl := range labels {
		if strings.Contains(lbl, "Broken") {
			brokenish = append(brokenish, lbl)
		}
	}
	if len(brokenish) > 1 {
		t.Fatalf("too many broken-function artifacts: %v", brokenish)
	}
}
