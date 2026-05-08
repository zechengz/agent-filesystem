package queryvector

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/agent-filesystem/internal/queryembedding"
	"github.com/redis/agent-filesystem/internal/queryindex"
	"github.com/redis/agent-filesystem/internal/rediscontent"
	"github.com/redis/go-redis/v9"
)

func TestSearchFallsBackToLocalVectorRanking(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = rdb.Close()
	})
	fsKey := "queryvector-test"
	writeVectorTestFile(t, ctx, rdb, fsKey, "1", "/docs/checkpoints.md", "checkpoint savepoint snapshot recovery\nrestore workspace state\n")
	writeVectorTestFile(t, ctx, rdb, fsKey, "2", "/notes/auth.md", "tenant auth token login\n")
	if _, err := Backfill(ctx, rdb, fsKey, queryembedding.NewTestProvider(""), SearchOptions{Path: "/docs"}); err != nil {
		t.Fatalf("Backfill() returned error: %v", err)
	}

	result, err := Search(ctx, rdb, fsKey, queryembedding.NewTestProvider(""), "how do I save a snapshot?", SearchOptions{Path: "/docs", Limit: 5})
	if err != nil {
		t.Fatalf("Search() returned error: %v", err)
	}
	if result.Stats.Backend != "local_vector" {
		t.Fatalf("backend = %q, want local_vector fallback", result.Stats.Backend)
	}
	if len(result.Results) != 1 || result.Results[0].Path != "/docs/checkpoints.md" {
		t.Fatalf("results = %#v, want scoped checkpoints result", result.Results)
	}
	if len(result.Results[0].SearchTypes) != 1 || result.Results[0].SearchTypes[0] != "vec" {
		t.Fatalf("search types = %#v, want vec evidence", result.Results[0].SearchTypes)
	}
	if !strings.Contains(result.Results[0].Snippet, "checkpoint") {
		t.Fatalf("snippet = %q, want chunk preview", result.Results[0].Snippet)
	}
}

func TestSearchSemanticSnippetUsesPreviewLineRangeUnlessFull(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = rdb.Close()
	})
	fsKey := "queryvector-test"
	lines := make([]string, 0, 40)
	for i := 1; i <= 40; i++ {
		lines = append(lines, fmt.Sprintf("semantic setup guide line %02d", i))
	}
	writeVectorTestFile(t, ctx, rdb, fsKey, "1", "/docs/long.md", strings.Join(lines, "\n"))
	provider := queryembedding.NewTestProvider("")
	if _, err := Backfill(ctx, rdb, fsKey, provider, SearchOptions{Path: "/docs"}); err != nil {
		t.Fatalf("Backfill() returned error: %v", err)
	}

	result, err := Search(ctx, rdb, fsKey, provider, "semantic setup", SearchOptions{Path: "/docs", Limit: 1})
	if err != nil {
		t.Fatalf("Search() returned error: %v", err)
	}
	if len(result.Results) != 1 {
		t.Fatalf("results = %#v, want one semantic result", result.Results)
	}
	got := result.Results[0]
	if got.StartLine != 1 || got.EndLine != semanticSnippetMaxLines {
		t.Fatalf("line range = %d-%d, want preview range 1-%d", got.StartLine, got.EndLine, semanticSnippetMaxLines)
	}
	if strings.Contains(got.Snippet, "line 20") {
		t.Fatalf("snippet = %q, want compact semantic preview", got.Snippet)
	}

	full, err := Search(ctx, rdb, fsKey, provider, "semantic setup", SearchOptions{Path: "/docs", Limit: 1, Full: true})
	if err != nil {
		t.Fatalf("Search(full) returned error: %v", err)
	}
	if full.Results[0].EndLine <= got.EndLine || !strings.Contains(full.Results[0].Snippet, "line 20") {
		t.Fatalf("full result = %+v, want full chunk output", full.Results[0])
	}
}

func TestSearchDoesNotBackfillMissingEmbeddings(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = rdb.Close()
	})
	fsKey := "queryvector-test"
	provider := &recordingProvider{dimension: 3}
	writeVectorTestFile(t, ctx, rdb, fsKey, "1", "/docs/checkpoints.md", "checkpoint savepoint snapshot recovery\n")

	result, err := Search(ctx, rdb, fsKey, provider, "checkpoint", SearchOptions{Path: "/docs", Limit: 5})
	if err != nil {
		t.Fatalf("Search() returned error: %v", err)
	}
	if len(result.Results) != 0 {
		t.Fatalf("results = %#v, want no results without prebuilt embeddings", result.Results)
	}
	if len(provider.batches) != 1 || provider.batches[0] != 1 {
		t.Fatalf("embedding batches = %#v, want only query embedding call", provider.batches)
	}
}

