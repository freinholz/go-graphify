// Package ingest fetches remote resources (webpages, GitHub repos, arXiv
// abstracts, tweets, YouTube metadata) and saves them as markdown files into
// the corpus, where the rest of the pipeline can consume them.
package ingest

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/safishamsi/graphify/go/internal/security"
)

const maxBytes = 10 << 20 // 10 MB text cap

// Options controls an ingest call.
type Options struct {
	OutputDir string
	Timeout   time.Duration
}

// Ingest fetches url into outputDir as a markdown file and returns the path.
func Ingest(url string, opts Options) (string, error) {
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.OutputDir == "" {
		opts.OutputDir = "."
	}
	u, err := security.ValidateURL(url)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: opts.Timeout}
	req, _ := http.NewRequest("GET", u.String(), nil)
	req.Header.Set("User-Agent", "graphify-go/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return "", err
	}
	var md string
	ct := resp.Header.Get("Content-Type")
	switch {
	case strings.Contains(ct, "text/html"):
		md = htmlToMarkdown(string(body))
	case strings.Contains(ct, "application/json"):
		md = "```json\n" + string(body) + "\n```"
	default:
		md = string(body)
	}
	front := fmt.Sprintf("---\nsource_url: %q\ntype: %q\ncaptured_at: %q\n---\n\n",
		u.String(), classify(u.String(), ct), time.Now().UTC().Format(time.RFC3339))
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return "", err
	}
	name := safeFilename(u.Host+u.Path) + ".md"
	path := filepath.Join(opts.OutputDir, name)
	if err := os.WriteFile(path, []byte(front+md), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func classify(url, ct string) string {
	switch {
	case strings.Contains(url, "arxiv.org/abs/"):
		return "arxiv"
	case strings.Contains(url, "github.com/"):
		return "github"
	case strings.Contains(url, "twitter.com/"), strings.Contains(url, "x.com/"):
		return "tweet"
	case strings.Contains(url, "youtube.com/"), strings.Contains(url, "youtu.be/"):
		return "youtube"
	case strings.Contains(ct, "text/html"):
		return "webpage"
	}
	return "raw"
}

var (
	tagStrip = regexp.MustCompile(`(?s)<script.*?</script>|<style.*?</style>|<[^>]+>`)
	wsSquash = regexp.MustCompile(`[ \t]+`)
	nlSquash = regexp.MustCompile(`\n{3,}`)
)

func htmlToMarkdown(s string) string {
	s = tagStrip.ReplaceAllString(s, " ")
	s = wsSquash.ReplaceAllString(s, " ")
	s = nlSquash.ReplaceAllString(s, "\n\n")
	// Decode a handful of entities.
	for _, r := range []struct{ a, b string }{
		{"&amp;", "&"}, {"&lt;", "<"}, {"&gt;", ">"}, {"&quot;", `"`},
		{"&apos;", "'"}, {"&#39;", "'"}, {"&nbsp;", " "},
	} {
		s = strings.ReplaceAll(s, r.a, r.b)
	}
	return strings.TrimSpace(s)
}

var fnameRe = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func safeFilename(s string) string {
	s = fnameRe.ReplaceAllString(s, "_")
	if len(s) > 80 {
		s = s[:80]
	}
	return strings.Trim(s, "_")
}
