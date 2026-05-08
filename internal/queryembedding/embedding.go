package queryembedding

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultOpenAIModel = "text-embedding-3-small"
	DefaultProvider    = "openai"

	maxOpenAIEmbeddingAttempts = 6
	defaultOpenAIRetryDelay    = time.Second
	maxOpenAIRetryDelay        = 10 * time.Second
)

var ErrUnavailable = errors.New("embedding provider unavailable")

type OpenAIAPIError struct {
	StatusCode int
	Code       string
	Message    string
	Model      string
	Raw        string
	RetryAfter time.Duration
}

func (e *OpenAIAPIError) Error() string {
	if e == nil {
		return "openai embeddings API error"
	}
	msg := strings.TrimSpace(e.Message)
	if msg == "" {
		msg = strings.TrimSpace(e.Raw)
	}
	if msg == "" {
		msg = "request failed"
	}
	if e.Code != "" {
		return fmt.Sprintf("openai embeddings HTTP %d (%s): %s", e.StatusCode, e.Code, msg)
	}
	return fmt.Sprintf("openai embeddings HTTP %d: %s", e.StatusCode, msg)
}

func (e *OpenAIAPIError) ModelUnavailable() bool {
	if e == nil {
		return false
	}
	code := strings.TrimSpace(e.Code)
	msg := strings.ToLower(e.Message)
	return code == "model_not_found" ||
		strings.Contains(msg, "does not have access to model") ||
		strings.Contains(msg, "model_not_found")
}

func (e *OpenAIAPIError) RateLimited() bool {
	if e == nil {
		return false
	}
	return e.StatusCode == http.StatusTooManyRequests || strings.TrimSpace(e.Code) == "rate_limit_exceeded"
}

type Provider interface {
	Name() string
	Model() string
	Dimension() int
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

type ProviderConfig struct {
	Provider   string
	Model      string
	APIKey     string
	BaseURL    string
	Dimensions int
	HTTPClient *http.Client
}

func NewProviderFromEnv(model string) (Provider, error) {
	cfg := ProviderConfig{
		Provider: strings.TrimSpace(strings.ToLower(os.Getenv("AFS_EMBED_PROVIDER"))),
		Model:    firstNonEmpty(model, os.Getenv("AFS_EMBED_MODEL")),
		APIKey:   strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
		BaseURL:  strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")),
	}
	if raw := strings.TrimSpace(os.Getenv("AFS_EMBED_DIMENSIONS")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("AFS_EMBED_DIMENSIONS must be a positive integer")
		}
		cfg.Dimensions = n
	}
	return NewProvider(cfg)
}

func NewProvider(cfg ProviderConfig) (Provider, error) {
	provider := strings.TrimSpace(strings.ToLower(cfg.Provider))
	model := strings.TrimSpace(cfg.Model)
	if strings.HasPrefix(strings.ToLower(model), "openai:") {
		provider = "openai"
		model = strings.TrimSpace(model[len("openai:"):])
	}
	if provider == "" {
		provider = DefaultProvider
	}
	switch provider {
	case "openai":
		if model == "" {
			model = DefaultOpenAIModel
		}
		if strings.TrimSpace(cfg.APIKey) == "" {
			return nil, fmt.Errorf("%w: set OPENAI_API_KEY for semantic embeddings", ErrUnavailable)
		}
		return NewOpenAIProvider(OpenAIConfig{
			Model:      model,
			APIKey:     cfg.APIKey,
			BaseURL:    cfg.BaseURL,
			Dimensions: cfg.Dimensions,
			HTTPClient: cfg.HTTPClient,
		})
	case "local":
		return nil, fmt.Errorf("%w: local GGUF embeddings are not wired into AFS yet; use AFS_EMBED_PROVIDER=openai with OPENAI_API_KEY", ErrUnavailable)
	case "test":
		return NewTestProvider(model), nil
	default:
		return nil, fmt.Errorf("unsupported embedding provider %q", provider)
	}
}

func FormatQuery(text, modelURI string) string {
	text = strings.TrimSpace(text)
	if isQwen3EmbeddingModel(modelURI) {
		return "Instruct: Retrieve relevant documents for the given query\nQuery: " + text
	}
	return "task: search result | query: " + text
}