func TestIndexNameAndEmbeddingFieldAreProviderScoped(t *testing.T) {
	fsKey := "queryvector-test"
	first := queryembedding.NewTestProvider("model-a")
	second := queryembedding.NewTestProvider("model-b")

	if IndexName(fsKey, first) == IndexName(fsKey, second) {
		t.Fatalf("IndexName should differ across embedding models")
	}
	if embeddingField(first) == embeddingField(second) {
		t.Fatalf("embeddingField should differ across embedding models")
	}
}

func TestEmbedAndStorePendingChunksSplitsAndPersistsPartialProgress(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = rdb.Close()
	})
	provider := &recordingProvider{dimension: 3, failOnBatch: 2}
	fieldName := embeddingField(provider)
	pending := make([]chunkEmbedding, maxEmbeddingBatchInputs+7)
	for i := range pending {
		key := fmt.Sprintf("chunk-%d", i)
		pending[i] = chunkEmbedding{Key: key, Text: strings.Repeat("workspace ", 300), Hash: fmt.Sprintf("hash-%d", i)}
		if err := rdb.HSet(ctx, key, "type", "chunk").Err(); err != nil {
			t.Fatalf("HSET chunk: %v", err)
		}
	}

	embedded, err := embedAndStorePendingChunks(ctx, rdb, fieldName, provider, pending)
	if !errors.Is(err, errRecordingProviderFailed) {
		t.Fatalf("embedAndStorePendingChunks() error = %v, want recording failure", err)
	}
	if embedded != maxEmbeddingBatchInputs {
		t.Fatalf("embedded = %d, want first batch persisted", embedded)
	}
	if len(provider.batches) < 2 {
		t.Fatalf("batches = %#v, want split embedding requests", provider.batches)
	}
	for _, size := range provider.batches {
		if size > maxEmbeddingBatchInputs {
			t.Fatalf("batch size = %d, want <= %d", size, maxEmbeddingBatchInputs)
		}
	}
	if got, err := rdb.HGet(ctx, pending[0].Key, "embedding_model").Result(); err != nil || got != provider.Model() {
		t.Fatalf("first persisted embedding_model = %q, %v; want %q", got, err, provider.Model())
	}
	if got, _ := rdb.HGet(ctx, pending[len(pending)-1].Key, "embedding_model").Result(); got != "" {
		t.Fatalf("last embedding_model = %q, want not persisted after failed batch", got)
	}
}

var errRecordingProviderFailed = errors.New("recording provider failed")

type recordingProvider struct {
	dimension   int
	batches     []int
	failOnBatch int
}

func (p *recordingProvider) Name() string {
	return "recording"
}

func (p *recordingProvider) Model() string {
	return "recording-embedding"
}

func (p *recordingProvider) Dimension() int {
	return p.dimension
}

func (p *recordingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	vectors, err := p.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return vectors[0], nil
}

func (p *recordingProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p.batches = append(p.batches, len(texts))
	if p.failOnBatch > 0 && len(p.batches) == p.failOnBatch {
		return nil, errRecordingProviderFailed
	}
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{1, 0, 0}
	}
	return out, nil
}

func writeVectorTestFile(t *testing.T, ctx context.Context, rdb *redis.Client, fsKey, inodeID, filePath, text string) {
	t.Helper()
	if err := rdb.Set(ctx, queryindex.ContentKey(fsKey, inodeID), []byte(text), 0).Err(); err != nil {
		t.Fatalf("SET content: %v", err)
	}
	if err := rdb.HSet(ctx, queryindex.InodeKey(fsKey, inodeID), map[string]interface{}{
		"type":           "file",
		"path":           filePath,
		"path_ancestors": queryindex.IndexedPathAncestors(filePath),
		"content_ref":    rediscontent.RefExternal,
		"size":           len(text),
	}).Err(); err != nil {
		t.Fatalf("HSET inode: %v", err)
	}
	pipe := rdb.Pipeline()
	queryindex.QueueMarkDirty(ctx, pipe, fsKey, inodeID)
	if _, err := pipe.Exec(ctx); err != nil {
		t.Fatalf("QueueMarkDirty: %v", err)
	}
}
