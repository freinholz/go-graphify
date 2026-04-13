package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/safishamsi/graphify/go/internal/core"
)

// FSStore is a local-filesystem implementation of Store, rooted at some
// directory (conventionally "graphify-out/").
type FSStore struct{ root string }

// NewFS creates an FSStore, creating the root directory if needed.
func NewFS(root string) (*FSStore, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &FSStore{root: abs}, nil
}

func (s *FSStore) Root() string { return s.root }

func (s *FSStore) resolve(rel string) (string, error) {
	// Normalize rel-path separators, reject absolute or parent-escaping paths.
	rel = filepath.FromSlash(strings.TrimPrefix(rel, "/"))
	full := filepath.Clean(filepath.Join(s.root, rel))
	if !strings.HasPrefix(full, s.root+string(filepath.Separator)) && full != s.root {
		return "", errors.New("store: path escapes root")
	}
	return full, nil
}

func (s *FSStore) LoadGraph() (*core.Graph, error) {
	p, err := s.resolve("graph.json")
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	return core.FromJSON(b)
}

func (s *FSStore) SaveGraph(g *core.Graph) error {
	b, err := g.ToJSON()
	if err != nil {
		return err
	}
	return s.WriteArtifact("graph.json", bytes.NewReader(b))
}

func (s *FSStore) CacheGet(key string) (*core.Extraction, bool, error) {
	p, err := s.resolve(filepath.Join("cache", key+".json"))
	if err != nil {
		return nil, false, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var ex core.Extraction
	if err := json.Unmarshal(b, &ex); err != nil {
		return nil, false, err
	}
	return &ex, true, nil
}

func (s *FSStore) CachePut(key string, ex *core.Extraction) error {
	b, err := json.MarshalIndent(ex, "", "  ")
	if err != nil {
		return err
	}
	return s.WriteArtifact(filepath.Join("cache", key+".json"), bytes.NewReader(b))
}

func (s *FSStore) CacheClear() error {
	p := filepath.Join(s.root, "cache")
	return os.RemoveAll(p)
}

func (s *FSStore) WriteArtifact(rel string, r io.Reader) error {
	full, err := s.resolve(rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	f, err := os.Create(full)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func (s *FSStore) ReadArtifact(rel string) ([]byte, error) {
	full, err := s.resolve(rel)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(full)
}

func (s *FSStore) ArtifactExists(rel string) bool {
	full, err := s.resolve(rel)
	if err != nil {
		return false
	}
	_, err = os.Stat(full)
	return err == nil
}

func (s *FSStore) AppendMemory(entry []byte) error {
	// Each entry lands in its own file (same approach as Python memory/{ts}.json).
	ts := timestamp()
	return s.WriteArtifact(filepath.Join("memory", ts+".json"), bytes.NewReader(entry))
}
