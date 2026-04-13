// Package detect walks a directory tree and returns files grouped by type,
// honouring .graphifyignore patterns, hidden-directory skipping and
// sensitive-file exclusion.
package detect

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// FileType groups extensions into the five categories recognised by graphify.
type FileType string

const (
	Code     FileType = "code"
	Document FileType = "document"
	Paper    FileType = "paper"
	Image    FileType = "image"
	Video    FileType = "video"
	Office   FileType = "office"
)

// Extensions recognised by graphify. Keep in sync with the Python detect.py.
var (
	CodeExts = map[string]bool{
		".py": true, ".ts": true, ".js": true, ".jsx": true, ".tsx": true,
		".go": true, ".rs": true, ".java": true, ".cpp": true, ".cc": true,
		".cxx": true, ".c": true, ".h": true, ".hpp": true, ".rb": true,
		".swift": true, ".kt": true, ".kts": true, ".cs": true, ".scala": true,
		".php": true, ".lua": true, ".zig": true, ".ps1": true, ".ex": true,
		".exs": true, ".m": true, ".mm": true, ".jl": true, ".vue": true,
		".svelte": true,
	}
	DocExts    = map[string]bool{".md": true, ".txt": true, ".rst": true}
	PaperExts  = map[string]bool{".pdf": true}
	ImageExts  = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".svg": true}
	VideoExts  = map[string]bool{".mp4": true, ".mov": true, ".webm": true, ".mkv": true, ".avi": true, ".m4v": true, ".mp3": true, ".wav": true, ".m4a": true, ".ogg": true}
	OfficeExts = map[string]bool{".docx": true, ".xlsx": true}

	// Sensitive file patterns we always skip.
	sensitiveNames = []string{
		".env", "credentials.json", "secrets.yaml", ".aws/credentials",
		".gcloud/credentials", "private_key.pem", "id_rsa",
	}
	sensitiveSuffixes = []string{".key", ".pem"}

	// Directories we always skip when not configured otherwise.
	defaultIgnoreDirs = []string{
		"node_modules", "__pycache__", ".git", ".svn", ".hg",
		"venv", ".venv", "env", ".tox", ".mypy_cache", ".pytest_cache",
		"dist", "build", "target", "out", "graphify-out", ".idea", ".vscode",
	}
)

// Result is the output of Collect.
type Result struct {
	Files     []Entry
	Counts    map[FileType]int
	TotalDocs int
	WordCount int
}

// Entry is a single discovered file.
type Entry struct {
	Path string
	Ext  string
	Kind FileType
}

// Collect walks root and returns matching files, respecting .graphifyignore.
func Collect(root string) (*Result, error) {
	patterns := loadGraphifyIgnore(root)
	res := &Result{Counts: map[FileType]int{}}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if strings.HasPrefix(name, ".") && name != "." {
				return filepath.SkipDir
			}
			for _, ig := range defaultIgnoreDirs {
				if name == ig {
					return filepath.SkipDir
				}
			}
			if matchesAny(rel+"/", patterns) {
				return filepath.SkipDir
			}
			return nil
		}
		if isSensitive(name, rel) {
			return nil
		}
		if matchesAny(rel, patterns) {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(name))
		kind, ok := classify(ext)
		if !ok {
			return nil
		}
		e := Entry{Path: p, Ext: ext, Kind: kind}
		res.Files = append(res.Files, e)
		res.Counts[kind]++
		res.TotalDocs++
		if kind == Code || kind == Document {
			if b, err := os.ReadFile(p); err == nil {
				res.WordCount += wordCount(b)
			}
		}
		return nil
	})
	return res, err
}

func classify(ext string) (FileType, bool) {
	switch {
	case CodeExts[ext]:
		return Code, true
	case DocExts[ext]:
		return Document, true
	case PaperExts[ext]:
		return Paper, true
	case ImageExts[ext]:
		return Image, true
	case VideoExts[ext]:
		return Video, true
	case OfficeExts[ext]:
		return Office, true
	}
	return "", false
}

func isSensitive(name, rel string) bool {
	for _, sn := range sensitiveNames {
		if name == sn || strings.HasSuffix(rel, "/"+sn) {
			return true
		}
	}
	for _, sx := range sensitiveSuffixes {
		if strings.HasSuffix(strings.ToLower(name), sx) {
			return true
		}
	}
	return false
}

func loadGraphifyIgnore(root string) []string {
	f, err := os.Open(filepath.Join(root, ".graphifyignore"))
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

func matchesAny(rel string, patterns []string) bool {
	for _, p := range patterns {
		if match(p, rel) {
			return true
		}
	}
	return false
}

// match implements gitignore-like globbing (simplified: *, **, trailing /).
func match(pattern, name string) bool {
	if strings.HasSuffix(pattern, "/") {
		// Directory match.
		pattern = strings.TrimSuffix(pattern, "/")
		return strings.HasPrefix(name, pattern+"/") || name == pattern
	}
	ok, err := filepath.Match(pattern, name)
	if err == nil && ok {
		return true
	}
	base := filepath.Base(name)
	ok, _ = filepath.Match(pattern, base)
	return ok
}

// wordCount counts whitespace-separated tokens.
func wordCount(b []byte) int {
	n, inWord := 0, false
	for _, c := range b {
		if c == ' ' || c == '\n' || c == '\t' || c == '\r' {
			if inWord {
				n++
				inWord = false
			}
		} else {
			inWord = true
		}
	}
	if inWord {
		n++
	}
	return n
}
