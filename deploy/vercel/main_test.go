package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDefaultListenAddrUsesPortEnv(t *testing.T) {
	t.Setenv("PORT", "3000")

	if got := defaultListenAddr(); got != ":3000" {
		t.Fatalf("defaultListenAddr() = %q, want :3000", got)
	}
}

func TestDefaultListenAddrFallsBackToLocalhost(t *testing.T) {
	t.Setenv("PORT", "")

	if got := defaultListenAddr(); got != "127.0.0.1:8091" {
		t.Fatalf("defaultListenAddr() = %q, want 127.0.0.1:8091", got)
	}
}

func TestDegradedCatalogHandlerReturnsServiceUnavailableForAPI(t *testing.T) {
	handler := newDegradedCatalogHandler("*")
	req := httptest.NewRequest(http.MethodGet, "/v1/catalog/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want *", got)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("degraded response is not JSON: %v", err)
	}
	if body["ok"] != false {
		t.Fatalf("ok = %v, want false", body["ok"])
	}
	if body["error"] != "catalog unavailable" {
		t.Fatalf("error = %v, want catalog unavailable", body["error"])
	}
}
