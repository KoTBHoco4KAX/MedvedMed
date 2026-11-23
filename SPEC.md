# Техническая спецификация: МедВед.Код2 (Триаж-бот)

## Стек
- Backend: Go (стандартный `net/http`, порт из `.env` `PORT=8080`). Отдает API (`/api/*`) и собранную статику фронтенда из `./frontend/dist`.
- Frontend: React + TypeScript + Vite + Tailwind CSS (в папке `./frontend`).
- База данных: SQLite (`modernc.org/sqlite`, режим WAL).
- AI: DeepSeek API (`deepseek-chat`, JSON-режим).

## Интеграция с MAX и Core API
1. Приложение открывается внутри iframe мини-аппа MAX по ссылке вида `https://hearty-squid-8971.a.tuna.free/?token=<JWT>`.
2. На сервере ЗАПРЕЩЕНО ставить заголовок `X-Frame-Options: DENY` (приложение должно свободно работать в iframe).
3. Фронтенд при загрузке забирает `token` из `window.location.search`, сразу удаляет его из адресной строки через `window.history.replaceState({}, '', window.location.pathname)` и отправляет `POST /api/auth` с телом `{"token": "..."}`.
4. Бэкенд в `POST /api/auth`:
   - Если `DEV_MODE=true` и `token` пустой — использует мокового пользователя: `_id: "dev-user-1"`, `birthDate: "1995-05-15"`, `sex: 2`, `firstName: "Тест"`.
   - В боевом режиме делает запрос `GET <CORE_URL>/api/v3/users/user-for-external-service?token=<JWT>` с заголовком `Authorization: Bearer <CORE_SERVICE_TOKEN>`.
   - Обрабатывает статусы ответа Core: 401 (токен протух или неверен), 403 (ошибка сервисного токена), 404 (пользователь не найден).
   - По ФЗ-152 бэкенд НЕ сохраняет в БД и НЕ логирует ФИО, СНИЛС, номер ОМС и телефон. В сессии хранится только `_id`, вычисленный возраст (`age`), пол (`sex`) и `firstName` для приветствия в UI.
   - Устанавливает HttpOnly cookie `session_id` (срок жизни 24 часа, `SameSite=None; Secure` для работы внутри кросс-доменного iframe!) и возвращает JSON: `{"firstName": "...", "age": 30, "sex": 1}`.