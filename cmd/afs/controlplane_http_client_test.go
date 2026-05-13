package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/redis/agent-filesystem/internal/controlplane"
	"github.com/redis/agent-filesystem/internal/mcptools"
)

func TestHTTPControlPlaneClientSessionPathUsesScopedDatabase(t *testing.T) {
	t.Helper()

	client := &httpControlPlaneClient{databaseID: "db_123"}
	got := client.clientWorkspaceSessionPath("ws_456", "sessions", "sess_789", "heartbeat")
	want := "/v1/client/databases/db_123/workspaces/ws_456/sessions/sess_789/heartbeat"
	if got != want {
		t.Fatalf("clientWorkspaceSessionPath = %q, want %q", got, want)
	}
}

func TestHTTPControlPlaneClientSessionPathFallsBackToWorkspaceRoute(t *testing.T) {
	t.Helper()

	client := &httpControlPlaneClient{}
	got := client.clientWorkspaceSessionPath("getting-started", "sessions", "sess_789")
	want := "/v1/client/workspaces/getting-started/sessions/sess_789"
	if got != want {
		t.Fatalf("clientWorkspaceSessionPath = %q, want %q", got, want)
	}
}

func TestHTTPControlPlaneClientAddWorkspaceCompositionMountAcceptsCreated(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v2/workspaces/ws_agent/mounts" {
			t.Fatalf("path = %q, want workspace composition mounts route", r.URL.Path)
		}
		var req controlplane.WorkspaceCompositionMount
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Decode(request) returned error: %v", err)
		}
		if req.VolumeID != "ws_volume" || req.MountPath != "/repo" {
			t.Fatalf("request = %+v, want selected volume at /repo", req)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(controlplane.WorkspaceCompositionDetail{
			ID:   "ws_agent",
			Name: "coding-agent",
			Mounts: []controlplane.WorkspaceCompositionMount{{
				VolumeID:   "ws_volume",
				VolumeName: "repo",
				MountPath:  "/repo",
			}},
		})
	}))
	defer server.Close()

	client := &httpControlPlaneClient{
		baseURL: server.URL,
		client:  &http.Client{Timeout: time.Minute},
	}
	detail, err := client.AddWorkspaceCompositionMount(context.Background(), "ws_agent", controlplane.WorkspaceCompositionMount{
		VolumeID:  "ws_volume",
		MountPath: "/repo",
	})
	if err != nil {
		t.Fatalf("AddWorkspaceCompositionMount() returned error: %v", err)
	}
	if detail.Name != "coding-agent" || len(detail.Mounts) != 1 || detail.Mounts[0].VolumeID != "ws_volume" {
		t.Fatalf("detail = %+v, want attached volume response", detail)
	}
}

func TestHTTPControlPlaneClientQueryUsesLongRunningClient(t *testing.T) {
	t.Helper()

	queryClientUsed := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/workspaces/ws_123/query" {
			t.Fatalf("path = %q, want query route", r.URL.Path)
		}
		queryClientUsed = true
		_ = json.NewEncoder(w).Encode(mcptools.FileQueryResponse{Status: mcptools.FileQueryStatusOK})
	}))
	defer server.Close()

	client := &httpControlPlaneClient{
		baseURL: server.URL,
		client:  &http.Client{Timeout: time.Nanosecond},
		queryer: &http.Client{Timeout: time.Minute},
	}
	if _, err := client.QueryWorkspace(context.Background(), "ws_123", mcptools.FileQueryRequest{Query: "help"}); err != nil {
		t.Fatalf("QueryWorkspace() returned error: %v", err)
	}
	if !queryClientUsed {
		t.Fatal("query route was not called")
	}
}

func TestHTTPControlPlaneClientQueryIndexRebuildUsesLongRunningClient(t *testing.T) {
	t.Helper()

	queryClientUsed := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/workspaces/ws_123/query/index/rebuild" {
			t.Fatalf("path = %q, want query index rebuild route", r.URL.Path)
		}
		queryClientUsed = true
		_ = json.NewEncoder(w).Encode(controlplane.WorkspaceQueryIndexRebuildResponse{Workspace: "repo"})
	}))
	defer server.Close()

	client := &httpControlPlaneClient{
		baseURL: server.URL,
		client:  &http.Client{Timeout: time.Nanosecond},
		queryer: &http.Client{Timeout: time.Minute},
	}
	if _, err := client.RebuildQueryIndex(context.Background(), "ws_123", controlplane.WorkspaceQueryIndexRebuildRequest{Wait: true, Embeddings: true}); err != nil {
		t.Fatalf("RebuildQueryIndex() returned error: %v", err)
	}
	if !queryClientUsed {
		t.Fatal("query index rebuild route was not called")
	}
}

func TestHTTPControlPlaneClientQueryModelRoutes(t *testing.T) {
	t.Helper()

	queryClientUsed := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/query/model/status":
			if r.Method != http.MethodGet {
				t.Fatalf("status method = %s, want GET", r.Method)
			}
			if r.URL.Query().Get("model") != "hf:org/repo/model.gguf" {
				t.Fatalf("status model = %q", r.URL.Query().Get("model"))
			}
			_ = json.NewEncoder(w).Encode(controlplane.QueryModelStatus{})
		case "/v1/query/model/download":
			if r.Method != http.MethodPost {
				t.Fatalf("download method = %s, want POST", r.Method)
			}
			queryClientUsed = true
			var req controlplane.QueryModelDownloadRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("Decode(download request) returned error: %v", err)
			}
			if req.Model != "hf:org/repo/model.gguf" {
				t.Fatalf("download model = %q", req.Model)
			}
			_ = json.NewEncoder(w).Encode(controlplane.QueryModelDownloadResult{})
		default:
			t.Fatalf("path = %q, want query model route", r.URL.Path)
		}
	}))
	defer server.Close()

	client := &httpControlPlaneClient{
		baseURL: server.URL,
		client:  &http.Client{Timeout: time.Minute},
		queryer: &http.Client{Timeout: time.Minute},
	}
	if _, err := client.QueryModelStatus(context.Background(), controlplane.QueryModelStatusRequest{Model: "hf:org/repo/model.gguf"}); err != nil {
		t.Fatalf("QueryModelStatus() returned error: %v", err)
	}
	if _, err := client.DownloadQueryModel(context.Background(), controlplane.QueryModelDownloadRequest{Model: "hf:org/repo/model.gguf"}); err != nil {
		t.Fatalf("DownloadQueryModel() returned error: %v", err)
	}
	if !queryClientUsed {
		t.Fatal("query model download route was not called with long-running client")
	}
}
