package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/redis/agent-filesystem/internal/controlplane"
	"github.com/redis/agent-filesystem/internal/uistatic"
)

const (
	configPathEnvVar  = "AFS_CONFIG_PATH"
	allowOriginEnvVar = "AFS_ALLOW_ORIGIN"
)

func main() {
	listenAddr := defaultListenAddr()
	configPath := strings.TrimSpace(os.Getenv(configPathEnvVar))
	allowOrigin := strings.TrimSpace(os.Getenv(allowOriginEnvVar))
	if allowOrigin == "" {
		allowOrigin = "*"
	}

	// The Vercel build is the cloud control plane. Let install.sh and the UI
	// runtime config report this unless an operator explicitly overrides it
	// (e.g. for a preview deployment impersonating self-hosted).
	if strings.TrimSpace(os.Getenv(controlplane.ProductModeEnvVar)) == "" {
		_ = os.Setenv(controlplane.ProductModeEnvVar, controlplane.ProductModeCloud)
	}

	// Unpack the embedded CLI bundle (populated at deploy time by prod.sh) and
	// point the control plane at the extracted directory so /v1/cli can serve
	// the matching binary on Vercel, where the project filesystem doesn't
	// include non-Go artifacts by default. The embed is the source of truth
	// when it exists, so it overrides any pre-set AFS_CLI_ARTIFACT_DIR.
	if dir, err := extractCLIBundle(); err != nil {
		fmt.Fprintln(os.Stderr, "warn: extract CLI bundle:", err)
	} else if dir != "" {
		_ = os.Setenv("AFS_CLI_ARTIFACT_DIR", dir)
		fmt.Fprintln(os.Stderr, "extracted CLI bundle to", dir)
	} else {
		fmt.Fprintln(os.Stderr, "no embedded CLI bundle; /v1/cli will use normal resolver")
	}

	auth, err := controlplane.LoadAuthHandlerFromEnv()
	if err != nil {
		fatal(err)
	}

	manager, err := controlplane.OpenDatabaseManager(configPath)
	var handler http.Handler
	if err != nil {
		fmt.Fprintln(os.Stderr, "warn: catalog unavailable; starting degraded handler:", err)
		handler = newDegradedCatalogHandler(allowOrigin)
	} else {
		defer manager.Close()
		handler = controlplane.NewHandlerWithOptions(manager, controlplane.HandlerOptions{AllowOrigin: allowOrigin, Auth: auth})
	}

	server := &http.Server{
		Addr:              listenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	fmt.Fprintf(os.Stderr, "AFS control plane listening on http://%s\n", listenAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fatal(err)
	}
}

func defaultListenAddr() string {
	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
		return ":" + port
	}
	return "127.0.0.1:8091"
}

func newDegradedCatalogHandler(allowOrigin string) http.Handler {
	uiFS, err := fs.Sub(uistatic.Content, "dist")
	if err != nil {
		return degradedCORS(http.HandlerFunc(writeCatalogUnavailable), allowOrigin)
	}
	if _, err := fs.Stat(uiFS, "index.html"); err != nil {
		return degradedCORS(http.HandlerFunc(writeCatalogUnavailable), allowOrigin)
	}

	fileServer := http.FileServer(http.FS(uiFS))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isControlPlanePath(r.URL.Path) {
			writeCatalogUnavailable(w, r)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(uiFS, path); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		clone := r.Clone(r.Context())
		clone.URL.Path = "/"
		fileServer.ServeHTTP(w, clone)
	})
	return degradedCORS(handler, allowOrigin)
}

func isControlPlanePath(path string) bool {
	return strings.HasPrefix(path, "/v1/") ||
		strings.HasPrefix(path, "/v2/") ||
		path == "/healthz" ||
		path == "/install.sh" ||
		path == "/mcp"
}

func writeCatalogUnavailable(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":     false,
		"error":  "catalog unavailable",
		"detail": "AFS Cloud is temporarily unable to reach its catalog database.",
	})
}

func degradedCORS(next http.Handler, allowOrigin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if allowOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		}
		next.ServeHTTP(w, r)
	})
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