func FormatDocument(text, title, modelURI string) string {
	text = strings.TrimSpace(text)
	title = strings.TrimSpace(title)
	if isQwen3EmbeddingModel(modelURI) {
		if title == "" {
			return text
		}
		return title + "\n" + text
	}
	if title == "" {
		title = "none"
	}
	return "title: " + title + " | text: " + text
}

func isQwen3EmbeddingModel(modelURI string) bool {
	modelURI = strings.ToLower(modelURI)
	return strings.Contains(modelURI, "qwen") && strings.Contains(modelURI, "embed")
}

type OpenAIConfig struct {
	Model      string
	APIKey     string
	BaseURL    string
	Dimensions int
	HTTPClient *http.Client
}

type OpenAIProvider struct {
	model      string
	apiKey     string
	baseURL    string
	dimensions int
	client     *http.Client
}

func NewOpenAIProvider(cfg OpenAIConfig) (*OpenAIProvider, error) {
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = DefaultOpenAIModel
	}
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("%w: set OPENAI_API_KEY", ErrUnavailable)
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	dimensions := cfg.Dimensions
	if dimensions <= 0 {
		dimensions = knownOpenAIDimension(model)
	}
	if dimensions <= 0 {
		return nil, fmt.Errorf("embedding dimension is unknown for model %q; set AFS_EMBED_DIMENSIONS", model)
	}
	return &OpenAIProvider{
		model:      model,
		apiKey:     apiKey,
		baseURL:    baseURL,
		dimensions: dimensions,
		client:     client,
	}, nil
}

func (p *OpenAIProvider) Name() string {
	return "openai"
}

func (p *OpenAIProvider) Model() string {
	return "openai:" + p.model
}

func (p *OpenAIProvider) Dimension() int {
	return p.dimensions
}

func (p *OpenAIProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	vectors, err := p.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vectors) != 1 {
		return nil, fmt.Errorf("openai embeddings returned %d vectors, want 1", len(vectors))
	}
	return vectors[0], nil
}

func (p *OpenAIProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}
	input := make([]string, 0, len(texts))
	for _, text := range texts {
		text = strings.TrimSpace(text)
		if text == "" {
			text = " "
		}
		input = append(input, text)
	}
	body := openAIEmbeddingRequest{
		Model:          p.model,
		Input:          input,
		EncodingFormat: "float",
	}
	if p.dimensions > 0 && openAIModelSupportsDimensions(p.model) {
		body.Dimensions = p.dimensions
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	var data []byte
	for attempt := 0; attempt < maxOpenAIEmbeddingAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/embeddings", bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
		req.Header.Set("Content-Type", "application/json")
		resp, err := p.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("%w: openai embeddings request failed: %v", ErrUnavailable, err)
		}
		data, err = io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		_ = resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			break
		}
		apiErr := openAIAPIError(resp.StatusCode, resp.Header, data, p.model)
		if !apiErr.RateLimited() || attempt == maxOpenAIEmbeddingAttempts-1 {
			return nil, fmt.Errorf("%w: %w", ErrUnavailable, apiErr)
		}
		if err := sleepOpenAIRetry(ctx, apiErr, attempt); err != nil {
			return nil, err
		}
	}
	var decoded openAIEmbeddingResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, err
	}
	out := make([][]float32, len(texts))
	for _, item := range decoded.Data {
		if item.Index < 0 || item.Index >= len(out) {
			return nil, fmt.Errorf("openai embeddings returned out-of-range index %d", item.Index)
		}
		if len(item.Embedding) != p.dimensions {
			return nil, fmt.Errorf("openai embeddings dimension = %d, want %d", len(item.Embedding), p.dimensions)
		}
		out[item.Index] = normalize(item.Embedding)
	}
	for i, vector := range out {
		if len(vector) == 0 {
			return nil, fmt.Errorf("openai embeddings response missing index %d", i)
		}
	}
	return out, nil
}

