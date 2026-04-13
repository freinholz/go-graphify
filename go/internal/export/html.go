package export

import (
	"bytes"
	"text/template"

	"github.com/safishamsi/graphify/go/internal/core"
)

// ToHTML renders an interactive vis-network page with the graph embedded.
func ToHTML(g *core.Graph) ([]byte, error) {
	data, err := ToJSON(g)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := htmlTemplate.Execute(&buf, map[string]any{
		"Nodes": g.Order(), "Edges": g.Size(), "Data": string(data),
	}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

var htmlTemplate = template.Must(template.New("graph").Parse(`<!doctype html>
<html><head>
<meta charset="utf-8"><title>graphify</title>
<script src="https://unpkg.com/vis-network/standalone/umd/vis-network.min.js"></script>
<style>
 body{margin:0;font-family:sans-serif;background:#111;color:#eee}
 #net{position:absolute;top:48px;left:0;right:280px;bottom:0}
 #side{position:absolute;top:48px;right:0;width:280px;bottom:0;padding:12px;overflow:auto;background:#181818;border-left:1px solid #333}
 header{padding:12px;background:#222;border-bottom:1px solid #333}
 input{width:calc(100% - 12px);padding:6px;background:#222;color:#eee;border:1px solid #333}
</style>
</head><body>
<header>graphify &middot; {{.Nodes}} nodes &middot; {{.Edges}} edges <input id="q" placeholder="search" /></header>
<div id="net"></div>
<div id="side"><h3>Selection</h3><div id="info">Click a node to inspect.</div></div>
<script>
const data = {{.Data}};
const nodes = new vis.DataSet(data.nodes);
const edges = new vis.DataSet(data.edges);
const net = new vis.Network(document.getElementById('net'), {nodes, edges}, {
  physics:{solver:'forceAtlas2Based',forceAtlas2Based:{gravitationalConstant:-30}},
  nodes:{shape:'dot',font:{color:'#eee'}},
  edges:{arrows:'to',color:{color:'#555'}}
});
net.on('selectNode', p => {
  const n = nodes.get(p.nodes[0]);
  const info = document.getElementById('info');
  info.innerHTML = '<b>'+n.label+'</b><br><small>'+n.source_file+':'+n.source_location+'</small><br>community '+n.community;
});
document.getElementById('q').addEventListener('input', e => {
  const q = e.target.value.toLowerCase();
  const upd = nodes.get().map(n => ({id:n.id, opacity: (!q || n.label.toLowerCase().includes(q))?1:0.15}));
  nodes.update(upd);
});
</script>
</body></html>`))
