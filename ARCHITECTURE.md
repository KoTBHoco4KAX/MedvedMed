# Архитектура МедВеда

## Структура

```text
.
├── main.go                     # тонкая точка входа: go run . / go build .
├── internal/
│   ├── config/                 # чтение .env, PUBLIC_URL, TUNA_DOMAIN
│   ├── web/                    # HTTP-адаптеры: статика и логирование
│   └── medved/
│       ├── app.go              # composition root: БД, HTTP-сервер, запуск
│       ├── session.go          # модель сессии и сообщений
│       ├── store.go            # SQLite: сессии и консультации
│       ├── fsm.go              # опросник, шаги, переходы
│       ├── emergency.go        # красные флаги и их правила
│       ├── llm.go              # DeepSeek/локальная карточка
│       ├── clarification.go    # AI-уточняющий вопрос
│       ├── core_client.go      # клиент MAX Core
│       ├── http_handlers.go    # HTTP API
│       ├── chat_handlers.go    # чат-эндпоинты
│       └── *_test.go
├── frontend/src/
│   ├── components/
│   │   ├── chat/               # Composer, MessageBubble
│   │   ├── layout/             # Header, прогресс
│   │   ├── modals/             # карта тела, экстренная памятка
│   │   └── result/             # ResultCard, HistoryPanel, EmergencyCard
│   ├── lib/                    # форматирование/эмодзи
│   ├── api.ts
│   └── types.ts
├── scripts/
├── Dockerfile
└── docker-compose.yml
```

## Принципы

- **SRP.** Конфигурация, HTTP-адаптеры, хранение, FSM, LLM и red flags разнесены по отдельным файлам/пакетам; компоненты интерфейса сгруппированы по роли.
- **OCP.** Новые red flags добавляются в реестр правил и `optionRedFlags`, новые шаги — в `stepsFor`/`stepSpec`, а не в обработчики HTTP.
- **DIP.** `Run()` — composition root; HTTP-слой зависит от `app`, а `app` получает конкретные адаптеры (store, Core, LLM) в одном месте. `internal/web` зависит только от `net/http`.
- **ISP.** `internal/web` предоставляет только нужные адаптеры (`SPAHandler`, `LogRequests`), не весь HTTP-слой.
- **Модульный монолит.** FSM, red flags и карточка пока живут в одном пакете `internal/medved`: у них общая транзакционная модель сессии и активные циклические зависимости. Следующий безопасный шаг — выделить `domain`-типы и затем `store`, `emergency`, `llm` в отдельные пакеты.

## Запуск

```powershell
go run .
```

Точка входа осталась прежней; `Dockerfile` собирает корневой пакет и копирует `internal/`.
