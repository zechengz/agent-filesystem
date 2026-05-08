package controlplane

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"
)

func TestCloudAdminAllowlistAuthorizesAdminEndpoints(t *testing.T) {
	t.Setenv(ProductModeEnvVar, ProductModeCloud)
	t.Setenv(authAdminSubjectsEnvVar, "admin-user")

	manager, databaseID := newTestManager(t)
	createOwnedTestWorkspace(t, manager, databaseID, "alice-user", "Alice", 1, "alice-repo")
	createOwnedTestWorkspace(t, manager, databaseID, "bob-user", "Bob", 2, "bob-repo")

	auth := newTrustedHeaderTestAuth(t)
	server := httptest.NewServer(NewHandlerWithOptions(manager, HandlerOptions{
		AllowOrigin: "*",
		Auth:        auth,
	}))
	defer server.Close()

	configReq, err := http.NewRequest(http.MethodGet, server.URL+"/v1/auth/config", nil)
	if err != nil {
		t.Fatalf("NewRequest(auth config) returned error: %v", err)
	}
	configReq.Header.Set("X-Forwarded-User", "admin-user")
	configResp, err := http.DefaultClient.Do(configReq)
	if err != nil {
		t.Fatalf("GET /v1/auth/config returned error: %v", err)
	}
	defer configResp.Body.Close()
	var config authRuntimeConfigResponse
	if err := json.NewDecoder(configResp.Body).Decode(&config); err != nil {
		t.Fatalf("Decode(auth config) returned error: %v", err)
	}
	if config.User == nil || !config.User.IsAdmin {
		t.Fatalf("auth config user = %#v, want admin", config.User)
	}

	for _, path := range []string{
		"/v1/admin/overview",
		"/v1/admin/users",
		"/v1/admin/databases",
		"/v1/admin/workspaces",
		"/v1/admin/agents",
	} {
		req, err := http.NewRequest(http.MethodGet, server.URL+path, nil)
		if err != nil {
			t.Fatalf("NewRequest(%s) returned error: %v", path, err)
		}
		req.Header.Set("X-Forwarded-User", "admin-user")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET %s returned error: %v", path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s status = %d, want %d, body=%s", path, resp.StatusCode, http.StatusOK, body)
		}
	}

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/admin/users", nil)
	if err != nil {
		t.Fatalf("NewRequest(admin users) returned error: %v", err)
	}
	req.Header.Set("X-Forwarded-User", "admin-user")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /v1/admin/users returned error: %v", err)
	}
	defer resp.Body.Close()
	var users adminUserListResponse
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		t.Fatalf("Decode(admin users) returned error: %v", err)
	}
	if !adminUsersContain(users.Items, "alice-user") || !adminUsersContain(users.Items, "bob-user") {
		t.Fatalf("admin users = %#v, want alice-user and bob-user", users.Items)
	}
}

