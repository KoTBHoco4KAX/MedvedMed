package config

import "testing"

func TestResolvePublicURL(t *testing.T) {
	t.Setenv("PUBLIC_URL", "https://medved.example.ru/")
	t.Setenv("TUNA_DOMAIN", "ignored.example.ru")
	if got := ResolvePublicURL("8080"); got != "https://medved.example.ru" {
		t.Fatalf("PUBLIC_URL: %q", got)
	}

	t.Setenv("PUBLIC_URL", "")
	t.Setenv("TUNA_DOMAIN", "medved.yourdomain.ru")
	if got := ResolvePublicURL("8080"); got != "https://medved.yourdomain.ru" {
		t.Fatalf("TUNA_DOMAIN: %q", got)
	}

	t.Setenv("TUNA_DOMAIN", "")
	if got := ResolvePublicURL("9090"); got != "http://127.0.0.1:9090" {
		t.Fatalf("fallback: %q", got)
	}
}
