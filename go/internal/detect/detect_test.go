package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollect(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.py", "b.go", "c.md", "d.txt", "noisy.min.js"} {
		os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644)
	}
	// Hidden and excluded.
	os.Mkdir(filepath.Join(root, ".git"), 0o755)
	os.WriteFile(filepath.Join(root, ".git", "ignored.py"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(root, ".env"), []byte("SECRET=1"), 0o644)

	r, err := Collect(root)
	if err != nil {
		t.Fatal(err)
	}
	if r.TotalDocs == 0 {
		t.Fatal("expected files")
	}
	// .env must not appear.
	for _, f := range r.Files {
		if filepath.Base(f.Path) == ".env" {
			t.Fatal(".env should be skipped")
		}
		if f.Path == filepath.Join(root, ".git", "ignored.py") {
			t.Fatal(".git should be skipped")
		}
	}
}

func TestGraphifyIgnore(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "keep.py"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(root, "skip.py"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(root, ".graphifyignore"), []byte("skip.py\n"), 0o644)
	r, _ := Collect(root)
	for _, f := range r.Files {
		if filepath.Base(f.Path) == "skip.py" {
			t.Fatal("skip.py should be ignored")
		}
	}
}
