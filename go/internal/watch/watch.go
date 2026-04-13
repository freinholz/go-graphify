// Package watch polls a directory tree and invokes a callback when any
// file's mtime changes. Using polling avoids platform-specific fsnotify
// dependencies; the trade-off is a small steady load, acceptable for
// interactive development. A fsnotify-backed implementation can replace
// this without changing the public API.
package watch

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Watch invokes onChange whenever any file under root is added, modified or
// removed. It returns when ctx is cancelled.
func Watch(ctx context.Context, root string, interval time.Duration, onChange func()) error {
	if interval <= 0 {
		interval = 3 * time.Second
	}
	prev, err := snapshot(root)
	if err != nil {
		return err
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			cur, err := snapshot(root)
			if err != nil {
				continue
			}
			if diff(prev, cur) {
				onChange()
			}
			prev = cur
		}
	}
}

func snapshot(root string) (map[string]time.Time, error) {
	out := map[string]time.Time{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == "graphify-out" || name == ".git" || name == "node_modules" {
				return fs.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		out[p] = info.ModTime()
		return nil
	})
	return out, err
}

func diff(a, b map[string]time.Time) bool {
	if len(a) != len(b) {
		return true
	}
	for k, v := range a {
		if !b[k].Equal(v) {
			return true
		}
	}
	return false
}
