// Package benchmark measures how much the knowledge graph reduces the LLM
// token budget needed to answer a question, vs. naively stuffing the whole
// corpus into context.
package benchmark

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/safishamsi/graphify/go/internal/core"
	"github.com/safishamsi/graphify/go/internal/pipeline"
)

// Result is per-question and overall token metrics.
type Result struct {
	CorpusTokens int            `json:"corpus_tokens"`
	Questions    map[string]int `json:"question_tokens"`
	Ratio        float64        `json:"reduction_ratio"`
}

// DefaultQuestions are asked against the graph to measure BFS-slice cost.
var DefaultQuestions = []string{
	"how does authentication work",
	"what is the main entry point",
	"how are errors handled",
	"where is the database schema",
	"what are the core data types",
}

// approxTokens estimates tokens from word count (rough 1.3× words).
func approxTokens(words int) int { return (words * 13) / 10 }

// Run scans the corpus under root, loads the provided graph, asks the sample
// questions and returns a reduction ratio.
func Run(root string, g *core.Graph) (Result, error) {
	words := corpusWords(root)
	corpusT := approxTokens(words)

	r := Result{CorpusTokens: corpusT, Questions: map[string]int{}}
	q := pipeline.GraphQuery{G: g}
	for _, question := range DefaultQuestions {
		nodes := q.Question(question, 3, 50)
		t := 0
		for _, n := range nodes {
			t += approxTokens(len(strings.Fields(n.Label + " " + n.SourceFile)))
		}
		r.Questions[question] = t
	}
	sum := 0
	for _, v := range r.Questions {
		sum += v
	}
	avg := 0
	if len(r.Questions) > 0 {
		avg = sum / len(r.Questions)
	}
	if avg > 0 {
		r.Ratio = float64(corpusT) / float64(avg)
	}
	return r, nil
}

func corpusWords(root string) int {
	n := 0
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		n += len(strings.Fields(string(b)))
		return nil
	})
	return n
}
