package queryembedding

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	DefaultLocalDimension = 768

	localHelperCommandEnv = "AFS_EMBED_HELPER_CMD"
	localHelperScriptEnv  = "AFS_EMBED_HELPER_SCRIPT"
	localHelperModuleEnv  = "AFS_NODE_LLAMA_CPP_MODULE"
	defaultLocalHelperCmd = "node"
)

//go:embed node_llama_helper.mjs
var nodeLlamaHelperSource string

type LocalGGUFConfig struct {
	Model      string
	Dimensions int
	Command    string
	Script     string
}

type LocalGGUFProvider struct {
	model      string
	cacheDir   string
	dimensions int
	command    string
	script     string

	mu      sync.Mutex
	helper  *localEmbeddingHelper
	nextReq atomic.Int64
}

func NewLocalGGUFProvider(cfg LocalGGUFConfig) (*LocalGGUFProvider, error) {
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = DefaultLocalModel
	}
	spec, err := ParseLocalModelSpec(model)
	if err != nil {
		return nil, err
	}
	cacheDir, err := localModelCacheDir("")
	if err != nil {
		return nil, err
	}
	dimensions := cfg.Dimensions
	if dimensions <= 0 {
		dimensions = knownLocalDimension(spec.ID)
	}
	if dimensions <= 0 {
		return nil, fmt.Errorf("embedding dimension is unknown for local model %q; set AFS_EMBED_DIMENSIONS", spec.ID)
	}
	command := firstNonEmpty(cfg.Command, os.Getenv(localHelperCommandEnv), defaultLocalHelperCmd)
	if _, err := exec.LookPath(command); err != nil {
		return nil, fmt.Errorf("%w: local embedding helper runtime %q was not found in PATH; install Node.js or set %s", ErrUnavailable, command, localHelperCommandEnv)
	}
	script := firstNonEmpty(cfg.Script, os.Getenv(localHelperScriptEnv))
	if script == "" {
		script, err = ensureEmbeddedNodeHelper(cacheDir)
		if err != nil {
			return nil, err
		}
	}
	return &LocalGGUFProvider{
		model:      spec.ID,
		cacheDir:   cacheDir,
		dimensions: dimensions,
		command:    command,
		script:     script,
	}, nil
}

func (p *LocalGGUFProvider) Name() string {
	return "local"
}

func (p *LocalGGUFProvider) Model() string {
	return p.model
}

func (p *LocalGGUFProvider) Dimension() int {
	return p.dimensions
}

func (p *LocalGGUFProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	vectors, err := p.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vectors) != 1 {
		return nil, fmt.Errorf("local embeddings returned %d vectors, want 1", len(vectors))
	}
	return vectors[0], nil
}

func (p *LocalGGUFProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	helper, err := p.ensureHelperLocked(ctx)
	if err != nil {
		return nil, err
	}
	req := localHelperRequest{
		ID:    p.nextReq.Add(1),
		Op:    "embedBatch",
		Texts: normalizeEmbeddingInputs(texts),
	}
	resp, err := helper.roundTrip(ctx, req)
	if err != nil {
		helper.stop()
		p.helper = nil
		return nil, err
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, resp.Error)
	}
	if len(resp.Vectors) != len(texts) {
		return nil, fmt.Errorf("local embeddings returned %d vectors, want %d", len(resp.Vectors), len(texts))
	}
	out := make([][]float32, len(resp.Vectors))
	for i, vec := range resp.Vectors {
		if len(vec) != p.dimensions {
			return nil, fmt.Errorf("local embedding dimension = %d, want %d", len(vec), p.dimensions)
		}
		out[i] = normalize(vec)
	}
	return out, nil
}

func (p *LocalGGUFProvider) ensureHelperLocked(ctx context.Context) (*localEmbeddingHelper, error) {
	if p.helper != nil && p.helper.alive() {
		return p.helper, nil
	}
	helper, err := startLocalEmbeddingHelper(ctx, localEmbeddingHelperConfig{
		Command:    p.command,
		Script:     p.script,
		Model:      p.model,
		CacheDir:   p.cacheDir,
		Dimensions: p.dimensions,
	})
	if err != nil {
		return nil, err
	}
	p.helper = helper
	return helper, nil
}

type localEmbeddingHelperConfig struct {
	Command    string
	Script     string
	Model      string
	CacheDir   string
	Dimensions int
}

type localEmbeddingHelper struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	reader *bufio.Reader
	done   chan error
	path   string
}

type localHelperRequest struct {
	ID    int64    `json:"id"`
	Op    string   `json:"op"`
	Texts []string `json:"texts"`
}

