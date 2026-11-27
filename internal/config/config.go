package config

import (
	"os"
	"strings"
)

type Config struct {
	Port             string
	DevMode          bool
	CoreURL          string
	CoreServiceToken string
	StaticDir        string
	DBPath           string
	DeepSeekAPIKey   string
	PublicURL        string
}

func Load() Config {
	port := envOr("PORT", "8080")
	return Config{
		Port:             port,
		DevMode:          isTrue(os.Getenv("DEV_MODE")),
		CoreURL:          trimTrailingSlash(os.Getenv("CORE_URL")),
		CoreServiceToken: os.Getenv("CORE_SERVICE_TOKEN"),
		StaticDir:        envOr("STATIC_DIR", "./frontend/dist"),
		DBPath:           envOr("DB_PATH", "./medbot.db"),
		DeepSeekAPIKey:   os.Getenv("DEEPSEEK_API_KEY"),
		PublicURL:        ResolvePublicURL(port),
	}
}

// ResolvePublicURL берёт PUBLIC_URL, иначе строит адрес из TUNA_DOMAIN.
// Никаких доменов в коде: для локального запуска остаётся localhost.
func ResolvePublicURL(port string) string {
	if value := trimTrailingSlash(strings.TrimSpace(os.Getenv("PUBLIC_URL"))); value != "" {
		return value
	}
	domain := trimTrailingSlash(strings.TrimSpace(os.Getenv("TUNA_DOMAIN")))
	if domain != "" {
		if strings.HasPrefix(domain, "http://") || strings.HasPrefix(domain, "https://") {
			return domain
		}
		return "https://" + domain
	}
	return "http://127.0.0.1:" + port
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func isTrue(value string) bool {
	return value == "true" || value == "TRUE" || value == "True" || value == "1"
}

func trimTrailingSlash(value string) string {
	for len(value) > 0 && value[len(value)-1] == '/' {
		value = value[:len(value)-1]
	}
	return value
}
