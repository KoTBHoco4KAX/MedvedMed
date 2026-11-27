package medved

import configpkg "medved-triage/internal/config"

// config — алиас, чтобы существующие тесты и app не зависели от имени пакета.
type config = configpkg.Config

func loadConfig() config {
	return configpkg.Load()
}

func resolvePublicURL(port string) string {
	return configpkg.ResolvePublicURL(port)
}
