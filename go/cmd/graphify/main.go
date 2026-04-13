// graphify — local-only CLI for the Go port.
//
// Subcommands mirror the Python CLI where the functionality is local. Hosted
// concerns (skill installers, MCP stdio) are intentionally omitted — they
// belong in the future hosted client/server.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/safishamsi/graphify/go/internal/benchmark"
	"github.com/safishamsi/graphify/go/internal/core"
	"github.com/safishamsi/graphify/go/internal/extract"
	"github.com/safishamsi/graphify/go/internal/ingest"
	"github.com/safishamsi/graphify/go/internal/pipeline"
	"github.com/safishamsi/graphify/go/internal/store"
	"github.com/safishamsi/graphify/go/internal/watch"
)

type godNode struct {
	Label string `json:"label"`
	Edges int    `json:"edges"`
}

func analyzeGodNodes(g *core.Graph, top int) []godNode {
	var gns []godNode
	for _, n := range g.Nodes() {
		gns = append(gns, godNode{Label: n.Label, Edges: g.Degree(n.ID)})
	}
	sort.Slice(gns, func(i, j int) bool {
		if gns[i].Edges != gns[j].Edges {
			return gns[i].Edges > gns[j].Edges
		}
		return gns[i].Label < gns[j].Label
	})
	if top > len(gns) {
		top = len(gns)
	}
	return gns[:top]
}

// Version is the binary version, overridable with -ldflags.
var Version = "0.1.0-dev"

func main() {
	extract.RegisterBuiltins()
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	var err error
	switch cmd {
	case "run", "build":
		err = runPipeline(args)
	case "query":
		err = runQuery(args)
	case "neighbors":
		err = runNeighbors(args)
	case "community":
		err = runCommunity(args)
	case "god-nodes":
		err = runGodNodes(args)
	case "path":
		err = runPath(args)
	case "stats":
		err = runStats(args)
	case "export":
		err = runExport(args)
	case "watch":
		err = runWatch(args)
	case "ingest":
		err = runIngest(args)
	case "benchmark":
		err = runBenchmark(args)
	case "version", "-v", "--version":
		fmt.Println(Version)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `graphify — knowledge graphs for local codebases

USAGE
  graphify <command> [flags]

COMMANDS
  run         Run the full pipeline (detect → extract → build → cluster → analyze → export)
  query       Ask a question and get relevant nodes via BFS from keyword seeds
  neighbors   List direct neighbors of a node
  community   List nodes in a community
  god-nodes   List the top-N most connected nodes
  path        Find shortest path between two labels
  stats       Print graph statistics
  export      Emit an additional export format from an existing graph.json
  watch       Re-run the pipeline on file changes
  ingest      Fetch a URL and save it as markdown under the corpus
  benchmark   Measure token-reduction vs naive corpus context
  version     Print the version
`)
}

func pipelineStore(outDir string) (store.Store, error) {
	if outDir == "" {
		outDir = "graphify-out"
	}
	return store.NewFS(outDir)
}

