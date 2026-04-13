package cluster

import (
	"testing"

	"github.com/safishamsi/graphify/go/internal/core"
)

func TestTwoComponentsSplit(t *testing.T) {
	g := core.NewGraph()
	for _, id := range []string{"a", "b", "c", "x", "y", "z"} {
		g.AddNode(core.Node{ID: id, Label: id})
	}
	// Component 1: a-b-c triangle.
	g.AddEdge(core.Edge{Source: "a", Target: "b", Relation: "r", Confidence: core.ConfExtracted})
	g.AddEdge(core.Edge{Source: "b", Target: "c", Relation: "r", Confidence: core.ConfExtracted})
	g.AddEdge(core.Edge{Source: "a", Target: "c", Relation: "r", Confidence: core.ConfExtracted})
	// Component 2: x-y-z triangle.
	g.AddEdge(core.Edge{Source: "x", Target: "y", Relation: "r", Confidence: core.ConfExtracted})
	g.AddEdge(core.Edge{Source: "y", Target: "z", Relation: "r", Confidence: core.ConfExtracted})
	g.AddEdge(core.Edge{Source: "x", Target: "z", Relation: "r", Confidence: core.ConfExtracted})
	r := Cluster(g)
	if len(r) < 2 {
		t.Fatalf("expected at least 2 communities, got %d", len(r))
	}
}

func TestCohesionBounds(t *testing.T) {
	g := core.NewGraph()
	for _, id := range []string{"a", "b", "c"} {
		g.AddNode(core.Node{ID: id, Label: id})
	}
	g.AddEdge(core.Edge{Source: "a", Target: "b", Relation: "r", Confidence: core.ConfExtracted})
	g.AddEdge(core.Edge{Source: "b", Target: "c", Relation: "r", Confidence: core.ConfExtracted})
	g.AddEdge(core.Edge{Source: "a", Target: "c", Relation: "r", Confidence: core.ConfExtracted})
	c := Cohesion(g, []string{"a", "b", "c"})
	if c <= 0 || c > 1 {
		t.Fatalf("cohesion out of range: %v", c)
	}
}
