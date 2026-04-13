package pipeline

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/safishamsi/graphify/go/internal/extract"
	"github.com/safishamsi/graphify/go/internal/store"
)

func init() { extract.RegisterBuiltins() }

func TestFullPipeline(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"a.py": "def foo():\n    bar()\n\ndef bar():\n    pass\n",
		"b.go": "package demo\nimport \"fmt\"\nfunc Run() { fmt.Println(1) }\n",
		"c.md": "# Topic\n\n[[Topic]] elaborates.\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := t.TempDir()
	st, err := store.NewFS(out)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Run(context.Background(), Options{
		Root: root, Store: st, UseCache: false,
		WantExports: []string{"json", "svg", "cypher"}, Logger: log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Graph.Order() == 0 {
		t.Fatal("empty graph")
	}
	if !st.ArtifactExists("graph.json") {
		t.Fatal("graph.json missing")
	}
	if !st.ArtifactExists("GRAPH_REPORT.md") {
		t.Fatal("GRAPH_REPORT.md missing")
	}
	if !st.ArtifactExists("graph.svg") {
		t.Fatal("graph.svg missing")
	}
	if !st.ArtifactExists("graph.cypher") {
		t.Fatal("graph.cypher missing")
	}
}

func TestQueryAgainstBuiltGraph(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "x.py"), []byte("def alpha():\n    beta()\n\ndef beta():\n    pass\n"), 0o644)
	out := t.TempDir()
	st, _ := store.NewFS(out)
	_, err := Run(context.Background(), Options{
		Root: root, Store: st, Logger: log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	g, err := st.LoadGraph()
	if err != nil {
		t.Fatal(err)
	}
	q := GraphQuery{G: g}
	n, ok := q.GetNode("alpha")
	if !ok {
		t.Fatal("alpha not found")
	}
	if n.Degree == 0 {
		t.Fatal("alpha should have edges")
	}
}
