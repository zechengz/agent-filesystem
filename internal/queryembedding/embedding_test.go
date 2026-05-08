package queryembedding

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
