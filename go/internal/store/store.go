// Package store defines the persistence abstraction that sits between the
// graphify engine and whatever medium is used to hold graphs, caches and
// reports.
//
// The CLI uses a filesystem-backed Store writing under graphify-out/. A future
// hosted version will provide an HTTP-backed Store that pushes artifacts to a
// backend server, and the engine code does not need to change.
package store

import (
	"io"

	"github.com/safishamsi/graphify/go/internal/core"
)

// Store is the artifact persistence interface used by the pipeline.
//
// Implementations must be safe for use by a single pipeline run; concurrency
// across runs is not required.
type Store interface {
	// Graph read/write.
	LoadGraph() (*core.Graph, error)
	SaveGraph(g *core.Graph) error

	// Per-file extraction cache keyed by SHA256 of file content.
	CacheGet(key string) (*core.Extraction, bool, error)
	CachePut(key string, ex *core.Extraction) error
	CacheClear() error

	// Raw artifact writer (reports, exports, vault files, etc).
	// relPath is relative to the store's root and forward-slash separated.
	WriteArtifact(relPath string, r io.Reader) error
	ReadArtifact(relPath string) ([]byte, error)
	ArtifactExists(relPath string) bool

	// Memory: Q&A feedback loop entries.
	AppendMemory(entry []byte) error

	// Root identifies the storage location for logging / CLI output.
	Root() string
}
