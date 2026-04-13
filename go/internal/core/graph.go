package core

import (
	"encoding/json"
	"sort"
)

// Graph is an in-memory, undirected, adjacency-list knowledge graph keyed by
// node ID. It is intentionally simple and dependency-free: the entire pipeline
// only needs membership, degree, neighbor traversal, BFS, and community
// annotation. More sophisticated algorithms can wrap this structure.
//
// Not safe for concurrent writes.
type Graph struct {
	nodes map[string]*Node
	// adj maps node ID -> neighbor ID -> list of edges between them.
	adj map[string]map[string][]*Edge
	// edges holds every edge (each stored once even though adj is symmetric).
	edges []*Edge
}

// NewGraph returns an empty graph.
func NewGraph() *Graph {
	return &Graph{
		nodes: map[string]*Node{},
		adj:   map[string]map[string][]*Edge{},
	}
}

// AddNode inserts a node. If a node with the same ID already exists, the new
// fields overwrite the old (last-write-wins; matches the Python behavior).
func (g *Graph) AddNode(n Node) {
	cp := n
	g.nodes[n.ID] = &cp
	if _, ok := g.adj[n.ID]; !ok {
		g.adj[n.ID] = map[string][]*Edge{}
	}
}

// AddEdge inserts an edge; auto-creates endpoint stubs if they are missing.
func (g *Graph) AddEdge(e Edge) {
	if _, ok := g.nodes[e.Source]; !ok {
		g.AddNode(Node{ID: e.Source, Label: e.Source})
	}
	if _, ok := g.nodes[e.Target]; !ok {
		g.AddNode(Node{ID: e.Target, Label: e.Target})
	}
	ep := e
	g.edges = append(g.edges, &ep)
	g.adj[e.Source][e.Target] = append(g.adj[e.Source][e.Target], &ep)
	g.adj[e.Target][e.Source] = append(g.adj[e.Target][e.Source], &ep)
}

// Node returns the node with the given ID, or nil.
func (g *Graph) Node(id string) *Node { return g.nodes[id] }

// Has reports whether the graph contains a node with this ID.
func (g *Graph) Has(id string) bool { _, ok := g.nodes[id]; return ok }

// Nodes returns a slice of all nodes (order undefined).
func (g *Graph) Nodes() []*Node {
	out := make([]*Node, 0, len(g.nodes))
	for _, n := range g.nodes {
		out = append(out, n)
	}
	return out
}

// Edges returns all edges.
func (g *Graph) Edges() []*Edge { return g.edges }

