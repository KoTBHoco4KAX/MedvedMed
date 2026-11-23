package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

type config struct {
	Port             string
	DevMode          bool
	CoreURL          string
	CoreServiceToken string
	StaticDir        string
	DBPath           string
	DeepSeekAPIKey   string
}

type app struct {
	cfg     config
	store   *store
	client  *http.Client // Core API, 10 c
	llm     *http.Client // DeepSeek, 20 c
	static  http.Handler
	started time.Time
}

func main() {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("godotenv: %v — работаю с переменными окружения", err)
	}

	cfg := loadConfig()

	db, err := openStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("МедВед: БД: %v", err)
	}
	defer func() {
		if err := db.close(); err != nil {
			log.Printf("МедВед: закрыть БД: %v", err)
		}
	}()

	a := &app{
		cfg:     cfg,
		store:   db,
		client:  &http.Client{Timeout: 10 * time.Second},
		llm:     &http.Client{Timeout: llmTimeout},
		static:  spaHandler(cfg.StaticDir),
		started: time.Now(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth", a.handleAuth)
	mux.HandleFunc("/api/me", a.handleMe)
	mux.HandleFunc("/api/chat/state", a.handleChatState)
	mux.HandleFunc("/api/chat/answer", a.handleChatAnswer)
	mux.HandleFunc("/api/chat/back", a.handleChatBack)
	mux.HandleFunc("/api/chat/reset", a.handleChatReset)
	mux.HandleFunc("/api/history", a.handleHistory)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeJSONError(w, http.StatusNotFound, "неизвестный метод API")
	})
	mux.Handle("/", a.static)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           logRequests(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("МедВед: сервер слушает http://127.0.0.1:%s (DEV_MODE=%t, БД: %s, статика: %s)",
			cfg.Port, cfg.DevMode, cfg.DBPath, cfg.StaticDir)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("сервер остановлен: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("завершение: %v", err)
	}
	log.Println("МедВед: сервер остановлен")
}

func loadConfig() config {
	return config{
		Port:             envOr("PORT", "8080"),
		DevMode:          isTrue(os.Getenv("DEV_MODE")),
		CoreURL:          trimTrailingSlash(os.Getenv("CORE_URL")),
		CoreServiceToken: os.Getenv("CORE_SERVICE_TOKEN"),
		StaticDir:        envOr("STATIC_DIR", "./frontend/dist"),
		DBPath:           envOr("DB_PATH", "./medbot.db"),
		DeepSeekAPIKey:   os.Getenv("DEEPSEEK_API_KEY"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func isTrue(v string) bool {
	return v == "true" || v == "TRUE" || v == "True" || v == "1"
}

func trimTrailingSlash(v string) string {
	for len(v) > 0 && v[len(v)-1] == '/' {
		v = v[:len(v)-1]
	}
	return v
}

// logRequests логирует только метод, путь и статус: без query (в нём токен) и без ПДн.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Millisecond))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
