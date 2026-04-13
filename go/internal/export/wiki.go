package export

import (
	"fmt"
	"sort"
	"strings"

	"github.com/safishamsi/graphify/go/internal/cluster"
	"github.com/safishamsi/graphify/go/internal/core"
)

// WikiFile is one rendered markdown page to be stored by the caller.
type WikiFile struct {
	RelPath string // e.g. "wiki/index.md"
	Body    []byte
}

// ToWiki renders a Wikipedia-style markdown vault (one file per community,
// one per god node, plus an index).
func ToWiki(g *core.Graph, comms cluster.Result) []WikiFile {
	files := []WikiFile{indexPage(g, comms)}
	for c, members := range comms {
		files = append(files, communityPage(g, c, members))
	}
	// Community-size sorted order for index determinism done in indexPage.
	return files
}

func indexPage(g *core.Graph, comms cluster.Result) WikiFile {
	var b strings.Builder
	fmt.Fprintf(&b, "# Knowledge Vault\n\n")
	fmt.Fprintf(&b, "**%d nodes · %d edges · %d communities**\n\n", g.Order(), g.Size(), len(comms))
	type row struct {
		c, n int
	}
	rows := make([]row, 0, len(comms))
	for c, m := range comms {
		rows = append(rows, row{c, len(m)})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].n > rows[j].n })
	b.WriteString("## Communities\n\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "- [[_COMMUNITY_%d]] — %d nodes\n", r.c, r.n)
	}
	return WikiFile{RelPath: "wiki/index.md", Body: []byte(b.String())}
}

func communityPage(g *core.Graph, id int, members []string) WikiFile {
	var b strings.Builder
	fmt.Fprintf(&b, "# Community %d\n\n", id)
	fmt.Fprintf(&b, "**%d nodes**\n\n", len(members))

	// Top nodes by degree.
	type sd struct {
		id    string
		label string
		deg   int
	}
	var tops []sd
	for _, m := range members {
		n := g.Node(m)
		if n == nil {
			continue
		}
		tops = append(tops, sd{m, n.Label, g.Degree(m)})
	}
	sort.Slice(tops, func(i, j int) bool {
		if tops[i].deg != tops[j].deg {
			return tops[i].deg > tops[j].deg
		}
		return tops[i].id < tops[j].id
	})
	if len(tops) > 25 {
		tops = tops[:25]
	}
	b.WriteString("## Key concepts\n\n")
	for _, s := range tops {
		fmt.Fprintf(&b, "- [[%s]] (%d)\n", s.label, s.deg)
	}

	// Cross-community edges.
	mset := map[string]bool{}
	for _, m := range members {
		mset[m] = true
	}
	type xe struct {
		target, relation string
		count            int
	}
	xmap := map[string]*xe{}
	for _, m := range members {
		for _, nb := range g.Neighbors(m) {
			if mset[nb] {
				continue
			}
			n := g.Node(nb)
			if n == nil {
				continue
			}
			key := n.Label
			if _, ok := xmap[key]; !ok {
				xmap[key] = &xe{target: n.Label}
			}
			xmap[key].count++
			for _, e := range g.EdgesBetween(m, nb) {
				xmap[key].relation = e.Relation
				break
			}
		}
	}
	if len(xmap) > 0 {
		b.WriteString("\n## Cross-community connections\n\n")
		xs := make([]*xe, 0, len(xmap))
		for _, v := range xmap {
			xs = append(xs, v)
		}
		sort.Slice(xs, func(i, j int) bool {
			if xs[i].count != xs[j].count {
				return xs[i].count > xs[j].count
			}
			return xs[i].target < xs[j].target
		})
		for _, v := range xs {
			fmt.Fprintf(&b, "- [[%s]] (%s ×%d)\n", v.target, v.relation, v.count)
		}
	}

	// Source files.
	fset := map[string]bool{}
	for _, m := range members {
		if n := g.Node(m); n != nil && n.SourceFile != "" {
			fset[n.SourceFile] = true
		}
	}
	if len(fset) > 0 {
		b.WriteString("\n## Source files\n\n")
		files := make([]string, 0, len(fset))
		for f := range fset {
			files = append(files, f)
		}
		sort.Strings(files)
		for _, f := range files {
			fmt.Fprintf(&b, "- `%s`\n", f)
		}
	}

	// Confidence breakdown.
	conf := map[core.Confidence]int{}
	total := 0
	for _, m := range members {
		for _, nb := range g.Neighbors(m) {
			for _, e := range g.EdgesBetween(m, nb) {
				conf[e.Confidence]++
				total++
			}
		}
	}
	if total > 0 {
		b.WriteString("\n## Confidence\n\n")
		for _, k := range []core.Confidence{core.ConfExtracted, core.ConfInferred, core.ConfAmbiguous} {
			fmt.Fprintf(&b, "- %s: %.0f%%\n", k, 100*float64(conf[k])/float64(total))
		}
	}
	return WikiFile{RelPath: fmt.Sprintf("wiki/_COMMUNITY_%d.md", id), Body: []byte(b.String())}
}
