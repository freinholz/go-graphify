// Package extract turns source files into structural (Node, Edge) tuples.
//
// The framework exposes an Extractor interface plus a registry indexed by file
// extension. The v1 built-in extractors are regex-driven and language-aware;
// they match the Python tree-sitter extractors' schema so the rest of the
// pipeline (build / cluster / analyze / export) stays identical and a
// tree-sitter-backed Extractor can drop in later without downstream changes.
package extract

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/safishamsi/graphify/go/internal/core"
)

// Extractor turns a single file into Nodes + Edges.
type Extractor interface {
	Extensions() []string
	Extract(path string, src []byte) (core.Extraction, error)
}

var registry = map[string]Extractor{}

// Register binds each of e.Extensions() to e in the global registry.
func Register(e Extractor) {
	for _, ext := range e.Extensions() {
		registry[strings.ToLower(ext)] = e
	}
}

// Extract dispatches on the path's extension. If no registered extractor
// matches, a generic keyword extractor is used for documents and a no-op
// extraction is returned for unknown types.
func Extract(path string) (core.Extraction, error) {
	ext := strings.ToLower(filepath.Ext(path))
	src, err := os.ReadFile(path)
	if err != nil {
		return core.Extraction{}, err
	}
	if ex, ok := registry[ext]; ok {
		return ex.Extract(path, src)
	}
	switch ext {
	case ".md", ".txt", ".rst":
		return extractDocument(path, src)
	}
	return core.Extraction{}, nil
}

// fileID returns a short deterministic id derived from the path. Used as a
// prefix so that same-named symbols in different files remain distinct.
func fileID(path string) string {
	h := sha1.Sum([]byte(path))
	return hex.EncodeToString(h[:4])
}

// makeID builds a node id by joining a file id with a symbol name.
func makeID(fid, sym string) string { return fid + "_" + slug(sym) }

// slug keeps identifier-safe characters from sym.
func slug(sym string) string {
	var b strings.Builder
	for _, r := range sym {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		case r == '.', r == '/', r == ':', r == '-':
			b.WriteByte('_')
		}
	}
	return b.String()
}

// lineOf returns the 1-indexed line that byteOffset falls on.
func lineOf(src []byte, byteOffset int) int {
	if byteOffset > len(src) {
		byteOffset = len(src)
	}
	line := 1
	for i := 0; i < byteOffset; i++ {
		if src[i] == '\n' {
			line++
		}
	}
	return line
}

// locStr formats a source location string.
func locStr(line int) string {
	if line <= 0 {
		return ""
	}
	return fmt.Sprintf("L%d", line)
}

// fileNode creates a file-level anchor node used as the `contains` parent.
func fileNode(path string, kind core.FileType) core.Node {
	return core.Node{
		ID:         fileID(path),
		Label:      filepath.Base(path),
		FileType:   kind,
		SourceFile: path,
	}
}

// dedupeNodes removes duplicate ids in-place (first occurrence wins).
func dedupeNodes(ns []core.Node) []core.Node {
	seen := map[string]bool{}
	out := ns[:0]
	for _, n := range ns {
		if seen[n.ID] {
			continue
		}
		seen[n.ID] = true
		out = append(out, n)
	}
	return out
}

// ErrNoExtractor is returned by Extract when no registered extractor matches.
var ErrNoExtractor = errors.New("no extractor for extension")

// compile is a convenience for package-level regex singletons.
func compile(expr string) *regexp.Regexp { return regexp.MustCompile(expr) }