func runPipeline(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	var (
		root     = fs.String("root", ".", "directory to scan")
		out      = fs.String("out", "graphify-out", "store/output directory")
		exports  = fs.String("export", "json,html,svg", "comma-separated exports: json,html,svg,cypher,canvas,wiki")
		noCache  = fs.Bool("no-cache", false, "disable per-file extraction cache")
		verbose  = fs.Bool("v", false, "verbose logging")
	)
	_ = fs.Parse(args)
	abs, err := pipeline.ResolveRoot(*root)
	if err != nil {
		return err
	}
	st, err := pipelineStore(*out)
	if err != nil {
		return err
	}
	logger := log.New(io.Discard, "", 0)
	if *verbose {
		logger = log.New(os.Stderr, "graphify: ", log.LstdFlags)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	res, err := pipeline.Run(ctx, pipeline.Options{
		Root: abs, Store: st, UseCache: !*noCache,
		WantExports: splitCSV(*exports), Logger: logger,
	})
	if err != nil {
		return err
	}
	fmt.Printf("done: %d files, %d nodes, %d edges, %d communities\n",
		res.Files, res.Graph.Order(), res.Graph.Size(), len(res.Communities))
	fmt.Printf("artifacts in %s:\n", st.Root())
	for _, a := range res.Artifacts {
		fmt.Println("  -", filepath.Join(st.Root(), a))
	}
	return nil
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func loadQuery(out string) (pipeline.GraphQuery, error) {
	st, err := pipelineStore(out)
	if err != nil {
		return pipeline.GraphQuery{}, err
	}
	g, err := st.LoadGraph()
	if err != nil {
		return pipeline.GraphQuery{}, fmt.Errorf("load graph from %s: %w", st.Root(), err)
	}
	return pipeline.GraphQuery{G: g}, nil
}

func runQuery(args []string) error {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	out := fs.String("out", "graphify-out", "")
	depth := fs.Int("depth", 2, "BFS depth")
	budget := fs.Int("budget", 50, "max nodes to return")
	_ = fs.Parse(args)
	q, err := loadQuery(*out)
	if err != nil {
		return err
	}
	question := strings.Join(fs.Args(), " ")
	if question == "" {
		return fmt.Errorf("provide a question")
	}
	nodes := q.Question(question, *depth, *budget)
	return writeJSON(nodes)
}

func runNeighbors(args []string) error {
	fs := flag.NewFlagSet("neighbors", flag.ExitOnError)
	out := fs.String("out", "graphify-out", "")
	rel := fs.String("rel", "", "filter by relation")
	_ = fs.Parse(args)
	if fs.NArg() < 1 {
		return fmt.Errorf("provide a label")
	}
	q, err := loadQuery(*out)
	if err != nil {
		return err
	}
	return writeJSON(q.Neighbors(fs.Arg(0), *rel))
}

func runCommunity(args []string) error {
	fs := flag.NewFlagSet("community", flag.ExitOnError)
	out := fs.String("out", "graphify-out", "")
	id := fs.Int("id", 0, "community id")
	_ = fs.Parse(args)
	q, err := loadQuery(*out)
	if err != nil {
		return err
	}
	return writeJSON(q.Community(*id))
}

func runGodNodes(args []string) error {
	fs := flag.NewFlagSet("god-nodes", flag.ExitOnError)
	out := fs.String("out", "graphify-out", "")
	top := fs.Int("top", 10, "top-N")
	_ = fs.Parse(args)
	q, err := loadQuery(*out)
	if err != nil {
		return err
	}
	return writeJSON(analyzeGodNodes(q.G, *top))
}

func runPath(args []string) error {
	fs := flag.NewFlagSet("path", flag.ExitOnError)
	out := fs.String("out", "graphify-out", "")
	from := fs.String("from", "", "")
	to := fs.String("to", "", "")
	maxHops := fs.Int("max-hops", 6, "")
	_ = fs.Parse(args)
	if *from == "" || *to == "" {
		return fmt.Errorf("-from and -to required")
	}
	q, err := loadQuery(*out)
	if err != nil {
		return err
	}
	return writeJSON(q.ShortestPath(*from, *to, *maxHops))
}

func runStats(args []string) error {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	out := fs.String("out", "graphify-out", "")
	_ = fs.Parse(args)
	q, err := loadQuery(*out)
	if err != nil {
		return err
	}
	g := q.G
	conf := map[string]int{}
	for _, e := range g.Edges() {
		conf[string(e.Confidence)]++
	}
	return writeJSON(map[string]any{
		"nodes": g.Order(), "edges": g.Size(), "confidence": conf,
	})
}

func runExport(args []string) error {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	out := fs.String("out", "graphify-out", "")
	kind := fs.String("kind", "html", "json|html|svg|cypher|canvas|wiki")
	_ = fs.Parse(args)
	st, err := pipelineStore(*out)
	if err != nil {
		return err
	}
	g, err := st.LoadGraph()
	if err != nil {
		return err
	}
	// Reconstruct communities from node attribute.
	comms := map[int][]string{}
	for _, n := range g.Nodes() {
		comms[n.Community] = append(comms[n.Community], n.ID)
	}
	// The pipeline.renderExport helper is unexported, so replicate briefly.
	switch *kind {
	case "wiki":
		for _, f := range pipeline.ExportWiki(g, comms) {
			_ = st.WriteArtifact(f.RelPath, strings.NewReader(string(f.Body)))
			fmt.Println(filepath.Join(st.Root(), f.RelPath))
		}
		return nil
	default:
		rel, data, err := pipeline.RenderExport(*kind, g)
		if err != nil {
			return err
		}
		if err := st.WriteArtifact(rel, strings.NewReader(string(data))); err != nil {
			return err
		}
		fmt.Println(filepath.Join(st.Root(), rel))
		return nil
	}
}

func runWatch(args []string) error {
	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	root := fs.String("root", ".", "")
	out := fs.String("out", "graphify-out", "")
	interval := fs.Duration("interval", 3*time.Second, "poll interval")
	_ = fs.Parse(args)
	abs, err := pipeline.ResolveRoot(*root)
	if err != nil {
		return err
	}
	st, err := pipelineStore(*out)
	if err != nil {
		return err
	}
	logger := log.New(os.Stderr, "graphify: ", log.LstdFlags)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	rerun := func() {
		logger.Println("change detected — rebuilding")
		if _, err := pipeline.Run(ctx, pipeline.Options{
			Root: abs, Store: st, UseCache: true,
			WantExports: []string{"json", "html"}, Logger: logger,
		}); err != nil {
			logger.Printf("rebuild failed: %v", err)
		}
	}
	rerun()
	return watch.Watch(ctx, abs, *interval, rerun)
}

func runIngest(args []string) error {
	fs := flag.NewFlagSet("ingest", flag.ExitOnError)
	url := fs.String("url", "", "URL to fetch")
	dir := fs.String("dir", "corpus", "target directory")
	_ = fs.Parse(args)
	if *url == "" {
		return fmt.Errorf("-url required")
	}
	p, err := ingest.Ingest(*url, ingest.Options{OutputDir: *dir})
	if err != nil {
		return err
	}
	fmt.Println(p)
	return nil
}

func runBenchmark(args []string) error {
	fs := flag.NewFlagSet("benchmark", flag.ExitOnError)
	root := fs.String("root", ".", "")
	out := fs.String("out", "graphify-out", "")
	_ = fs.Parse(args)
	abs, err := pipeline.ResolveRoot(*root)
	if err != nil {
		return err
	}
	st, err := pipelineStore(*out)
	if err != nil {
		return err
	}
	g, err := st.LoadGraph()
	if err != nil {
		return err
	}
	r, err := benchmark.Run(abs, g)
	if err != nil {
		return err
	}
	return writeJSON(r)
}

func writeJSON(v any) error {
	return json.NewEncoder(os.Stdout).Encode(v)
}
