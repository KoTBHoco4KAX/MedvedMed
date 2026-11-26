package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolvePublicURL(t *testing.T) {
	t.Setenv("PUBLIC_URL", "https://medved.example.ru/")
	t.Setenv("TUNA_DOMAIN", "ignored.example.ru")
	if got := resolvePublicURL("8080"); got != "https://medved.example.ru" {
		t.Fatalf("PUBLIC_URL: %q", got)
	}

	t.Setenv("PUBLIC_URL", "")
	t.Setenv("TUNA_DOMAIN", "medved.yourdomain.ru")
	if got := resolvePublicURL("8080"); got != "https://medved.yourdomain.ru" {
		t.Fatalf("TUNA_DOMAIN: %q", got)
	}

	t.Setenv("TUNA_DOMAIN", "")
	if got := resolvePublicURL("9090"); got != "http://127.0.0.1:9090" {
		t.Fatalf("fallback: %q", got)
	}
}

func TestHealthEndpoint(t *testing.T) {
	a := &app{}
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	a.handleHealth(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("health status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); body != "{\"status\":\"ok\"}\n" {
		t.Fatalf("health body = %q", body)
	}
}
