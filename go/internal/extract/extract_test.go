package extract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func init() { RegisterBuiltins() }

func writeTemp(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPythonExtract(t *testing.T) {
	p := writeTemp(t, "m.py", `import os
from pathlib import Path

class Greeter(Base):
    def hello(self):
        greet("world")

def greet(name):
    return name
`)
	ex, err := Extract(p)
	if err != nil {
		t.Fatal(err)
	}
	var gotGreet, gotGreeter, gotImport bool
	for _, n := range ex.Nodes {
		if n.Label == "greet" {
			gotGreet = true
		}
		if n.Label == "Greeter" {
			gotGreeter = true
		}
		if n.Label == "os" {
			gotImport = true
		}
	}
	if !(gotGreet && gotGreeter && gotImport) {
		t.Fatalf("missing expected nodes: greet=%v Greeter=%v os=%v", gotGreet, gotGreeter, gotImport)
	}
	var containsEdge, inheritsEdge, callsEdge bool
	for _, e := range ex.Edges {
		if e.Relation == "contains" {
			containsEdge = true
		}
		if e.Relation == "inherits" {
			inheritsEdge = true
		}
		if e.Relation == "calls" {
			callsEdge = true
		}
	}
	if !(containsEdge && inheritsEdge && callsEdge) {
		t.Fatalf("missing edges: contains=%v inherits=%v calls=%v", containsEdge, inheritsEdge, callsEdge)
	}
}

func TestGoExtract(t *testing.T) {
	p := writeTemp(t, "m.go", `package demo

import (
	"fmt"
	"os"
)

type Widget struct{ name string }

func (w Widget) Greet() string { return fmt.Sprintf("hi %s", w.name) }

func main() { os.Exit(0) }
`)
	ex, err := Extract(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(ex.Nodes) < 3 {
		t.Fatalf("expected at least 3 nodes, got %d", len(ex.Nodes))
	}
	var sawFmt bool
	for _, n := range ex.Nodes {
		if n.Label == "fmt" {
			sawFmt = true
		}
	}
	if !sawFmt {
		t.Fatalf("expected fmt import node")
	}
}

func TestDocumentExtract(t *testing.T) {
	p := writeTemp(t, "note.md", `# Intro

See [[Related]] for more.

## Subsection
`)
	ex, err := Extract(p)
	if err != nil {
		t.Fatal(err)
	}
	labels := []string{}
	for _, n := range ex.Nodes {
		labels = append(labels, n.Label)
	}
	joined := strings.Join(labels, ",")
	if !strings.Contains(joined, "Intro") || !strings.Contains(joined, "Related") {
		t.Fatalf("missing expected labels, got %s", joined)
	}
}
