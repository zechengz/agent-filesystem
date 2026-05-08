package queryembedding

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTestProviderEmbedsRelatedTextCloserThanUnrelatedText(t *testing.T) {
	provider := NewTestProvider("")
	query, err := provider.Embed(context.Background(), "how do I save a snapshot?")
	if err != nil {
		t.Fatalf("Embed(query) returned error: %v", err)
	}
	related, err := provider.Embed(context.Background(), "checkpoint savepoint recovery guide")
	if err != nil {
		t.Fatalf("Embed(related) returned error: %v", err)
	}
	unrelated, err := provider.Embed(context.Background(), "tenant auth token login")
	if err != nil {
		t.Fatalf("Embed(unrelated) returned error: %v", err)
	}

	if got, want := len(query), TestDimension; got != want {
		t.Fatalf("vector dimension = %d, want %d", got, want)
	}
	if Cosine(query, related) <= Cosine(query, unrelated) {
		t.Fatalf("related score %.4f should beat unrelated %.4f", Cosine(query, related), Cosine(query, unrelated))
	}
}

func TestOpenAIProviderEmbedsBatch(t *testing.T) {
	var sawAuth bool
	var sawEncoding bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Fatalf("path = %q, want /v1/embeddings", r.URL.Path)
		}
		sawAuth = r.Header.Get("Authorization") == "Bearer test-key"
		var request openAIEmbeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("Decode(request) returned error: %v", err)
		}
		sawEncoding = request.EncodingFormat == "float"
		if request.Model != "text-embedding-3-small" {
			t.Fatalf("model = %q, want text-embedding-3-small", request.Model)
		}
		if len(request.Input) != 2 || !strings.Contains(request.Input[0], "query") {
			t.Fatalf("input = %#v, want two formatted inputs", request.Input)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"index": 0, "embedding": []float32{1, 0, 0}},
				{"index": 1, "embedding": []float32{0, 2, 0}},
			},
		})
	}))
	defer server.Close()

	provider, err := NewOpenAIProvider(OpenAIConfig{
		Model:      "text-embedding-3-small",
		APIKey:     "test-key",
		BaseURL:    server.URL,
		Dimensions: 3,
	})
	if err != nil {
		t.Fatalf("NewOpenAIProvider() returned error: %v", err)
	}
	vectors, err := provider.EmbedBatch(context.Background(), []string{
		FormatQuery("query", provider.Model()),
		FormatDocument("document", "doc.md", provider.Model()),
	})
	if err != nil {
		t.Fatalf("EmbedBatch() returned error: %v", err)
	}
	if !sawAuth || !sawEncoding {
		t.Fatalf("sawAuth=%t sawEncoding=%t, want both true", sawAuth, sawEncoding)
	}
	if len(vectors) != 2 || len(vectors[0]) != 3 || len(vectors[1]) != 3 {
		t.Fatalf("vectors = %#v, want two 3D vectors", vectors)
	}
	if vectors[1][1] != 1 {
		t.Fatalf("second vector = %#v, want normalized float vector", vectors[1])
	}
}

func TestOpenAIProviderReturnsStructuredAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "Project `proj_test` does not have access to model `text-embedding-3-small`",
				"code":    "model_not_found",
			},
		})
	}))
	defer server.Close()

	provider, err := NewOpenAIProvider(OpenAIConfig{
		Model:      "text-embedding-3-small",
		APIKey:     "test-key",
		BaseURL:    server.URL,
		Dimensions: 3,
	})
	if err != nil {
		t.Fatalf("NewOpenAIProvider() returned error: %v", err)
	}
	_, err = provider.EmbedBatch(context.Background(), []string{"query"})
	if err == nil {
		t.Fatal("EmbedBatch() returned nil error, want OpenAI API error")
	}
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("EmbedBatch() error = %v, want ErrUnavailable", err)
	}
	var apiErr *OpenAIAPIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("EmbedBatch() error = %v, want OpenAIAPIError", err)
	}
	if apiErr.StatusCode != http.StatusForbidden || apiErr.Code != "model_not_found" || apiErr.Model != "text-embedding-3-small" || !apiErr.ModelUnavailable() {
		t.Fatalf("OpenAIAPIError = %+v, want structured model access error", apiErr)
	}
}

