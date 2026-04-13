// Package cache computes content-addressed keys for files and mediates
// per-file extraction caching via a Store.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"regexp"
	"strings"

	"github.com/safishamsi/graphify/go/internal/core"
	"github.com/safishamsi/graphify/go/internal/store"
)

// KeyForFile returns a stable cache key derived from the file path + body.
// For markdown files the YAML frontmatter is stripped from the body first so
// that pure metadata edits don't bust the cache (matches Python behavior).
func KeyForFile(path string) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if strings.HasSuffix(strings.ToLower(path), ".md") {
		body = stripFrontmatter(body)
	}
	h := sha256.New()
	h.Write([]byte(path))
	h.Write([]byte{0})
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil)), nil
}

var frontmatterRe = regexp.MustCompile(`(?s)\A---\s*\n.*?\n---\s*\n`)

func stripFrontmatter(b []byte) []byte {
	return frontmatterRe.ReplaceAll(b, nil)
}

// Get returns a cached extraction for the file if the content hash matches.
func Get(s store.Store, path string) (*core.Extraction, bool, error) {
	key, err := KeyForFile(path)
	if err != nil {
		return nil, false, err
	}
	return s.CacheGet(key)
}

// Put stores an extraction under the file's current content hash.
func Put(s store.Store, path string, ex *core.Extraction) error {
	key, err := KeyForFile(path)
	if err != nil {
		return err
	}
	return s.CachePut(key, ex)
}
