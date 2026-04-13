// Package report renders a markdown audit trail (GRAPH_REPORT.md).
package report

import (
	"fmt"
	"sort"
	"strings"

	"github.com/safishamsi/graphify/go/internal/analyze"
	"github.com/safishamsi/graphify/go/internal/cluster"
	"github.com/safishamsi/graphify/go/internal/core"
)

// Render produces the full GRAPH_REPORT.md string.
func Render(g *core.Graph, res analyze.Result, comms cluster.Result) string {
	var b strings.Builder
	b.WriteString("# Graph Report\n\n")

	// Summary.
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "- Nodes: **%d**\n", res.Stats.Nodes)
	fmt.Fprintf(&b, "- Edges: **%d**\n", res.Stats.Edges)
	fmt.Fprintf(&b, "- Communities: **%d**\n\n", res.Stats.Communities)
	if len(res.Stats.Confidence) > 0 {
		b.WriteString("**Confidence breakdown**\n\n")
		keys := make([]string, 0, len(res.Stats.Confidence))
		for k := range res.Stats.Confidence {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "- %s: %d\n", k, res.Stats.Confidence[k])
		}
		b.WriteString("\n")
	}

	// God nodes.
	if len(res.GodNodes) > 0 {
		b.WriteString("## God Nodes\n\n")
		b.WriteString("| Rank | Label | Edges |\n|---:|---|---:|\n")
		for i, gn := range res.GodNodes {
			fmt.Fprintf(&b, "| %d | [[%s]] | %d |\n", i+1, gn.Label, gn.Edges)
		}
		b.WriteString("\n")
	}

	// Surprising connections.
	if len(res.Surprises) > 0 {
		b.WriteString("## Surprising Connections\n\n")
		for _, s := range res.Surprises {
			fmt.Fprintf(&b, "- **[[%s]] → [[%s]]** (`%s`, %s) — %s\n",
				s.Source, s.Target, s.Relation, s.Confidence, s.Note)
		}
		b.WriteString("\n")
	}

	// Communities.
	if len(comms) > 0 {
		b.WriteString("## Communities\n\n")
		ids := make([]int, 0, len(comms))
		for c := range comms {
			ids = append(ids, c)
		}
		sort.Ints(ids)
		for _, c := range ids {
			members := comms[c]
			fmt.Fprintf(&b, "### Community %d (%d nodes)\n\n", c, len(members))
			// Top 8 by degree.
			type sd struct {
				id    string
				label string
				deg   int
			}
			sds := make([]sd, 0, len(members))
			for _, id := range members {
				n := g.Node(id)
				if n == nil {
					continue
				}
				sds = append(sds, sd{id, n.Label, g.Degree(id)})
			}
			sort.Slice(sds, func(i, j int) bool {
				if sds[i].deg != sds[j].deg {
					return sds[i].deg > sds[j].deg
				}
				return sds[i].id < sds[j].id
			})
			if len(sds) > 8 {
				sds = sds[:8]
			}
			for _, s := range sds {
				fmt.Fprintf(&b, "- [[%s]] (%d)\n", s.label, s.deg)
			}
			b.WriteString("\n")
		}
	}

	// Suggested questions.
	if len(res.Questions) > 0 {
		b.WriteString("## Suggested Questions\n\n")
		for _, q := range res.Questions {
			fmt.Fprintf(&b, "- %s\n", q)
		}
		b.WriteString("\n")
	}

	// Knowledge gaps.
	if isolates := findIsolates(g); len(isolates) > 0 {
		b.WriteString("## Knowledge Gaps\n\n")
		fmt.Fprintf(&b, "- %d isolated nodes (no edges)\n", len(isolates))
		if len(isolates) > 10 {
			isolates = isolates[:10]
		}
		for _, n := range isolates {
			fmt.Fprintf(&b, "  - `%s`\n", n.Label)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func findIsolates(g *core.Graph) []*core.Node {
	var out []*core.Node
	for _, n := range g.Nodes() {
		if g.Degree(n.ID) == 0 {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
