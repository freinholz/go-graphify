// Package cluster implements community detection on a graphify Graph.
//
// The v1 algorithm is a Louvain-style greedy modularity optimiser; it is
// dependency-free and deterministic (node-id sorted). A Leiden implementation
// can be wired in later behind the same public API.
package cluster

import (
	"sort"

	"github.com/safishamsi/graphify/go/internal/core"
)

// Result maps community ID (int) to sorted node-id slice.
type Result map[int][]string

// Cluster assigns every node to a community, mutating the graph in place via
// Node.Community, and returns a Result mapping.
func Cluster(g *core.Graph) Result {
	comm := louvain(g)
	for id, c := range comm {
		n := g.Node(id)
		if n != nil {
			n.Community = c
		}
	}
	return groupByCommunity(comm)
}

// Cohesion returns intra-community edge density for a community.
func Cohesion(g *core.Graph, members []string) float64 {
	if len(members) < 2 {
		return 0
	}
	inSet := map[string]bool{}
	for _, m := range members {
		inSet[m] = true
	}
	intra := 0
	for _, a := range members {
		for _, b := range g.Neighbors(a) {
			if inSet[b] && a < b {
				intra++
			}
		}
	}
	maxEdges := len(members) * (len(members) - 1) / 2
	if maxEdges == 0 {
		return 0
	}
	return float64(intra) / float64(maxEdges)
}

func groupByCommunity(assign map[string]int) Result {
	r := Result{}
	for id, c := range assign {
		r[c] = append(r[c], id)
	}
	for _, v := range r {
		sort.Strings(v)
	}
	return r
}

// louvain runs a single-level Louvain on the undirected graph.
// Returns node-id -> community-id.
func louvain(g *core.Graph) map[string]int {
	nodes := g.Nodes()
	if len(nodes) == 0 {
		return map[string]int{}
	}
	// Stable node ordering for determinism.
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })

	// Initial: each node in its own community.
	comm := map[string]int{}
	for i, n := range nodes {
		comm[n.ID] = i
	}

	// Precompute weights.
	// Treat multi-edges as weight = sum of edge weights (default 1).
	weight := func(a, b string) float64 {
		w := 0.0
		for _, e := range g.EdgesBetween(a, b) {
			ew := e.Weight
			if ew == 0 {
				ew = 1
			}
			w += ew
		}
		return w
	}
	degree := map[string]float64{}
	m2 := 0.0 // sum of weights * 2
	for _, n := range nodes {
		d := 0.0
		for _, nb := range g.Neighbors(n.ID) {
			d += weight(n.ID, nb)
		}
		degree[n.ID] = d
		m2 += d
	}
	if m2 == 0 {
		return comm
	}
	// Community aggregates.
	sumTot := map[int]float64{}
	for id, c := range comm {
		sumTot[c] += degree[id]
	}

	// One pass: for each node, try moving to a neighbor's community if gain > 0.
	changed := true
	for iter := 0; iter < 16 && changed; iter++ {
		changed = false
		for _, n := range nodes {
			curC := comm[n.ID]
			ki := degree[n.ID]
			// Remove from current.
			sumTot[curC] -= ki
			// Candidate communities = self plus neighbors'.
			best := curC
			bestGain := 0.0
			// kiIn(c) = sum of weights from i to nodes in community c.
			kiIn := map[int]float64{}
			for _, nb := range g.Neighbors(n.ID) {
				if nb == n.ID {
					continue
				}
				kiIn[comm[nb]] += weight(n.ID, nb)
			}
			for c, kin := range kiIn {
				gain := kin - sumTot[c]*ki/m2
				if gain > bestGain {
					bestGain = gain
					best = c
				}
			}
			// Place node in best community.
			comm[n.ID] = best
			sumTot[best] += ki
			if best != curC {
				changed = true
			}
		}
	}
	// Renumber communities compactly.
	renum := map[int]int{}
	next := 0
	// Deterministic order by smallest node id in each community.
	firstID := map[int]string{}
	for id, c := range comm {
		if f, ok := firstID[c]; !ok || id < f {
			firstID[c] = id
		}
	}
	commIDs := make([]int, 0, len(firstID))
	for c := range firstID {
		commIDs = append(commIDs, c)
	}
	sort.Slice(commIDs, func(i, j int) bool {
		return firstID[commIDs[i]] < firstID[commIDs[j]]
	})
	for _, c := range commIDs {
		renum[c] = next
		next++
	}
	for id, c := range comm {
		comm[id] = renum[c]
	}
	return comm
}
