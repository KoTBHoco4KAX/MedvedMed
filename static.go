package main

import (
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// spaHandler раздаёт собранный фронтенд из dist и делает SPA-fallback на index.html.
// Важно: X-Frame-Options не выставляется — мини-апп MAX открывает сервис в iframe.
func spaHandler(dist string) http.Handler {
	dist = filepath.Clean(dist)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		clean := path.Clean("/" + r.URL.Path)
		rel := strings.TrimPrefix(clean, "/")
		if rel != "" {
			full := filepath.Join(dist, filepath.FromSlash(rel))
			if info, err := os.Stat(full); err == nil && !info.IsDir() {
				if strings.HasPrefix(rel, "assets/") {
					// Vite складывает сюда файлы с хешем в имени — можно кешировать надолго.
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				http.ServeFile(w, r, full)
				return
			}
		}

		index := filepath.Join(dist, "index.html")
		if _, err := os.Stat(index); err != nil {
			log.Printf("статика не найдена: %s (соберите фронтенд: cd frontend && pnpm build)", index)
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("Фронтенд не собран. Выполните: cd frontend && pnpm install && pnpm build"))
			return
		}

		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, index)
	})
}
