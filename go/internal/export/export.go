// Package export renders a graph to various downstream formats.
//
// The exporters do not perform I/O directly; they produce bytes and the caller
// (typically the CLI or a future server) writes them into a Store. This keeps
// export logic testable and allows a hosted backend to ship artifacts over the
// wire instead of to disk.
package export

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/safishamsi/graphify/go/internal/core"
)

// CommunityColors are the ten-colour palette used for visualisations.
var CommunityColors = []string{
	"#4E79A7", "#F28E2B", "#E15759", "#76B7B2", "#59A14F",
	"#EDC948", "#B07AA1", "#FF9DA7", "#9C755F", "#BAB0AC",
}

// SafeName sanitises a label for filesystem use (removes /*?:"<>|#^[]).
func SafeName(label string) string {
	var b strings.Builder
	for _, r := range label {
		switch r {
		case '/', '\\', '*', '?', ':', '"', '<', '>', '|', '#', '^', '[', ']':
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// ToJSON serialises the graph in node-link form, with viz-friendly colours.
func ToJSON(g *core.Graph) ([]byte, error) {
	type ndOut struct {
		ID             string `json:"id"`
		Label          string `json:"label"`
		FileType       string `json:"file_type,omitempty"`
		SourceFile     string `json:"source_file,omitempty"`
		SourceLocation string `json:"source_location,omitempty"`
		Community      int    `json:"community"`
		Color          string `json:"color,omitempty"`
		Size           int    `json:"size,omitempty"`
	}
	type edOut struct {
		From       string `json:"from"`
		To         string `json:"to"`
		Relation   string `json:"relation"`
		Confidence string `json:"confidence"`
	}
	var nodes []ndOut
	ids := sortedIDs(g)
	for _, id := range ids {
		n := g.Node(id)
		nodes = append(nodes, ndOut{
			ID: n.ID, Label: n.Label, FileType: string(n.FileType),
			SourceFile: n.SourceFile, SourceLocation: n.SourceLocation,
			Community: n.Community, Color: colorFor(n.Community),
			Size: 5 + g.Degree(n.ID),
		})
	}
	var edges []edOut
	for _, e := range g.Edges() {
		edges = append(edges, edOut{
			From: e.Source, To: e.Target,
			Relation: e.Relation, Confidence: string(e.Confidence),
		})
	}
	return json.MarshalIndent(map[string]any{
		"nodes": nodes, "edges": edges,
	}, "", "  ")
}

func colorFor(c int) string {
	if c < 0 {
		return "#888"
	}
	return CommunityColors[c%len(CommunityColors)]
}

func sortedIDs(g *core.Graph) []string {
	ids := make([]string, 0, g.Order())
	for _, n := range g.Nodes() {
		ids = append(ids, n.ID)
	}
	sort.Strings(ids)
	return ids
}

// ToCypher emits Neo4j Cypher CREATE statements.
func ToCypher(g *core.Graph) []byte {
	var b strings.Builder
	for _, id := range sortedIDs(g) {
		n := g.Node(id)
		fmt.Fprintf(&b, "CREATE (:%s {id: %q, label: %q, community: %d});\n",
			cypherLabel(string(n.FileType)), n.ID, n.Label, n.Community)
	}
	for _, e := range g.Edges() {
		fmt.Fprintf(&b, "MATCH (a {id: %q}), (b {id: %q}) CREATE (a)-[:%s {confidence: %q}]->(b);\n",
			e.Source, e.Target, cypherRel(e.Relation), e.Confidence)
	}
	return []byte(b.String())
}

func cypherLabel(ft string) string {
	if ft == "" {
		return "Node"
	}
	return strings.Title(ft)
}

func cypherRel(r string) string {
	if r == "" {
		return "RELATED"
	}
	return strings.ToUpper(strings.ReplaceAll(r, " ", "_"))
}

// ToCanvas emits an Obsidian Canvas JSON.
func ToCanvas(g *core.Graph) []byte {
	type node struct {
		ID     string `json:"id"`
		Type   string `json:"type"`
		Text   string `json:"text"`
		X      int    `json:"x"`
		Y      int    `json:"y"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
		Color  string `json:"color,omitempty"`
	}
	type edge struct {
		ID       string `json:"id"`
		From     string `json:"fromNode"`
		To       string `json:"toNode"`
		Label    string `json:"label,omitempty"`
		FromSide string `json:"fromSide"`
		ToSide   string `json:"toSide"`
	}
	nodes := []node{}
	// Very simple grid layout.
	ids := sortedIDs(g)
	cols := 6
	for i, id := range ids {
		n := g.Node(id)
		nodes = append(nodes, node{
			ID: id, Type: "text", Text: n.Label,
			X: (i % cols) * 260, Y: (i / cols) * 120,
			Width: 240, Height: 80, Color: colorFor(n.Community),
		})
	}
	edges := []edge{}
	for i, e := range g.Edges() {
		edges = append(edges, edge{
			ID: fmt.Sprintf("e%d", i), From: e.Source, To: e.Target,
			Label: e.Relation, FromSide: "right", ToSide: "left",
		})
	}
	b, _ := json.MarshalIndent(map[string]any{"nodes": nodes, "edges": edges}, "", "  ")
	// Canvas expects field "nodes"/"edges" with specific name; keep as-is.
	// Map "width"/"height" properly.
	b = []byte(strings.ReplaceAll(string(b), `"width":`, `"width":`))
	return b
}
