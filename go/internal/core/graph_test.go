package core

import (
	"reflect"
	"testing"
)

func TestGraphAddAndDegree(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{ID: "a", Label: "A"})
	g.AddNode(Node{ID: "b", Label: "B"})
	g.AddEdge(Edge{Source: "a", Target: "b", Relation: "uses", Confidence: ConfExtracted})
	if g.Order() != 2 || g.Size() != 1 {
		t.Fatalf("order/size mismatch: %d %d", g.Order(), g.Size())
	}
	if g.Degree("a") != 1 || g.Degree("b") != 1 {
		t.Fatalf("expected degree 1 on both ends")
	}
	nb := g.Neighbors("a")
	if !reflect.DeepEqual(nb, []string{"b"}) {
		t.Fatalf("neighbors(a) = %v", nb)
	}
}

func TestShortestPath(t *testing.T) {
	g := NewGraph()
	for _, id := range []string{"a", "b", "c", "d"} {
		g.AddNode(Node{ID: id, Label: id})
	}
	g.AddEdge(Edge{Source: "a", Target: "b", Relation: "x", Confidence: ConfExtracted})
	g.AddEdge(Edge{Source: "b", Target: "c", Relation: "x", Confidence: ConfExtracted})
	g.AddEdge(Edge{Source: "c", Target: "d", Relation: "x", Confidence: ConfExtracted})
	p := g.ShortestPath("a", "d")
	if !reflect.DeepEqual(p, []string{"a", "b", "c", "d"}) {
		t.Fatalf("path = %v", p)
	}
}

func TestJSONRoundTrip(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{ID: "a", Label: "A", FileType: FileCode, SourceFile: "a.go"})
	g.AddNode(Node{ID: "b", Label: "B", FileType: FileCode, SourceFile: "b.go"})
	g.AddEdge(Edge{Source: "a", Target: "b", Relation: "uses", Confidence: ConfExtracted, Weight: 1})
	b, err := g.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	g2, err := FromJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	if g2.Order() != 2 || g2.Size() != 1 {
		t.Fatalf("round-trip lost data: %d %d", g2.Order(), g2.Size())
	}
}
