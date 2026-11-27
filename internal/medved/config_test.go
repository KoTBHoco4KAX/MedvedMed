package medved

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestDevModeUsesMockWomanEvenWithRealToken(t *testing.T) {
	st, err := openStore(filepath.Join(t.TempDir(), "dev-auth.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.close()

	a := &app{store: st, cfg: config{DevMode: true}}
	req := httptest.NewRequest(http.MethodPost, "/api/auth", strings.NewReader(`{"token":"real-max-token"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	a.handleAuth(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("auth status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var response authResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("auth JSON: %v", err)
	}
	if response.Sex != devUserSex || response.Age != devUserAge {
		t.Fatalf("dev profile = age %d sex %d", response.Age, response.Sex)
	}

	sess, err := st.loadSession(response.SessionToken)
	if err != nil {
		t.Fatalf("loadSession: %v", err)
	}
	if sess.CoreUserID != devUserID || sess.Sex != 2 {
		t.Fatalf("session = user %q age %d sex %d", sess.CoreUserID, sess.Age, sess.Sex)
	}
	if _, ok := findStep(sess.steps(), stepPregnancy); !ok {
		t.Fatal("для dev-женщины 30 лет должен быть шаг беременности/лактации")
	}
}