func TestCloudAdminEndpointsRejectNonAdmin(t *testing.T) {
	t.Setenv(ProductModeEnvVar, ProductModeCloud)
	t.Setenv(authAdminSubjectsEnvVar, "admin-user")

	manager, _ := newTestManager(t)
	auth := newTrustedHeaderTestAuth(t)
	server := httptest.NewServer(NewHandlerWithOptions(manager, HandlerOptions{
		AllowOrigin: "*",
		Auth:        auth,
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/admin/overview", nil)
	if err != nil {
		t.Fatalf("NewRequest(admin overview) returned error: %v", err)
	}
	req.Header.Set("X-Forwarded-User", "regular-user")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /v1/admin/overview returned error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET /v1/admin/overview status = %d, want %d, body=%s", resp.StatusCode, http.StatusForbidden, body)
	}

	configReq, err := http.NewRequest(http.MethodGet, server.URL+"/v1/auth/config", nil)
	if err != nil {
		t.Fatalf("NewRequest(auth config) returned error: %v", err)
	}
	configReq.Header.Set("X-Forwarded-User", "regular-user")
	configResp, err := http.DefaultClient.Do(configReq)
	if err != nil {
		t.Fatalf("GET /v1/auth/config returned error: %v", err)
	}
	defer configResp.Body.Close()
	var config authRuntimeConfigResponse
	if err := json.NewDecoder(configResp.Body).Decode(&config); err != nil {
		t.Fatalf("Decode(auth config) returned error: %v", err)
	}
	if config.User != nil && config.User.IsAdmin {
		t.Fatalf("regular user auth config = %#v, want non-admin", config.User)
	}
}

func TestCloudQueryModelDownloadRequiresAdminAndAllowsAdminCLIToken(t *testing.T) {
	t.Setenv(ProductModeEnvVar, ProductModeCloud)
	t.Setenv(authAdminSubjectsEnvVar, "admin-user")
	cacheDir := t.TempDir()
	t.Setenv("AFS_EMBED_MODEL_DIR", cacheDir)

	payload := []byte("tiny gguf")
	modelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/model.gguf" {
			t.Fatalf("model path = %q, want /model.gguf", r.URL.Path)
		}
		_, _ = w.Write(payload)
	}))
	defer modelServer.Close()
	fakeModelPath := cacheDir + "/fake-model.gguf"
	if err := os.WriteFile(fakeModelPath, payload, 0o644); err != nil {
		t.Fatalf("WriteFile(fake model) returned error: %v", err)
	}
	helperPath := cacheDir + "/fake-helper"
	helperScript := `#!/bin/sh
while IFS= read -r line; do
  case "$line" in
    *'"op":"ready"'*) printf '{"id":0,"path":"%s"}\n' "$FAKE_EMBED_MODEL_PATH" ;;
    *) printf '%s\n' '{"id":1,"vectors":[[1,0,0]]}' ;;
  esac
done
`
	if err := os.WriteFile(helperPath, []byte(helperScript), 0o755); err != nil {
		t.Fatalf("WriteFile(fake helper) returned error: %v", err)
	}
	t.Setenv("AFS_EMBED_HELPER_CMD", helperPath)
	t.Setenv("AFS_EMBED_DIMENSIONS", "3")
	t.Setenv("FAKE_EMBED_MODEL_PATH", fakeModelPath)

	manager, databaseID := newTestManager(t)
	adminCtx := context.WithValue(context.Background(), authIdentityContextKey, AuthIdentity{
		Subject:  "admin-user",
		Name:     "Admin",
		Provider: string(AuthModeTrustedHeader),
	})
	adminWorkspace, err := manager.CreateWorkspace(adminCtx, databaseID, createWorkspaceRequest{
		Name:   "admin-repo",
		Source: sourceRef{Kind: SourceBlank},
	})
	if err != nil {
		t.Fatalf("CreateWorkspace(admin) returned error: %v", err)
	}
	adminToken, err := manager.createCLIAccessTokenRecord(context.Background(), "admin-user", "Admin", databaseID, adminWorkspace.ID, adminWorkspace.Name)
	if err != nil {
		t.Fatalf("createCLIAccessTokenRecord(admin) returned error: %v", err)
	}
	auth := newTrustedHeaderTestAuth(t)
	server := httptest.NewServer(NewHandlerWithOptions(manager, HandlerOptions{
		AllowOrigin: "*",
		Auth:        auth,
	}))
	defer server.Close()

	statusReq, err := http.NewRequest(http.MethodGet, server.URL+"/v1/query/model/status?model="+url.QueryEscape(modelServer.URL+"/model.gguf"), nil)
	if err != nil {
		t.Fatalf("NewRequest(status) returned error: %v", err)
	}
	statusReq.Header.Set("X-Forwarded-User", "regular-user")
	statusResp, err := http.DefaultClient.Do(statusReq)
	if err != nil {
		t.Fatalf("GET query model status returned error: %v", err)
	}
	_ = statusResp.Body.Close()
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("GET query model status = %d, want %d", statusResp.StatusCode, http.StatusOK)
	}

	body := []byte(`{"model":` + strconv.Quote(modelServer.URL+"/model.gguf") + `}`)
	regularReq, err := http.NewRequest(http.MethodPost, server.URL+"/v1/query/model/download", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest(regular download) returned error: %v", err)
	}
	regularReq.Header.Set("X-Forwarded-User", "regular-user")
	regularResp, err := http.DefaultClient.Do(regularReq)
	if err != nil {
		t.Fatalf("POST regular query model download returned error: %v", err)
	}
	_ = regularResp.Body.Close()
	if regularResp.StatusCode != http.StatusForbidden {
		t.Fatalf("POST regular query model download = %d, want %d", regularResp.StatusCode, http.StatusForbidden)
	}

	adminReq, err := http.NewRequest(http.MethodPost, server.URL+"/v1/query/model/download", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest(admin download) returned error: %v", err)
	}
	adminReq.Header.Set("Authorization", "Bearer "+adminToken)
	adminResp, err := http.DefaultClient.Do(adminReq)
	if err != nil {
		t.Fatalf("POST admin query model download returned error: %v", err)
	}
	defer adminResp.Body.Close()
	if adminResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(adminResp.Body)
		t.Fatalf("POST admin query model download = %d, want %d, body=%s", adminResp.StatusCode, http.StatusOK, body)
	}
	var result QueryModelDownloadResult
	if err := json.NewDecoder(adminResp.Body).Decode(&result); err != nil {
		t.Fatalf("Decode(query model download) returned error: %v", err)
	}
	if !result.Exists || result.Path != fakeModelPath {
		t.Fatalf("download result = %+v, want helper-resolved cached model", result)
	}
}

