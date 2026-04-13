package pipeline

import (
	"sort"
	"strings"

	"github.com/safishamsi/graphify/go/internal/core"
)

// GraphQuery wraps a loaded Graph with the set of operations the Python MCP
// server exposed (get_node, neighbors, community, god_nodes, shortest_path,
// BFS question search). It is intentionally transport-neutral: the CLI drives
// it locally now; a future HTTP/gRPC server will drive it remotely.
type GraphQuery struct{ G *core.Graph }

// NodeView is a lightweight serialisable node.
type NodeView struct {
	ID             string `json:"id"`
	Label          string `json:"label"`
	FileType       string `json:"file_type,omitempty"`
	SourceFile     string `json:"source_file,omitempty"`
	SourceLocation string `json:"source_location,omitempty"`
	Community      int    `json:"community"`
	Degree         int    `json:"degree"`
}

func (q GraphQuery) view(n *core.Node) NodeView {
	return NodeView{
		ID: n.ID, Label: n.Label, FileType: string(n.FileType),
		SourceFile: n.SourceFile, SourceLocation: n.SourceLocation,
		Community: n.Community, Degree: q.G.Degree(n.ID),
	}
}

// GetNode returns a node by exact label (or ID fallback).
func (q GraphQuery) GetNode(labelOrID string) (NodeView, bool) {
	if n := q.G.Node(labelOrID); n != nil {
		return q.view(n), true
	}
	if n := q.G.FindByLabel(labelOrID); n != nil {
		return q.view(n), true
	}
	return NodeView{}, false
}

// Neighbor is a direct neighbor with edge info.
type Neighbor struct {
	Node       NodeView `json:"node"`
	Relation   string   `json:"relation"`
	Confidence string   `json:"confidence"`
}

// Neighbors returns direct neighbors of labelOrID, optionally filtered by
// relation.
func (q GraphQuery) Neighbors(labelOrID, relationFilter string) []Neighbor {
	nv, ok := q.GetNode(labelOrID)
	if !ok {
		return nil
	}
	var out []Neighbor
	for _, nb := range q.G.Neighbors(nv.ID) {
		nb := nb
		for _, e := range q.G.EdgesBetween(nv.ID, nb) {
			if relationFilter != "" && e.Relation != relationFilter {
				continue
			}
			n := q.G.Node(nb)
			if n == nil {
				continue
			}
			out = append(out, Neighbor{
				Node: q.view(n), Relation: e.Relation, Confidence: string(e.Confidence),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Node.Degree != out[j].Node.Degree {
			return out[i].Node.Degree > out[j].Node.Degree
		}
		return out[i].Node.Label < out[j].Node.Label
	})
	return out
}

// Community returns every node in community c.
func (q GraphQuery) Community(c int) []NodeView {
	var out []NodeView
	for _, n := range q.G.Nodes() {
		if n.Community == c {
			out = append(out, q.view(n))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Degree != out[j].Degree {
			return out[i].Degree > out[j].Degree
		}
		return out[i].Label < out[j].Label
	})
	return out
}

// ShortestPath between two labels.
func (q GraphQuery) ShortestPath(from, to string, maxHops int) []NodeView {
	a, ok := q.GetNode(from)
	if !ok {
		return nil
	}
	b, ok := q.GetNode(to)
	if !ok {
		return nil
	}
	path := q.G.ShortestPath(a.ID, b.ID)
	if path == nil || (maxHops > 0 && len(path)-1 > maxHops) {
		return nil
	}
	var out []NodeView
	for _, id := range path {
		if n := q.G.Node(id); n != nil {
			out = append(out, q.view(n))
		}
	}
	return out
}

// Question runs a simple keyword BFS: matches label substrings against q,
// then expands `depth` hops, returning a context digest.
func (q GraphQuery) Question(question string, depth int, budget int) []NodeView {
	if depth <= 0 {
		depth = 2
	}
	keys := tokenize(question)
	seeds := map[string]bool{}
	for _, n := range q.G.Nodes() {
		for _, k := range keys {
			if strings.Contains(strings.ToLower(n.Label), k) {
				seeds[n.ID] = true
				break
			}
		}
	}
	visited := map[string]bool{}
	var out []NodeView
	for id := range seeds {
		q.G.BFS(id, depth, func(x string, d int) bool {
			if visited[x] {
				return true
			}
			visited[x] = true
			if n := q.G.Node(x); n != nil {
				out = append(out, q.view(n))
			}
			if budget > 0 && len(out) >= budget {
				return false
			}
			return true
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Degree != out[j].Degree {
			return out[i].Degree > out[j].Degree
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	var cur strings.Builder
	var out []string
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			cur.WriteRune(r)
		} else if cur.Len() > 0 {
			if cur.Len() > 2 {
				out = append(out, cur.String())
			}
			cur.Reset()
		}
	}
	if cur.Len() > 2 {
		out = append(out, cur.String())
	}
	return out
}