// Neighbors returns the IDs directly connected to id.
func (g *Graph) Neighbors(id string) []string {
	m := g.adj[id]
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// EdgesBetween returns all edges between a and b (either direction).
func (g *Graph) EdgesBetween(a, b string) []*Edge {
	if m, ok := g.adj[a]; ok {
		return m[b]
	}
	return nil
}

// Degree returns the number of distinct neighbors (not edge multiplicity).
func (g *Graph) Degree(id string) int { return len(g.adj[id]) }

// Order returns the number of nodes.
func (g *Graph) Order() int { return len(g.nodes) }

// Size returns the number of edges.
func (g *Graph) Size() int { return len(g.edges) }

// FindByLabel looks up a node by its human label (case-sensitive). If multiple
// nodes share the label, the first (by sorted ID) is returned.
func (g *Graph) FindByLabel(label string) *Node {
	var ids []string
	for id, n := range g.nodes {
		if n.Label == label {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	sort.Strings(ids)
	return g.nodes[ids[0]]
}

// BFS performs breadth-first traversal from start up to maxDepth hops.
// visit is called for every node reached (including start). If visit returns
// false, traversal stops.
func (g *Graph) BFS(start string, maxDepth int, visit func(id string, depth int) bool) {
	if _, ok := g.nodes[start]; !ok {
		return
	}
	seen := map[string]int{start: 0}
	queue := []string{start}
	for len(queue) > 0 {
		head := queue[0]
		queue = queue[1:]
		d := seen[head]
		if !visit(head, d) {
			return
		}
		if d >= maxDepth {
			continue
		}
		for _, nb := range g.Neighbors(head) {
			if _, v := seen[nb]; !v {
				seen[nb] = d + 1
				queue = append(queue, nb)
			}
		}
	}
}

// ShortestPath returns a shortest path (by hop count) from src to dst, or nil.
func (g *Graph) ShortestPath(src, dst string) []string {
	if src == dst {
		return []string{src}
	}
	if _, ok := g.nodes[src]; !ok {
		return nil
	}
	prev := map[string]string{src: ""}
	queue := []string{src}
	for len(queue) > 0 {
		head := queue[0]
		queue = queue[1:]
		for _, nb := range g.Neighbors(head) {
			if _, ok := prev[nb]; ok {
				continue
			}
			prev[nb] = head
			if nb == dst {
				// Reconstruct.
				var path []string
				for cur := dst; cur != ""; cur = prev[cur] {
					path = append([]string{cur}, path...)
					if cur == src {
						break
					}
				}
				return path
			}
			queue = append(queue, nb)
		}
	}
	return nil
}

// NodeLink is the vis.js / NetworkX node-link JSON format.
type NodeLink struct {
	Directed   bool              `json:"directed"`
	Multigraph bool              `json:"multigraph"`
	Graph      map[string]any    `json:"graph"`
	Nodes      []json.RawMessage `json:"nodes"`
	Links      []json.RawMessage `json:"links"`
}

// ToJSON serialises the graph in node-link form, compatible with the Python
// graphify graph.json schema.
func (g *Graph) ToJSON() ([]byte, error) {
	type nodeOut struct {
		ID             string   `json:"id"`
		Label          string   `json:"label"`
		FileType       FileType `json:"file_type,omitempty"`
		SourceFile     string   `json:"source_file,omitempty"`
		SourceLocation string   `json:"source_location,omitempty"`
		Community      int      `json:"community,omitempty"`
	}
	type edgeOut struct {
		Source         string     `json:"source"`
		Target         string     `json:"target"`
		Relation       string     `json:"relation"`
		Confidence     Confidence `json:"confidence"`
		SourceFile     string     `json:"source_file,omitempty"`
		SourceLocation string     `json:"source_location,omitempty"`
		Weight         float64    `json:"weight,omitempty"`
	}
	var ns []nodeOut
	ids := make([]string, 0, len(g.nodes))
	for id := range g.nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		n := g.nodes[id]
		ns = append(ns, nodeOut{
			ID: n.ID, Label: n.Label, FileType: n.FileType,
			SourceFile: n.SourceFile, SourceLocation: n.SourceLocation,
			Community: n.Community,
		})
	}
	var es []edgeOut
	for _, e := range g.edges {
		es = append(es, edgeOut{
			Source: e.Source, Target: e.Target, Relation: e.Relation,
			Confidence: e.Confidence, SourceFile: e.SourceFile,
			SourceLocation: e.SourceLocation, Weight: e.Weight,
		})
	}
	return json.MarshalIndent(map[string]any{
		"directed":   false,
		"multigraph": true,
		"graph":      map[string]any{},
		"nodes":      ns,
		"links":      es,
	}, "", "  ")
}

// FromJSON loads a node-link JSON payload into a Graph.
func FromJSON(data []byte) (*Graph, error) {
	var raw struct {
		Nodes []Node `json:"nodes"`
		Links []Edge `json:"links"`
		Edges []Edge `json:"edges"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	g := NewGraph()
	for _, n := range raw.Nodes {
		g.AddNode(n)
	}
	edges := raw.Links
	if len(edges) == 0 {
		edges = raw.Edges
	}
	for _, e := range edges {
		g.AddEdge(e)
	}
	return g, nil
}