type localHelperResponse struct {
	ID      int64       `json:"id"`
	Vectors [][]float32 `json:"vectors,omitempty"`
	Path    string      `json:"path,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func startLocalEmbeddingHelper(ctx context.Context, cfg localEmbeddingHelperConfig) (*localEmbeddingHelper, error) {
	args := []string{
		cfg.Script,
		"--model", cfg.Model,
		"--cache-dir", cfg.CacheDir,
		"--dimensions", strconv.Itoa(cfg.Dimensions),
	}
	cmd := exec.CommandContext(ctx, cfg.Command, args...)
	cmd.Env = os.Environ()
	stderr := &limitedBuffer{Limit: 32 << 10}
	cmd.Stderr = stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%w: start local embedding helper: %v", ErrUnavailable, err)
	}
	helper := &localEmbeddingHelper{
		cmd:    cmd,
		stdin:  stdin,
		reader: bufio.NewReader(stdout),
		done:   make(chan error, 1),
	}
	go func() {
		helper.done <- cmd.Wait()
	}()

	readyCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	resp, err := helper.roundTrip(readyCtx, localHelperRequest{ID: 0, Op: "ready"})
	if err != nil {
		helper.stop()
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("%w: local embedding helper failed to start: %s", ErrUnavailable, msg)
		}
		return nil, err
	}
	if resp.Error != "" {
		helper.stop()
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, resp.Error)
	}
	helper.path = strings.TrimSpace(resp.Path)
	return helper, nil
}

func (h *localEmbeddingHelper) alive() bool {
	select {
	case <-h.done:
		return false
	default:
		return true
	}
}

func (h *localEmbeddingHelper) roundTrip(ctx context.Context, req localHelperRequest) (localHelperResponse, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return localHelperResponse{}, err
	}
	if _, err := h.stdin.Write(append(payload, '\n')); err != nil {
		return localHelperResponse{}, fmt.Errorf("%w: write local embedding helper request: %v", ErrUnavailable, err)
	}
	type readResult struct {
		resp localHelperResponse
		err  error
	}
	ch := make(chan readResult, 1)
	go func() {
		line, err := h.reader.ReadBytes('\n')
		if err != nil {
			ch <- readResult{err: fmt.Errorf("%w: read local embedding helper response: %v", ErrUnavailable, err)}
			return
		}
		var resp localHelperResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			ch <- readResult{err: fmt.Errorf("%w: decode local embedding helper response: %v", ErrUnavailable, err)}
			return
		}
		ch <- readResult{resp: resp}
	}()
	select {
	case result := <-ch:
		if result.err != nil {
			return localHelperResponse{}, result.err
		}
		if result.resp.ID != req.ID {
			return localHelperResponse{}, fmt.Errorf("%w: local embedding helper response id %d, want %d", ErrUnavailable, result.resp.ID, req.ID)
		}
		return result.resp, nil
	case <-ctx.Done():
		h.stop()
		return localHelperResponse{}, ctx.Err()
	case err := <-h.done:
		if err == nil {
			err = errors.New("helper exited")
		}
		return localHelperResponse{}, fmt.Errorf("%w: local embedding helper exited: %v", ErrUnavailable, err)
	}
}

func (h *localEmbeddingHelper) stop() {
	if h == nil || h.cmd == nil || h.cmd.Process == nil {
		return
	}
	_ = h.stdin.Close()
	_ = h.cmd.Process.Kill()
	select {
	case <-h.done:
	case <-time.After(2 * time.Second):
	}
}

type limitedBuffer struct {
	Limit int
	data  []byte
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Limit <= 0 || len(b.data) < b.Limit {
		remaining := b.Limit - len(b.data)
		if b.Limit <= 0 || remaining > len(p) {
			remaining = len(p)
		}
		b.data = append(b.data, p[:remaining]...)
	}
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	return string(b.data)
}

func ensureEmbeddedNodeHelper(cacheDir string) (string, error) {
	helperDir := filepath.Join(cacheDir, "helper")
	if err := os.MkdirAll(helperDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(helperDir, "node_llama_helper.mjs")
	if existing, err := os.ReadFile(path); err == nil && string(existing) == nodeLlamaHelperSource {
		return path, nil
	}
	if err := os.WriteFile(path, []byte(nodeLlamaHelperSource), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func normalizeEmbeddingInputs(texts []string) []string {
	out := make([]string, len(texts))
	for i, text := range texts {
		text = strings.TrimSpace(text)
		if text == "" {
			text = " "
		}
		out[i] = text
	}
	return out
}

func knownLocalDimension(model string) int {
	model = strings.ToLower(strings.TrimSpace(model))
	if strings.Contains(model, "embeddinggemma") || strings.Contains(model, "embedding-gemma") {
		return DefaultLocalDimension
	}
	return 0
}

func WarmLocalGGUFModel(ctx context.Context, model string) (LocalModelDownloadResult, error) {
	before, err := ResolveLocalModelStatus(model, "")
	if err != nil {
		return LocalModelDownloadResult{}, err
	}
	dimensions := 0
	if raw := strings.TrimSpace(os.Getenv("AFS_EMBED_DIMENSIONS")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return LocalModelDownloadResult{}, fmt.Errorf("AFS_EMBED_DIMENSIONS must be a positive integer")
		}
		dimensions = n
	}
	provider, err := NewLocalGGUFProvider(LocalGGUFConfig{Model: model, Dimensions: dimensions})
	if err != nil {
		return LocalModelDownloadResult{}, err
	}
	provider.mu.Lock()
	helper, err := provider.ensureHelperLocked(ctx)
	if err != nil {
		provider.mu.Unlock()
		return LocalModelDownloadResult{}, err
	}
	path := strings.TrimSpace(helper.path)
	helper.stop()
	provider.helper = nil
	provider.mu.Unlock()

	after := before
	if path != "" {
		after.Path = path
	}
	if st, err := os.Stat(after.Path); err == nil && !st.IsDir() {
		after.Exists = true
		after.SizeBytes = st.Size()
	}
	return LocalModelDownloadResult{
		LocalModelStatus: after,
		Downloaded:       !before.Exists && after.Exists,
	}, nil
}