func TestSelfHostedIgnoresAdminAllowlist(t *testing.T) {
	t.Setenv(ProductModeEnvVar, ProductModeSelfHosted)
	t.Setenv(authAdminSubjectsEnvVar, "admin-user")

	manager, _ := newTestManager(t)
	auth := newTrustedHeaderTestAuth(t)
	server := httptest.NewServer(NewHandlerWithOptions(manager, HandlerOptions{
		AllowOrigin: "*",
		Auth:        auth,
	}))
	defer server.Close()

	configReq, err := http.NewRequest(http.MethodGet, server.URL+"/v1/auth/config", nil)
	if err != nil {
		t.Fatalf("NewRequest(auth config) returned error: %v", err)
	}
	configReq.Header.Set("X-Forwarded-User", "admin-user")
	configResp, err := http.DefaultClient.Do(configReq)
	if err != nil {
		t.Fatalf("GET /v1/auth/config returned error: %v", err)
	}
	defer configResp.Body.Close()
	var config authRuntimeConfigResponse
	if err := json.NewDecoder(configResp.Body).Decode(&config); err != nil {
		t.Fatalf("Decode(auth config) returned error: %v", err)
	}
	if config.User != nil && config.User.IsAdmin {
		t.Fatalf("self-hosted auth config = %#v, want non-admin", config.User)
	}

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/admin/overview", nil)
	if err != nil {
		t.Fatalf("NewRequest(admin overview) returned error: %v", err)
	}
	req.Header.Set("X-Forwarded-User", "admin-user")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /v1/admin/overview returned error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET /v1/admin/overview status = %d, want %d, body=%s", resp.StatusCode, http.StatusForbidden, body)
	}
}

func TestAdminEndpointsDoNotChangeUserScopedLists(t *testing.T) {
	t.Setenv(ProductModeEnvVar, ProductModeCloud)
	t.Setenv(authAdminSubjectsEnvVar, "admin-user")

	manager, databaseID := newTestManager(t)
	createOwnedTestWorkspace(t, manager, databaseID, "alice-user", "Alice", 1, "alice-repo")
	createOwnedTestWorkspace(t, manager, databaseID, "bob-user", "Bob", 2, "bob-repo")

	auth := newTrustedHeaderTestAuth(t)
	server := httptest.NewServer(NewHandlerWithOptions(manager, HandlerOptions{
		AllowOrigin: "*",
		Auth:        auth,
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/workspaces", nil)
	if err != nil {
		t.Fatalf("NewRequest(workspaces) returned error: %v", err)
	}
	req.Header.Set("X-Forwarded-User", "alice-user")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /v1/workspaces returned error: %v", err)
	}
	defer resp.Body.Close()
	var workspaces workspaceListResponse
	if err := json.NewDecoder(resp.Body).Decode(&workspaces); err != nil {
		t.Fatalf("Decode(workspaces) returned error: %v", err)
	}
	if !workspaceListContains(workspaces.Items, "alice-repo") {
		t.Fatalf("alice scoped workspaces = %#v, want alice-repo", workspaces.Items)
	}
	if workspaceListContains(workspaces.Items, "bob-repo") {
		t.Fatalf("alice scoped workspaces = %#v, must not include bob-repo", workspaces.Items)
	}
}

func newTrustedHeaderTestAuth(t *testing.T) *AuthHandler {
	t.Helper()
	auth, err := NewAuthHandler(AuthConfig{
		Mode:              AuthModeTrustedHeader,
		TrustedUserHeader: "X-Forwarded-User",
		TrustedNameHeader: "X-Forwarded-Name",
	})
	if err != nil {
		t.Fatalf("NewAuthHandler() returned error: %v", err)
	}
	return auth
}

func createOwnedTestWorkspace(t *testing.T, manager *DatabaseManager, baseDatabaseID, subject, label string, redisDB int, workspaceName string) {
	t.Helper()

	manager.mu.Lock()
	baseProfile := manager.profiles[baseDatabaseID]
	manager.mu.Unlock()

	ctx := context.WithValue(context.Background(), authIdentityContextKey, AuthIdentity{
		Subject:  subject,
		Name:     label,
		Provider: string(AuthModeTrustedHeader),
	})
	database, err := manager.UpsertDatabase(ctx, "", upsertDatabaseRequest{
		Name:      label + " DB",
		RedisAddr: baseProfile.RedisAddr,
		RedisDB:   redisDB,
	})
	if err != nil {
		t.Fatalf("UpsertDatabase(%s) returned error: %v", subject, err)
	}
	if _, err := manager.CreateWorkspace(ctx, database.ID, createWorkspaceRequest{
		Name:   workspaceName,
		Source: sourceRef{Kind: SourceBlank},
	}); err != nil {
		t.Fatalf("CreateWorkspace(%s) returned error: %v", workspaceName, err)
	}
}

func adminUsersContain(items []adminUserRecord, subject string) bool {
	for _, item := range items {
		if item.Subject == subject {
			return true
		}
	}
	return false
}

func workspaceListContains(items []workspaceSummary, name string) bool {
	for _, item := range items {
		if item.Name == name {
			return true
		}
	}
	return false
}