func TestNewProviderFromEnvRequiresOpenAIKey(t *testing.T) {
	t.Setenv("AFS_EMBED_PROVIDER", "openai")
	t.Setenv("OPENAI_API_KEY", "")
	_, err := NewProviderFromEnv("openai:text-embedding-3-small")
	if err == nil {
		t.Fatal("NewProviderFromEnv() returned nil error, want missing key")
	}
	if !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("error = %q, want OPENAI_API_KEY guidance", err)
	}
}

func TestNewProviderFromEnvInfersLocalGGUFModel(t *testing.T) {
	t.Setenv("AFS_EMBED_PROVIDER", "")
	t.Setenv("AFS_EMBED_MODEL_DIR", t.TempDir())
	t.Setenv(localHelperCommandEnv, filepath.Join(t.TempDir(), "missing-node"))
	_, err := NewProviderFromEnv(DefaultLocalModel)
	if err == nil {
		t.Fatal("NewProviderFromEnv() returned nil error, want local runtime unavailable")
	}
	if !strings.Contains(err.Error(), "local embedding helper runtime") {
		t.Fatalf("error = %q, want local GGUF guidance", err)
	}
}

func TestNewProviderFromEnvUsesLocalGGUFRuntime(t *testing.T) {
	cacheDir := t.TempDir()
	runtimePath := filepath.Join(t.TempDir(), "afs-fake-embedding-helper")
	script := `#!/bin/sh
while IFS= read -r line; do
  case "$line" in
    *'"op":"ready"'*) printf '%s\n' '{"id":0}' ;;
    *) printf '%s\n' '{"id":1,"vectors":[[3,4,0]]}' ;;
  esac
done
`
	if err := os.WriteFile(runtimePath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(runtime) returned error: %v", err)
	}
	t.Setenv("AFS_EMBED_PROVIDER", "")
	t.Setenv("AFS_EMBED_MODEL_DIR", cacheDir)
	t.Setenv("AFS_EMBED_DIMENSIONS", "3")
	t.Setenv(localHelperCommandEnv, runtimePath)

	provider, err := NewProviderFromEnv(DefaultLocalModel)
	if err != nil {
		t.Fatalf("NewProviderFromEnv(local) returned error: %v", err)
	}
	if provider.Name() != "local" || provider.Model() != DefaultLocalModel || provider.Dimension() != 3 {
		t.Fatalf("provider = %s %s %d, want local %s 3", provider.Name(), provider.Model(), provider.Dimension(), DefaultLocalModel)
	}
	vec, err := provider.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Embed(local) returned error: %v", err)
	}
	if len(vec) != 3 || vec[0] <= 0.59 || vec[1] <= 0.79 || vec[2] != 0 {
		t.Fatalf("vec = %#v, want normalized fake runtime output", vec)
	}
}

func TestParseLocalModelSpecDefaultsToEmbeddingGemmaGGUF(t *testing.T) {
	spec, err := ParseLocalModelSpec("")
	if err != nil {
		t.Fatalf("ParseLocalModelSpec() returned error: %v", err)
	}
	if spec.ID != DefaultLocalModel ||
		spec.Repo != "ggml-org/embeddinggemma-300M-GGUF" ||
		spec.Filename != "embeddinggemma-300M-Q8_0.gguf" ||
		!strings.Contains(spec.URL, "/ggml-org/embeddinggemma-300M-GGUF/resolve/main/embeddinggemma-300M-Q8_0.gguf") {
		t.Fatalf("spec = %+v, want default embeddinggemma GGUF", spec)
	}
}

func TestEncodeDecodeFloat32RoundTrip(t *testing.T) {
	in := []float32{0.25, -0.5, 1}
	got := DecodeFloat32(EncodeFloat32(in))
	if len(got) != len(in) {
		t.Fatalf("decoded length = %d, want %d", len(got), len(in))
	}
	for i := range in {
		if got[i] != in[i] {
			t.Fatalf("decoded[%d] = %f, want %f", i, got[i], in[i])
		}
	}
}