func openAIAPIError(statusCode int, header http.Header, data []byte, model string) *OpenAIAPIError {
	raw := strings.TrimSpace(string(data))
	apiErr := &OpenAIAPIError{
		StatusCode: statusCode,
		Model:      strings.TrimSpace(model),
		Raw:        raw,
	}
	var decoded struct {
		Error struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &decoded); err == nil {
		apiErr.Message = strings.TrimSpace(decoded.Error.Message)
		apiErr.Code = strings.TrimSpace(decoded.Error.Code)
	}
	apiErr.RetryAfter = openAIRetryDelay(header, apiErr.Message)
	return apiErr
}

func sleepOpenAIRetry(ctx context.Context, apiErr *OpenAIAPIError, attempt int) error {
	delay := defaultOpenAIRetryDelay
	if apiErr != nil && apiErr.RetryAfter > 0 {
		delay = apiErr.RetryAfter
	} else {
		delay = time.Duration(1<<attempt) * defaultOpenAIRetryDelay
	}
	if delay > maxOpenAIRetryDelay {
		delay = maxOpenAIRetryDelay
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func openAIRetryDelay(header http.Header, message string) time.Duration {
	if header != nil {
		raw := strings.TrimSpace(header.Get("Retry-After"))
		if raw != "" {
			if seconds, err := strconv.ParseFloat(raw, 64); err == nil && seconds > 0 {
				return time.Duration(seconds * float64(time.Second))
			}
			if parsed, err := http.ParseTime(raw); err == nil {
				if delay := time.Until(parsed); delay > 0 {
					return delay
				}
			}
		}
	}
	lower := strings.ToLower(message)
	marker := "try again in "
	idx := strings.Index(lower, marker)
	if idx < 0 {
		return 0
	}
	rest := lower[idx+len(marker):]
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return 0
	}
	raw := strings.TrimRight(fields[0], ".,")
	switch {
	case strings.HasSuffix(raw, "ms"):
		value, err := strconv.ParseFloat(strings.TrimSuffix(raw, "ms"), 64)
		if err == nil && value > 0 {
			return time.Duration(value * float64(time.Millisecond))
		}
	case strings.HasSuffix(raw, "s"):
		value, err := strconv.ParseFloat(strings.TrimSuffix(raw, "s"), 64)
		if err == nil && value > 0 {
			return time.Duration(value * float64(time.Second))
		}
	default:
		value, err := strconv.ParseFloat(raw, 64)
		if err == nil && value > 0 && len(fields) > 1 && strings.HasPrefix(fields[1], "second") {
			return time.Duration(value * float64(time.Second))
		}
	}
	return 0
}

type openAIEmbeddingRequest struct {
	Model          string   `json:"model"`
	Input          []string `json:"input"`
	EncodingFormat string   `json:"encoding_format"`
	Dimensions     int      `json:"dimensions,omitempty"`
}

type openAIEmbeddingResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func knownOpenAIDimension(model string) int {
	switch strings.TrimSpace(model) {
	case "text-embedding-3-small", "text-embedding-ada-002":
		return 1536
	case "text-embedding-3-large":
		return 3072
	default:
		return 0
	}
}

func openAIModelSupportsDimensions(model string) bool {
	return strings.HasPrefix(strings.TrimSpace(model), "text-embedding-3")
}

func EncodeFloat32(vec []float32) []byte {
	out := make([]byte, len(vec)*4)
	for i, value := range vec {
		binary.LittleEndian.PutUint32(out[i*4:], math.Float32bits(value))
	}
	return out
}

func DecodeFloat32(data []byte) []float32 {
	if len(data)%4 != 0 {
		return nil
	}
	out := make([]float32, len(data)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[i*4:]))
	}
	return out
}

func normalize(vec []float32) []float32 {
	norm := 0.0
	for _, value := range vec {
		norm += float64(value * value)
	}
	if norm == 0 {
		return vec
	}
	scale := float32(1 / math.Sqrt(norm))
	out := make([]float32, len(vec))
	for i, value := range vec {
		out[i] = value * scale
	}
	return out
}

func Cosine(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	dot := 0.0
	for i := range a {
		dot += float64(a[i] * b[i])
	}
	if dot < 0 {
		return 0
	}
	if dot > 1 {
		return 1
	}
	return dot
}

func TitleFromPath(p string) string {
	name := strings.TrimSpace(path.Base(p))
	if name == "." || name == "/" {
		return ""
	}
	return name
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
