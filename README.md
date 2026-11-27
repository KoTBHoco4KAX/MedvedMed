# МедВед.Код2 — триаж-бот мини-аппа MAX

Go-бэкенд: авторизация через MAX Core, опрос о самочувствии (FSM), детектор
неотложных состояний, маршрутная карточка через DeepSeek с локальным фоллбэком,
SQLite-история консультаций. Фронтенд — Vite + React + TypeScript + Tailwind CSS.

Полное дерево шагов, условий ветвления и всех кнопок — в `DIALOG_TREE.md`.
Архитектурные решения и структура — в [ARCHITECTURE.md](ARCHITECTURE.md).

## Структура

```text
main.go                       тонкая точка входа (go run .)
internal/config/              .env, PUBLIC_URL, TUNA_DOMAIN
internal/web/                 статика и HTTP-логирование
internal/medved/app.go        composition root: БД, сервер, graceful shutdown
internal/medved/config_compat.go  алиас конфигурации и совместимость пакета
internal/medved/http_handlers.go  /api/auth, /api/me, /api/health
internal/medved/core_client.go    Core API: токен MAX -> пользователь
internal/medved/store.go          SQLite (modernc.org/sqlite, WAL)
internal/medved/session.go        модель сессии и ленты сообщений
internal/medved/emergency.go      детектор критических симптомов
internal/medved/fsm.go            шаги опроса, прогресс, ответ/возврат/сброс
internal/medved/llm.go            DeepSeek + локальный фоллбэк карточки
internal/medved/chat_handlers.go  чат-эндпоинты и история
frontend/src/components/chat/     Composer, MessageBubble
frontend/src/components/layout/   Header, прогресс
frontend/src/components/modals/    карта тела, экстренная памятка
frontend/src/components/result/    ResultCard, HistoryPanel, EmergencyCard
frontend/                     мобильный UI: Vite + React + TS + Tailwind
```

## Интерфейс

Мобильный экран под iframe MAX (`max-w-md mx-auto`, `h-[100dvh]`): белая шапка с названием,
приветствием и синей кнопкой «История», синий прогресс-бар «Шаг X из Y» с «← Назад»,
лента диалога (белые карточки бота, синие ответы справа, дисклеймер — информационная плашка),
нижняя панель с вариантами ответа и свободным вводом.

Палитра строгая: фон `#F5F6F8`, карточки и панели `#FFFFFF`, акцент `#0077FF`
(hover `#0066DD`). Без градиентов, теней, свечений и декоративных рамок:
разделение строится на контрасте белого и светло-серого. Тонкая граница остаётся только
у поля ввода, белых кнопок («Назад», «Начать заново») и красной рамки `2px #DC2626`
у карточки экстренного вызова 103/112.

Блок жалоб собран последовательно: категория → локализация → давность → интенсивность →
сопутствующие симптомы. При наличии аллергии открывается уточняющий шаг «На что именно
аллергия», а вредные привычки собираются циклом атомарных факторов риска (курить/алкоголь/
ПАВ/недосып/гиподинамия) с завершением по «На этом всё» — выбранное копится в истории.
Детектор красных флагов после каждого из шагов жалоб учитывает накопленный контекст, включая иррадиацию боли (левая ключица/плечо/рука, под левой лопаткой,
нижняя челюсть, эпигастрий) вместе с высокой интенсивностью, внезапным началом или
сопутствующей одышкой/неврологией — это риск острого коронарного синдрома.

На старте можно выбрать «Заполняю за ребёнка (до 18 лет)»: возраст ребёнка уточняется
отдельным шагом, шаг беременности пропускается, температура > 39,5 °C трактуется как
красный флаг, а рекомендации ведут к педиатру или детскому специалисту.

Итоговая карточка — бейдж срочности, врач, место, блоки обоснования/оплаты/подготовки
и предупреждение про 103/112. Срочность приходит кодами `emergency` (немедленно 103/112),
`urgent_24h` (дежурный врач или травмпункт сегодня) и `planned`; при `emergency` карточка
оформляется красной рамкой 2px с кнопками вызова 103/112.

Кнопка «← Назад» на шаг опроса перенесена в строку прогресс-бара; внизу остаются только
варианты ответа и поле ввода. «История обращений» — аккордеон: по клику раскрываются
полная рекомендация и блок «Ваши ответы при обращении» (сохраняются в `consultations.summary_json`).

