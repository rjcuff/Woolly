// Package recall finds the memories most relevant to a query, with keyword
// search plus Ollama embeddings when they're available.
package recall

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/rjcuff/woolly/internal/memory"
)

// Mode says how a recall was scored.
type Mode string

const (
	ModeHybrid  Mode = "hybrid"  // keyword + semantic
	ModeKeyword Mode = "keyword" // keyword only (no embedder available)
)

// Embedder turns text into vectors.
type Embedder interface {
	Model() string
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// Result is one recalled memory with its relevance score.
type Result struct {
	Memory memory.Memory `json:"memory"`
	Score  float64       `json:"score"`
}

// Engine runs recall queries against a store.
type Engine struct {
	store     *memory.Store
	emb       Embedder
	cachePath string

	mu            sync.Mutex
	cache         *cacheFile
	disabledUntil time.Time
	lastErr       error
}

// New creates an engine. emb may be nil for keyword-only search. Embeddings
// are cached in cachePath so each memory is only embedded once.
func New(store *memory.Store, emb Embedder, cachePath string) *Engine {
	return &Engine{store: store, emb: emb, cachePath: cachePath, cache: loadCache(cachePath)}
}

// LastEmbedError returns the most recent embedding failure, if any.
func (e *Engine) LastEmbedError() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastErr
}

// Recall returns up to limit memories relevant to query, best first.
// includeAll also searches superseded and conflicting memories.
func (e *Engine) Recall(ctx context.Context, query string, limit int, includeAll bool) ([]Result, Mode, error) {
	return e.RecallIn(ctx, query, limit, includeAll, "")
}

// RecallIn is Recall inside one project: it searches memories about the
// user plus that project's memories, and leaves other projects out. An empty
// project searches everything.
func (e *Engine) RecallIn(ctx context.Context, query string, limit int, includeAll bool, project string) ([]Result, Mode, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, ModeKeyword, errors.New("query is empty")
	}
	if limit <= 0 {
		limit = 5
	}
	all, err := e.store.List(includeAll)
	if err != nil {
		return nil, ModeKeyword, err
	}
	project = memory.NormalizeProject(project)
	var mems []memory.Memory
	for i := range all {
		if all[i].In(project) {
			mems = append(mems, all[i])
		}
	}
	if len(mems) == 0 {
		return nil, ModeKeyword, nil
	}

	docs := make([]string, len(mems))
	toks := make([][]string, len(mems))
	for i, m := range mems {
		docs[i] = docText(m)
		toks[i] = tokenize(docs[i])
	}
	kw := bm25(expand(tokenize(query)), toks)

	mode := ModeKeyword
	var sem []float64
	if vecs, qv, err := e.vectors(ctx, mems, docs, query); err == nil {
		mode = ModeHybrid
		sem = make([]float64, len(mems))
		for i := range vecs {
			sem[i] = cosine(vecs[i], qv)
		}
	}

	now := time.Now()
	var out []Result
	for i, m := range mems {
		score := kw[i]
		if mode == ModeHybrid {
			if kw[i] == 0 && sem[i] < 0.5 {
				continue
			}
			score = 0.65*math.Max(sem[i], 0) + 0.35*kw[i]
		} else if kw[i] == 0 {
			continue
		}
		ageDays := now.Sub(m.UpdatedAt).Hours() / 24
		score *= 1 + 0.05*math.Exp(-ageDays/30) // slight preference for fresh memories
		if m.Status != memory.StatusActive {
			score *= 0.5
		}
		out = append(out, Result{Memory: m, Score: math.Round(score*1000) / 1000})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, mode, nil
}

func docText(m memory.Memory) string {
	key := strings.NewReplacer(".", " ", "_", " ", "-", " ", "/", " ").Replace(m.Key)
	return strings.TrimSpace(m.Content + " " + key + " " + strings.Join(m.Tags, " ") + " " + m.Project)
}

