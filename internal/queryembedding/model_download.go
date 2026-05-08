package queryembedding

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultLocalModel = "hf:ggml-org/embeddinggemma-300M-GGUF/embeddinggemma-300M-Q8_0.gguf"
)

type LocalModelSpec struct {
	ID       string `json:"id"`
	Repo     string `json:"repo,omitempty"`
	Filename string `json:"filename"`
	URL      string `json:"url"`
}

type LocalModelStatus struct {
	Spec      LocalModelSpec `json:"spec"`
	CacheDir  string         `json:"cache_dir"`
	Path      string         `json:"path"`
	Exists    bool           `json:"exists"`
	SizeBytes int64          `json:"size_bytes"`
}

type LocalModelDownloadResult struct {
	LocalModelStatus
	Downloaded bool `json:"downloaded"`
}

func ResolveLocalModelStatus(model, cacheDir string) (LocalModelStatus, error) {
	spec, err := ParseLocalModelSpec(model)
	if err != nil {
		return LocalModelStatus{}, err
	}
	dir, err := localModelCacheDir(cacheDir)
	if err != nil {
		return LocalModelStatus{}, err
	}
	modelPath := filepath.Join(dir, localModelCacheName(spec))
	status := LocalModelStatus{
		Spec:     spec,
		CacheDir: dir,
		Path:     modelPath,
	}
	if st, err := os.Stat(modelPath); err == nil && !st.IsDir() {
		status.Exists = true
		status.SizeBytes = st.Size()
	}
	return status, nil
}

func ParseLocalModelSpec(model string) (LocalModelSpec, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		model = DefaultLocalModel
	}
	if strings.HasPrefix(strings.ToLower(model), "local:") {
		model = strings.TrimSpace(model[len("local:"):])
	}
	if strings.HasPrefix(strings.ToLower(model), "hf:") {
		rest := strings.TrimSpace(model[len("hf:"):])
		parts := strings.Split(rest, "/")
		if len(parts) < 3 {
			return LocalModelSpec{}, fmt.Errorf("local Hugging Face model must be hf:<org>/<repo>/<file>")
		}
		repo := strings.Join(parts[:len(parts)-1], "/")
		filename := parts[len(parts)-1]
		return LocalModelSpec{
			ID:       "hf:" + repo + "/" + filename,
			Repo:     repo,
			Filename: filename,
			URL:      "https://huggingface.co/" + repo + "/resolve/main/" + filename,
		}, nil
	}
	if strings.HasPrefix(model, "http://") || strings.HasPrefix(model, "https://") {
		filename := pathBaseURL(model)
		if filename == "" {
			return LocalModelSpec{}, fmt.Errorf("local model URL must include a filename")
		}
		return LocalModelSpec{ID: model, Filename: filename, URL: model}, nil
	}
	return LocalModelSpec{}, fmt.Errorf("unsupported local model %q (expected hf:<org>/<repo>/<file> or https://...)", model)
}

func localModelCacheDir(cacheDir string) (string, error) {
	cacheDir = strings.TrimSpace(cacheDir)
	if cacheDir == "" {
		if fromEnv := strings.TrimSpace(os.Getenv("AFS_EMBED_MODEL_DIR")); fromEnv != "" {
			cacheDir = fromEnv
		}
	}
	if strings.HasPrefix(cacheDir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		cacheDir = filepath.Join(home, cacheDir[2:])
	}
	if cacheDir == "" {
		userCache, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		cacheDir = filepath.Join(userCache, "afs", "models")
	}
	return filepath.Abs(cacheDir)
}

func localModelCacheName(spec LocalModelSpec) string {
	if spec.Repo == "" {
		return spec.Filename
	}
	return "hf_" + strings.NewReplacer("/", "_", ":", "_").Replace(spec.Repo+"_"+spec.Filename)
}

func pathBaseURL(value string) string {
	if idx := strings.IndexAny(value, "?#"); idx >= 0 {
		value = value[:idx]
	}
	value = strings.TrimRight(value, "/")
	idx := strings.LastIndex(value, "/")
	if idx < 0 {
		return value
	}
	return value[idx+1:]
}
