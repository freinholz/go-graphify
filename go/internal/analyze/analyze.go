// Package analyze extracts insights from a clustered graph: most-connected
// entities, cross-community edges, and domain questions.
package analyze

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/safishamsi/graphify/go/internal/cluster"
	"github.com/safishamsi/graphify/go/internal/core"
)

// GodNode is a high-degree entity.
type GodNode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Edges int    `json:"edges"`
}

// Surprise is an edge that crosses otherwise disjoint communities.
type Surprise struct {
	Source      string           `json:"source"`
	Target      string           `json:"target"`
	Relation    string           `json:"relation"`
	Confidence  core.Confidence  `json:"confidence"`
	Note        string           `json:"note"`
	SourceFiles []string         `json:"source_files,omitempty"`
	Score       float64          `json:"score"`
}

// Result bundles the outputs of a single analysis pass.
type Result struct {
	GodNodes  []GodNode      `json:"god_nodes"`
	Surprises []Surprise     `json:"surprises"`
	Questions []string       `json:"questions"`
	Stats     core.Stats     `json:"stats"`
	Cohesion  map[int]float64 `json:"cohesion,omitempty"`
}

// Analyze produces god nodes, surprises, and suggested questions.
func Analyze(g *core.Graph, comms cluster.Result) Result {
	return Result{
		GodNodes:  GodNodes(g, 10),
		Surprises: Surprises(g, comms, 5),
		Questions: Questions(g, comms),
		Stats:     StatsOf(g, comms),
		Cohesion:  cohesionMap(g, comms),
	}
}

// GodNodes returns the top-N nodes by degree, filtering out file-level hubs
// (nodes whose label equals their filename) and method stubs.
func GodNodes(g *core.Graph, topN int) []GodNode {
	type scored struct {
		n *core.Node
		d int
	}
	var cands []scored
	for _, n := range g.Nodes() {
		if isFileHub(n) {
			continue
		}
		if strings.HasPrefix(n.Label, ".") || strings.Contains(n.Label, ".()") {
			continue
		}
		cands = append(cands, scored{n, g.Degree(n.ID)})
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].d != cands[j].d {
			return cands[i].d > cands[j].d
		}
		return cands[i].n.ID < cands[j].n.ID
	})
	if topN > len(cands) {
		topN = len(cands)
	}
	out := make([]GodNode, 0, topN)
	for _, s := range cands[:topN] {
		out = append(out, GodNode{ID: s.n.ID, Label: s.n.Label, Edges: s.d})
	}
	return out
}

// Surprises returns cross-community edges, scored by confidence and span.
func Surprises(g *core.Graph, comms cluster.Result, topN int) []Surprise {
	commOf := map[string]int{}
	for c, members := range comms {
		for _, id := range members {
			commOf[id] = c
		}
	}
	var out []Surprise
	for _, e := range g.Edges() {
		ca, ok1 := commOf[e.Source]
		cb, ok2 := commOf[e.Target]
		if !ok1 || !ok2 || ca == cb {
			continue
		}
		s, t := g.Node(e.Source), g.Node(e.Target)
		if s == nil || t == nil {
			continue
		}
		if isConcept(s) || isConcept(t) {
			continue
		}
		score := confidenceWeight(e.Confidence) + 0.5
		sp := Surprise{
			Source: s.Label, Target: t.Label, Relation: e.Relation,
			Confidence: e.Confidence, Note: "cross-community",
			Score: score,
		}
		if e.SourceFile != "" {
			sp.SourceFiles = append(sp.SourceFiles, e.SourceFile)
		}
		out = append(out, sp)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Source < out[j].Source
	})
	if topN > len(out) {
		topN = len(out)
	}
	return out[:topN]
}

// Questions produces domain-specific questions the graph can answer.
func Questions(g *core.Graph, comms cluster.Result) []string {
	gods := GodNodes(g, 3)
	var qs []string
	for _, gd := range gods {
		qs = append(qs, fmt.Sprintf("How does %s interact with the rest of the system?", gd.Label))
	}
	if len(comms) > 1 {
		qs = append(qs, fmt.Sprintf("What are the %d major modules and how do they connect?", len(comms)))
	}
	// Pick the largest community and ask about it.
	var biggest int
	for c, members := range comms {
		if len(members) > len(comms[biggest]) {
			biggest = c
		}
	}
	if len(comms[biggest]) > 0 {
		qs = append(qs, fmt.Sprintf("What's the responsibility of community %d (%d nodes)?", biggest, len(comms[biggest])))
	}
	qs = append(qs,
		"Which entities are central to the codebase?",
		"Where are the surprising cross-module dependencies?",
	)
	return qs
}

// StatsOf summarises the graph.
func StatsOf(g *core.Graph, comms cluster.Result) core.Stats {
	s := core.Stats{
		Nodes:       g.Order(),
		Edges:       g.Size(),
		Communities: len(comms),
		Confidence:  map[string]int{},
	}
	for _, e := range g.Edges() {
		s.Confidence[string(e.Confidence)]++
	}
	return s
}

func cohesionMap(g *core.Graph, comms cluster.Result) map[int]float64 {
	out := map[int]float64{}
	for c, m := range comms {
		out[c] = cluster.Cohesion(g, m)
	}
	return out
}

func confidenceWeight(c core.Confidence) float64 {
	switch c {
	case core.ConfAmbiguous:
		return 2.0
	case core.ConfInferred:
		return 1.0
	default:
		return 0.5
	}
}

// isFileHub returns true if the node looks like a file-level anchor
// (its label matches its filename).
func isFileHub(n *core.Node) bool {
	if n.SourceFile == "" {
		return false
	}
	return n.Label == filepath.Base(n.SourceFile)
}

// isConcept returns true for nodes injected manually (no source_file, no ext).
func isConcept(n *core.Node) bool {
	if n.SourceFile == "" {
		return true
	}
	if filepath.Ext(n.SourceFile) == "" {
		return true
	}
	return false
}