## Запуск

```powershell
# 1. Собрать фронтенд
cd frontend
pnpm install
pnpm build
cd ..

# 2. Запустить сервер (читает .env, создаёт SQLite-базу)
go run .
```

Откройте http://127.0.0.1:8080 — в dev-режиме фронт сразу авторизуется как моковый пользователь.

### DEV_MODE

`DEV_MODE=true` в `.env` — `POST /api/auth` с пустым токеном создаёт сессию
мокового пользователя (`_id: dev-1`, Богдан, 18 лет, пол 1) без обращения в Core.

Разово, не меняя `.env`:

```powershell
$env:DEV_MODE='true'; go run .
```

### Фронтенд в режиме разработки

```powershell
cd frontend
pnpm dev
```

Vite поднимается на http://127.0.0.1:5173 и проксирует `/api` на `http://127.0.0.1:8080`.

## Запуск в Docker

Требуются Docker Engine и Docker Compose v2.

Фронтенд и Go-бинарник собираются внутри образа, отдельно запускать `pnpm build` не нужно. Docker сам скачает официальный образ `yuccastream/tuna:latest`, запустит Tuna рядом с приложением и авторизует её API-ключом из `.env`.

1. Создайте `.env` из безопасной заглушки:

   ```powershell
   Copy-Item .env.example .env
   ```

