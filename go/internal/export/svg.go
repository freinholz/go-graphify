package export

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"

	"github.com/safishamsi/graphify/go/internal/core"
)

// ToSVG renders the graph with a deterministic spring layout.
func ToSVG(g *core.Graph) []byte {
	pos := springLayout(g, 800, 600, 100, 42)
	var b bytes.Buffer
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 800 600" style="background:#111">`)
	// Edges first so nodes render on top.
	for _, e := range g.Edges() {
		a, ok1 := pos[e.Source]
		c, ok2 := pos[e.Target]
		if !ok1 || !ok2 {
			continue
		}
		fmt.Fprintf(&b, `<line x1="%f" y1="%f" x2="%f" y2="%f" stroke="#555" stroke-opacity="0.5"/>`,
			a.X, a.Y, c.X, c.Y)
	}
	for _, id := range sortedIDs(g) {
		p := pos[id]
		n := g.Node(id)
		r := 3 + float64(g.Degree(id))*0.5
		fmt.Fprintf(&b, `<circle cx="%f" cy="%f" r="%f" fill="%s"/>`, p.X, p.Y, r, colorFor(n.Community))
		fmt.Fprintf(&b, `<text x="%f" y="%f" font-size="9" fill="#ddd">%s</text>`,
			p.X+r+2, p.Y+3, xmlEscape(n.Label))
	}
	b.WriteString(`</svg>`)
	return b.Bytes()
}

type pt struct{ X, Y float64 }

// springLayout is a minimal Fruchterman–Reingold implementation.
func springLayout(g *core.Graph, w, h, iters int, seed int64) map[string]pt {
	r := rand.New(rand.NewSource(seed))
	pos := map[string]pt{}
	for _, n := range g.Nodes() {
		pos[n.ID] = pt{X: r.Float64() * float64(w), Y: r.Float64() * float64(h)}
	}
	area := float64(w * h)
	k := math.Sqrt(area / float64(max1(g.Order())))
	t := float64(w) / 10
	for it := 0; it < iters; it++ {
		disp := map[string]pt{}
		// Repulsive.
		ids := sortedIDsForLayout(g)
		for _, a := range ids {
			for _, b := range ids {
				if a == b {
					continue
				}
				dx := pos[a].X - pos[b].X
				dy := pos[a].Y - pos[b].Y
				d := math.Hypot(dx, dy) + 0.01
				f := k * k / d
				p := disp[a]
				p.X += dx / d * f
				p.Y += dy / d * f
				disp[a] = p
			}
		}
		// Attractive.
		for _, e := range g.Edges() {
			a, ok1 := pos[e.Source]
			b, ok2 := pos[e.Target]
			if !ok1 || !ok2 {
				continue
			}
			dx := a.X - b.X
			dy := a.Y - b.Y
			d := math.Hypot(dx, dy) + 0.01
			f := d * d / k
			pa := disp[e.Source]
			pa.X -= dx / d * f
			pa.Y -= dy / d * f
			disp[e.Source] = pa
			pb := disp[e.Target]
			pb.X += dx / d * f
			pb.Y += dy / d * f
			disp[e.Target] = pb
		}
		for id, d := range disp {
			p := pos[id]
			dmag := math.Hypot(d.X, d.Y) + 0.01
			p.X += d.X / dmag * math.Min(dmag, t)
			p.Y += d.Y / dmag * math.Min(dmag, t)
			p.X = math.Max(10, math.Min(float64(w)-10, p.X))
			p.Y = math.Max(10, math.Min(float64(h)-10, p.Y))
			pos[id] = p
		}
		t *= 0.95
	}
	return pos
}

func sortedIDsForLayout(g *core.Graph) []string {
	ids := make([]string, 0, g.Order())
	for _, n := range g.Nodes() {
		ids = append(ids, n.ID)
	}
	// deterministic order
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j-1] > ids[j]; j-- {
			ids[j-1], ids[j] = ids[j], ids[j-1]
		}
	}
	return ids
}

func max1(n int) int {
	if n == 0 {
		return 1
	}
	return n
}

func xmlEscape(s string) string {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		switch r {
		case '<':
			out = append(out, []byte("&lt;")...)
		case '>':
			out = append(out, []byte("&gt;")...)
		case '&':
			out = append(out, []byte("&amp;")...)
		case '"':
			out = append(out, []byte("&quot;")...)
		default:
			out = append(out, string(r)...)
		}
	}
	return string(out)
}
