// Package pipeline wires every stage (detect → extract → build → cluster →
// analyze → report → export) into one call. Both the CLI and any future
// server/front-end for the hosted version use this as their entry point,
// keeping engine behaviour identical across drivers.
package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/safishamsi/graphify/go/internal/analyze"
	"github.com/safishamsi/graphify/go/internal/build"
	"github.com/safishamsi/graphify/go/internal/cache"
	"github.com/safishamsi/graphify/go/internal/cluster"
	"github.com/safishamsi/graphify/go/internal/core"
	"github.com/safishamsi/graphify/go/internal/detect"
	"github.com/safishamsi/graphify/go/internal/export"
	"github.com/safishamsi/graphify/go/internal/extract"
	"github.com/safishamsi/graphify/go/internal/report"
	"github.com/safishamsi/graphify/go/internal/store"
	"github.com/safishamsi/graphify/go/internal/validate"
)

// Options controls a single pipeline run.
type Options struct {
	Root         string
	Store        store.Store
	UseCache     bool
	WantExports  []string // any of "json","html","svg","cypher","canvas","wiki"
	Logger       *log.Logger
}

// Result summarises what the run produced.
type Result struct {
	Files        int            `json:"files"`
	Extractions  int            `json:"extractions"`
	Graph        *core.Graph    `json:"-"`
	Communities  cluster.Result `json:"-"`
	Analysis     analyze.Result `json:"analysis"`
	Artifacts    []string       `json:"artifacts"`
}

// Run executes the pipeline end-to-end.
func Run(ctx context.Context, opts Options) (*Result, error) {
	if opts.Logger == nil {
		opts.Logger = log.New(io.Discard, "", 0)
	}
	// 1. Detect.
	opts.Logger.Printf("detect: scanning %s", opts.Root)
	det, err := detect.Collect(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("detect: %w", err)
	}
	opts.Logger.Printf("detect: %d files", det.TotalDocs)

	// 2. Extract (with cache).
	var exs []core.Extraction
	for _, f := range det.Files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		if opts.UseCache {
			if ex, ok, err := cache.Get(opts.Store, f.Path); err == nil && ok {
				exs = append(exs, *ex)
				continue
			}
		}
		ex, err := extract.Extract(f.Path)
		if err != nil {
			opts.Logger.Printf("extract: skip %s: %v", f.Path, err)
			continue
		}
		if errs := validate.Extraction(&ex); len(errs) > 0 {
			opts.Logger.Printf("validate: %s: %d issues", f.Path, len(errs))
		}
		if opts.UseCache {
			_ = cache.Put(opts.Store, f.Path, &ex)
		}
		exs = append(exs, ex)
	}
	opts.Logger.Printf("extract: %d extractions", len(exs))

	// 3. Build.
	g := build.Build(exs)
	opts.Logger.Printf("build: %d nodes, %d edges", g.Order(), g.Size())

	// 4. Cluster.
	comms := cluster.Cluster(g)
	opts.Logger.Printf("cluster: %d communities", len(comms))

	// 5. Analyze.
	an := analyze.Analyze(g, comms)

	// 6. Persist graph.
	if err := opts.Store.SaveGraph(g); err != nil {
		return nil, fmt.Errorf("save graph: %w", err)
	}

	// 7. Report.
	rep := report.Render(g, an, comms)
	if err := opts.Store.WriteArtifact("GRAPH_REPORT.md", bytes.NewReader([]byte(rep))); err != nil {
		return nil, fmt.Errorf("write report: %w", err)
	}

	artifacts := []string{"graph.json", "GRAPH_REPORT.md"}
	// 8. Export extras.
	for _, want := range opts.WantExports {
		rel, data, err := renderExport(want, g, comms)
		if err != nil {
			opts.Logger.Printf("export %s: %v", want, err)
			continue
		}
		if rel == "wiki" {
			for _, f := range export.ToWiki(g, comms) {
				if err := opts.Store.WriteArtifact(f.RelPath, bytes.NewReader(f.Body)); err == nil {
					artifacts = append(artifacts, f.RelPath)
				}
			}
			continue
		}
		if err := opts.Store.WriteArtifact(rel, bytes.NewReader(data)); err != nil {
			opts.Logger.Printf("write %s: %v", rel, err)
			continue
		}
		artifacts = append(artifacts, rel)
	}

	// 9. Emit run summary JSON.
	summary := map[string]any{
		"files": det.TotalDocs, "extractions": len(exs),
		"nodes": g.Order(), "edges": g.Size(),
		"communities": len(comms),
	}
	sb, _ := json.MarshalIndent(summary, "", "  ")
	_ = opts.Store.WriteArtifact("summary.json", bytes.NewReader(sb))

	return &Result{
		Files: det.TotalDocs, Extractions: len(exs),
		Graph: g, Communities: comms, Analysis: an,
		Artifacts: artifacts,
	}, nil
}

func renderExport(kind string, g *core.Graph, comms cluster.Result) (string, []byte, error) {
	if kind == "wiki" {
		return "wiki", nil, nil
	}
	return RenderExport(kind, g)
}

// RenderExport is the public counterpart used by the CLI's `export` command
// to produce a single (non-wiki) export artifact from a loaded graph.
func RenderExport(kind string, g *core.Graph) (string, []byte, error) {
	switch kind {
	case "json":
		b, err := export.ToJSON(g)
		return "graph.viz.json", b, err
	case "html":
		b, err := export.ToHTML(g)
		return "graph.html", b, err
	case "svg":
		return "graph.svg", export.ToSVG(g), nil
	case "cypher":
		return "graph.cypher", export.ToCypher(g), nil
	case "canvas":
		return "graph.canvas", export.ToCanvas(g), nil
	}
	return "", nil, fmt.Errorf("unknown export %q", kind)
}

// ExportWiki is the public counterpart used by the CLI for wiki export.
func ExportWiki(g *core.Graph, comms map[int][]string) []export.WikiFile {
	return export.ToWiki(g, cluster.Result(comms))
}

// ResolveRoot expands a user-supplied path to absolute, defaulting to CWD.
func ResolveRoot(p string) (string, error) {
	if p == "" {
		p = "."
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(abs); err != nil {
		return "", err
	}
	return abs, nil
}
