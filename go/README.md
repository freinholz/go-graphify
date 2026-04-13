# graphify (Go)

Local-only Go port of [graphify](https://github.com/safishamsi/graphify).
Turns a folder of code/docs into a queryable knowledge graph and writes all
artifacts under `graphify-out/`.

## Status

v0.1 — stdlib-only, zero external dependencies, static binary.

| Pipeline stage   | Status | Notes                                                              |
|------------------|:------:|--------------------------------------------------------------------|
| detect           |  ✅    | `.graphifyignore`, hidden-dir skip, sensitive-file exclusion       |
| extract          |  ✅    | Regex-driven, 18 languages; pluggable `Extractor` interface        |
| build            |  ✅    | Dedupes parallel edges into weighted edges                         |
| cluster          |  ✅    | Single-level Louvain, deterministic                                |
| analyze          |  ✅    | God nodes, cross-community surprises, suggested questions          |
| report           |  ✅    | `GRAPH_REPORT.md` with wikilinks                                   |
| export           |  ✅    | JSON, vis.js HTML, SVG, Neo4j Cypher, Obsidian Canvas, wiki vault  |
| cache            |  ✅    | SHA256 per-file, markdown frontmatter-aware                        |
| watch            |  ✅    | Poll-based (3s default), stdlib only                               |
| ingest           |  ✅    | URL → markdown with SSRF guard                                     |
| benchmark        |  ✅    | Corpus-vs-BFS-slice token estimate                                 |
| query            |  ✅    | get_node, neighbors, community, god-nodes, path, BFS question     |

## Install

```
go install github.com/safishamsi/graphify/go/cmd/graphify@latest
```

## Use

```
graphify run -root . -out graphify-out -export json,html,svg,wiki
graphify stats -out graphify-out
graphify query -out graphify-out "how does authentication work"
graphify neighbors -out graphify-out HTTPTransport
graphify path -from Client -to HTTPTransport -out graphify-out
graphify watch -root . -out graphify-out
```

## Architecture

Every stage is a small, dependency-free package under `internal/`. The
`Store` interface in `internal/store` is the hosted-ready boundary: the CLI
uses `FSStore` writing to disk today; a future hosted client/server will
implement the same interface backed by HTTP, so the pipeline code does not
need to change.

```
detect → extract → build → cluster → analyze → report → export
                                    │
                                    └─> pipeline.GraphQuery (MCP-equivalent ops)
```

| Package               | Role                                                                      |
|-----------------------|---------------------------------------------------------------------------|
| `internal/core`       | `Node`, `Edge`, `Extraction`, `Graph` (adjacency-list, BFS, shortest path) |
| `internal/store`      | `Store` interface + `FSStore` (graphify-out/ on disk)                     |
| `internal/detect`     | File walker, `.graphifyignore`, sensitive-file skip                       |
| `internal/extract`    | Language-pluggable extractors (Python/Go/JS/TS + 15 others via `GenericExtractor`) |
| `internal/build`      | Extraction merge + parallel-edge collapse                                 |
| `internal/cluster`    | Louvain community detection                                               |
| `internal/analyze`    | God nodes, surprising connections, suggested questions                    |
| `internal/report`     | `GRAPH_REPORT.md` renderer                                                |
| `internal/export`     | JSON, HTML (vis.js), SVG, Neo4j Cypher, Canvas, wiki vault                |
| `internal/cache`      | SHA256 per-file extraction cache                                          |
| `internal/validate`   | Extraction schema validation                                              |
| `internal/security`   | URL, IP, path, label validators                                           |
| `internal/ingest`     | URL fetch → markdown                                                      |
| `internal/watch`      | Polling file watcher                                                      |
| `internal/benchmark`  | Token-reduction metric                                                    |
| `internal/pipeline`   | Stage orchestrator + `GraphQuery` (transport-neutral)                     |
| `cmd/graphify`        | Local CLI driver                                                          |

## Extension points for a future hosted version

Three well-defined seams let the same engine back a hosted deployment without
refactoring:

1. **`store.Store`** — swap `FSStore` for an HTTP-backed implementation to
   push graphs and artifacts to a server.
2. **`extract.Extractor`** — register additional extractors (tree-sitter,
   LLM-based semantic) alongside the regex v1 ones. `Register()` is public.
3. **`pipeline.GraphQuery`** — the exact set of operations the Python MCP
   server exposed. A future HTTP/gRPC façade wraps this struct one-to-one.

## Testing

```
cd go && go test ./...
```
