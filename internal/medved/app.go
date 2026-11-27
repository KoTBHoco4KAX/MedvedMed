package medved

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

	"medved-triage/internal/web"
)

type app struct {
	cfg     config
	store   *store
	client  *http.Client // Core API, 10 c
	llm     *http.Client // DeepSeek, 20 c
	static  http.Handler
	started time.Time
}

// Run загружает конфигурацию, поднимает HTTP-сервер и ждёт SIGINT/SIGTERM.
// Это единственная точка входа пакета: детали HTTP и FSM остаются внутри.
func Run() {
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
		static:  web.SPAHandler(cfg.StaticDir),
		started: time.Now(),
	}

	mux := newMux(a)
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           web.LogRequests(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("МедВед: сервер слушает http://127.0.0.1:%s (DEV_MODE=%t, БД: %s, статика: %s, public: %s)",
			cfg.Port, cfg.DevMode, cfg.DBPath, cfg.StaticDir, cfg.PublicURL)
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

func newMux(a *app) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", a.handleHealth)
	mux.HandleFunc("/api/auth", a.handleAuth)
	mux.HandleFunc("/api/me", a.handleMe)
	mux.HandleFunc("/api/chat/state", a.handleChatState)
	mux.HandleFunc("/api/chat/answer", a.handleChatAnswer)
	mux.HandleFunc("/api/chat/back", a.handleChatBack)
	mux.HandleFunc("/api/chat/reset", a.handleChatReset)
	mux.HandleFunc("/api/history", a.handleHistory)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSONError(w, http.StatusNotFound, "неизвестный метод API")
	})
	mux.Handle("/", a.static)
	return mux
}