// vectors returns embeddings for every doc plus the query, embedding only
// what is missing from the cache.
func (e *Engine) vectors(ctx context.Context, mems []memory.Memory, docs []string, query string) ([][]float32, []float32, error) {
	if e.emb == nil {
		return nil, nil, errors.New("no embedder configured")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if time.Now().Before(e.disabledUntil) {
		return nil, nil, e.lastErr
	}
	model := e.emb.Model()
	if e.cache.Model != model {
		e.cache = &cacheFile{Model: model, Entries: map[string]cacheEntry{}}
	}

	vecs := make([][]float32, len(mems))
	var missIdx []int
	var texts []string
	for i, m := range mems {
		h := hashText(docs[i])
		if ent, ok := e.cache.Entries[m.ID]; ok && ent.Hash == h && len(ent.Vec) > 0 {
			vecs[i] = ent.Vec
			continue
		}
		missIdx = append(missIdx, i)
		texts = append(texts, docs[i])
	}
	texts = append(texts, query)

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	var got [][]float32
	for start := 0; start < len(texts); start += 64 {
		end := min(start+64, len(texts))
		chunk, err := e.emb.Embed(ctx, texts[start:end])
		if err == nil && len(chunk) != end-start {
			err = errors.New("embedder returned the wrong number of vectors")
		}
		if err != nil {
			e.lastErr = err
			e.disabledUntil = time.Now().Add(60 * time.Second) // don't retry on every query
			return nil, nil, err
		}
		got = append(got, chunk...)
	}
	for j, i := range missIdx {
		vecs[i] = got[j]
		e.cache.Entries[mems[i].ID] = cacheEntry{Hash: hashText(docs[i]), Vec: got[j]}
	}
	if len(missIdx) > 0 {
		_ = e.cache.save(e.cachePath)
	}
	e.lastErr = nil
	return vecs, got[len(got)-1], nil
}

type cacheEntry struct {
	Hash string    `json:"hash"`
	Vec  []float32 `json:"vec"`
}

type cacheFile struct {
	Model   string                `json:"model"`
	Entries map[string]cacheEntry `json:"entries"`
}

func loadCache(path string) *cacheFile {
	c := &cacheFile{Entries: map[string]cacheEntry{}}
	if path == "" {
		return c
	}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, c)
		if c.Entries == nil {
			c.Entries = map[string]cacheEntry{}
		}
	}
	return c
}

func (c *cacheFile) save(path string) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func hashText(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

func cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

var stopwords = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`a an the and or but if of to in on at by for with from as is are was were be been
		being am do does did have has had i me my we our you your he she it its they them their this that these those
		what which who whom where when why how about into over under again then than so not no yes can will would
		should could there here all any some just also very`) {
		stopwords[w] = true
	}
}

func tokenize(s string) []string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	out := fields[:0]
	for _, f := range fields {
		if len(f) < 2 || stopwords[f] {
			continue
		}
		out = append(out, stem(f))
	}
	return out
}

// synonyms bridges the most common personal-memory questions to the words
// facts are usually written with, so keyword-only search still answers
// "where do I live?" with "Moved to Denver". Semantic search covers the rest.
var synonyms = map[string][]string{}

func init() {
	groups := [][]string{
		{"live", "location", "home", "city", "moved", "move", "address", "based", "town"},
		{"work", "job", "company", "employer", "role", "career", "office"},
		{"like", "prefer", "preference", "favorite", "love", "enjoy"},
		{"dislike", "hate", "avoid"},
		{"name", "named", "called"},
		{"pet", "dog", "cat"},
		{"family", "wife", "husband", "partner", "kid", "child", "son", "daughter"},
		{"code", "coding", "programming", "language", "stack", "editor"},
		{"birthday", "born", "age"},
	}
	for _, g := range groups {
		for _, w := range g {
			synonyms[stem(w)] = g
		}
	}
}

func expand(q []string) []string {
	out := append([]string(nil), q...)
	for _, t := range q {
		for _, s := range synonyms[t] {
			out = append(out, stem(s))
		}
	}
	return out
}

// stem is a deliberately tiny stemmer: enough to match "dogs"/"dog" and
// "lives"/"live" without pulling in a dependency.
func stem(w string) string {
	switch {
	case len(w) > 4 && strings.HasSuffix(w, "ies"):
		return w[:len(w)-3] + "y"
	case len(w) > 3 && strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss"):
		return w[:len(w)-1]
	}
	return w
}

// bm25 scores each doc against the query and normalizes scores to 0..1.
func bm25(query []string, docs [][]string) []float64 {
	const k1, b = 1.2, 0.75
	scores := make([]float64, len(docs))
	if len(query) == 0 || len(docs) == 0 {
		return scores
	}
	df := map[string]int{}
	tfs := make([]map[string]int, len(docs))
	total := 0
	for i, d := range docs {
		total += len(d)
		tf := map[string]int{}
		for _, t := range d {
			tf[t]++
		}
		tfs[i] = tf
		for t := range tf {
			df[t]++
		}
	}
	n := float64(len(docs))
	avg := float64(total) / n
	if avg == 0 {
		avg = 1
	}
	seen := map[string]bool{}
	for _, t := range query {
		if seen[t] || df[t] == 0 {
			continue
		}
		seen[t] = true
		d := float64(df[t])
		idf := math.Log(1 + (n-d+0.5)/(d+0.5))
		for i := range docs {
			f := float64(tfs[i][t])
			if f == 0 {
				continue
			}
			dl := float64(len(docs[i]))
			scores[i] += idf * f * (k1 + 1) / (f + k1*(1-b+b*dl/avg))
		}
	}
	var hi float64
	for _, s := range scores {
		hi = math.Max(hi, s)
	}
	if hi > 0 {
		for i := range scores {
			scores[i] /= hi
		}
	}
	return scores
}