2. Заполните в `.env`:

   ```env
   TUNA_DOMAIN=medvedmed.ru
   TUNA_API_KEY=tak_your_tuna_api_key_here
   PUBLIC_URL=https://medvedmed.ru
   DEEPSEEK_API_KEY=sk-your-deepseek-api-key-here
   CORE_URL=https://core.example.max.ru
   CORE_SERVICE_TOKEN=your-core-service-token-here
   ```

   `TUNA_API_KEY` создаётся в [личном кабинете Tuna](https://my.tuna.am/api_keys?lang=ru) с набором прав «Для туннелей». Браузерный `tuna login` внутри контейнера невозможен — именно API-ключ заменяет вход в аккаунт.

   В `docker-compose.yml` ключ уже передаётся как переменная окружения `TUNA_API_KEY`; вписывать его в `command` не нужно. В команде используется адрес `medbot:8080` — это имя сервиса и внутренний порт Docker, а не `localhost:8080`.

   Если `PUBLIC_URL` не задан, приложение построит его из `https://${TUNA_DOMAIN}`.

3. Соберите и запустите:

   ```powershell
   docker compose up -d --build
   ```

   Docker:
   - соберёт `medbot`;
   - скачает `yuccastream/tuna:latest`, если образа ещё нет;
   - дождётся healthcheck приложения;
   - запустит `tuna http medbot:8080 --domain=${TUNA_DOMAIN}`;
   - авторизует Tuna через `TUNA_API_KEY`.

4. Проверьте:

   - локально: http://127.0.0.1:8080;
   - публично: `https://medvedmed.ru`;
   - healthcheck: `GET /api/health`.

Полезные команды:

```powershell
docker compose ps                    # app + tuna
docker compose logs -f medbot        # логи приложения
docker compose logs -f tuna          # логи Tuna, ищите строку Forwarding/url
docker compose down                  # остановить оба контейнера
```

### Если Tuna уже установлена на хосте

Можно не запускать Tuna-контейнер, а поднять только приложение:

```powershell
docker compose up -d medbot
tuna http 8080 --domain=medvedmed.ru
```

Порт `8080` опубликован только на `127.0.0.1`, поэтому локальная Tuna его увидит. Для автоматической установки/входа на Windows есть `.\scripts\tuna-host.ps1 -Domain medvedmed.ru`.

Данные SQLite лежат в `./data/medbot.db` на хосте: эта папка подключена к `/app/data` и переживает перезапуск контейнера. Путь задаётся в `docker-compose.yml` через `DB_PATH=/app/data/medbot.db`.

После изменения кода или зависимостей пересоберите образ:

```powershell
docker compose up -d --build
```

## API

### Авторизация

`POST /api/auth` — обмен токена MAX на серверную сессию:

```powershell
curl.exe -s -X POST http://127.0.0.1:8080/api/auth -H "Content-Type: application/json" -d '{\"token\":\"\"}'
# {"sessionToken":"...","firstName":"Богдан","age":18,"sex":1}
```

`GET /api/me` — проверка, что сессия жива:

```powershell
curl.exe -s http://127.0.0.1:8080/api/me -H "X-Session-Token: <sessionToken>"
```

Ошибки приходят как `{"error": "текст"}` с тем же статусом, что вернул Core
(401/403/404); сетевые сбои и 5xx Core отдаются как 502.

### Опрос (все запросы требуют `X-Session-Token`)

| Метод | Путь | Назначение |
| --- | --- | --- |
| `GET` | `/api/chat/state` | текущий шаг, вопрос, кнопки, прогресс, лента сообщений, флаг `isEmergency`, карточка |
| `POST` | `/api/chat/answer` | тело `{"answer": "..."}`: проверка красных флагов, шаг вперёд, на финале — LLM и карточка |
| `POST` | `/api/chat/back` | возврат на предыдущий шаг (запрещён при `isEmergency`) |
| `POST` | `/api/chat/reset` | новый опрос с шага `disclaimer` |
| `GET` | `/api/history` | прошлые консультации пользователя из `consultations` |

```powershell
curl.exe -s -X POST http://127.0.0.1:8080/api/chat/answer ^
  -H "X-Session-Token: <sessionToken>" -H "Content-Type: application/json" ^
  -d '{\"answer\":\"Понятно, начать опрос\"}'
```

Ответ состояния:

```json
{
  "step": "complaint_category",
  "question": "Что беспокоит вас больше всего? ...",
  "options": ["Боль", "Температура", "Дыхание", "ЖКТ", "Кожа", "Травма", "Головокружение, онемение, сон"],
  "allowFreeText": true,
  "canGoBack": true,
  "progress": { "current": 2, "total": 15 },
  "messages": [{ "role": "bot", "step": "disclaimer", "text": "...", "at": "..." }],
  "isEmergency": false,
  "completed": false
}
```

На шаге `result` дополнительно приходит `card`: `specialty`, `urgency`, `place`,
`reasoning`, `paymentNote`, `preparationTips`.

### Красные флаги

Любой входящий текст и выбранный вариант проверяются детектором **до LLM**:
боль/сдавливание в груди, одышка, обморок, судороги, кровотечение, признаки
инсульта, анафилаксия, острая боль в животе, температура > 39,5 °C у ребёнка,
травма головы с потерей сознания/тошнотой/спутанностью. При срабатывании опрос
прерывается (`isEmergency: true`, шаг `emergency`), в ленте — инструкция
«Вызовите скорую помощь (103 или 112)» и причина. Возврат назад и новые ответы
в этом состоянии запрещены, помогает только `reset`.

## Переменные окружения

| Переменная | По умолчанию | Назначение |
| --- | --- | --- |
| `PORT` | `8080` | порт HTTP-сервера |
| `DEV_MODE` | `false` | моковый пользователь без Core |
| `PUBLIC_URL` | из `TUNA_DOMAIN` | публичный URL мини-приложения |
| `TUNA_DOMAIN` | — | домен туннеля Tuna без схемы |
| `TUNA_API_KEY` | — | API-ключ Tuna |
| `CORE_URL` | — | база Core API |
| `CORE_SERVICE_TOKEN` | — | Bearer-токен сервиса для Core |
| `DB_PATH` | `./medbot.db` | файл SQLite (WAL) |
| `DEEPSEEK_API_KEY` | — | ключ DeepSeek; пустой — только локальные правила |
| `STATIC_DIR` | `./frontend/dist` | каталог собранного фронтенда |
| `MAX_BOT_TOKEN` | — | токен бота MAX для настройки мини-приложения |

## Что учтено

- ФЗ-152: в БД только `core_user_id` (`_id` из Core), `firstName` для приветствия,
  возраст, пол и ответы опроса. ФИО целиком, СНИЛС, ОМС, телефон не сохраняются и не логируются.
- Сессии и консультации переживают рестарт: SQLite в режиме WAL.
- `sessionToken` — 32 случайных байта из `crypto/rand`, он же `sessions.id`.
- Карточка гарантированно собирается даже без DeepSeek: при пустом ключе,
  таймауте, ошибке HTTP или битом JSON работает локальный фоллбэк.
- `X-Frame-Options` не выставляется: мини-апп MAX работает в iframe.
- Токен из `?token=` удаляется из адресной строки до запроса на сервер.
- В логах только метод, путь и статус — без query (там токен) и без персональных данных.
