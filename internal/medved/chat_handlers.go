package medved

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
)

type answerRequest struct {
	Answer string `json:"answer"`
}

const maxAnswerBody = 64 << 10

// requireSession достаёт сессию по X-Session-Token или сразу отвечает 401.
func (a *app) requireSession(w http.ResponseWriter, r *http.Request) (*chatSession, bool) {
	token := strings.TrimSpace(r.Header.Get("X-Session-Token"))
	if token == "" {
		writeJSONError(w, http.StatusUnauthorized, "Заголовок X-Session-Token не передан.")
		return nil, false
	}

	sess, err := a.store.loadSession(token)
	if err != nil {
		if errors.Is(err, errSessionNotFound) {
			writeJSONError(w, http.StatusUnauthorized, "Сессия не найдена или истекла. Авторизуйтесь заново.")
			return nil, false
		}
		log.Printf("chat: загрузить сессию: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "Не удалось загрузить сессию.")
		return nil, false
	}

	// В DEV_MODE не подхватываем старые реальные сессии MAX:
	// иначе в браузере остаётся прежний пол/возраст и dev-настройки не работают.
	if a.cfg.DevMode && sess.CoreUserID != devUserID {
		writeJSONError(w, http.StatusUnauthorized, "DEV_MODE: используется тестовый пользователь. Обновите страницу.")
		return nil, false
	}

	// Сессии, сохранённые до переработки шагов, могут стоять на удалённом шаге:
	// в этом случае начинаем опрос заново, чтобы FSM не застряла.
	if !sess.IsEmergency {
		if _, ok := findStep(sess.steps(), sess.CurrentStep); !ok {
			log.Printf("chat: сессия на устаревшем шаге %q — опрос сброшен", sess.CurrentStep)
			initSession(sess)
			if err := a.store.saveSession(sess); err != nil {
				log.Printf("chat: сбросить устаревшую сессию: %v", err)
				writeJSONError(w, http.StatusInternalServerError, "Не удалось загрузить сессию.")
				return nil, false
			}
		}
	}

	return sess, true
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	w.Header().Set("Allow", method)
	writeJSONError(w, http.StatusMethodNotAllowed, "используйте "+method+" "+r.URL.Path)
	return false
}

func decodeAnswer(w http.ResponseWriter, r *http.Request) (answerRequest, bool) {
	var payload answerRequest
	err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxAnswerBody)).Decode(&payload)
	if err != nil && !errors.Is(err, io.EOF) {
		writeJSONError(w, http.StatusBadRequest, `тело запроса должно быть JSON вида {"answer": "..."}`)
		return answerRequest{}, false
	}
	return payload, true
}

// GET /api/chat/state — текущее состояние опроса, лента сообщений и карточка.
func (a *app) handleChatState(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	sess, ok := a.requireSession(w, r)
	if !ok {
		return
	}

	// Сессия без ленты (например, создана до появления опроса) инициализируется лениво.
	if len(sess.Messages) == 0 {
		initSession(sess)
		if err := a.store.saveSession(sess); err != nil {
			log.Printf("chat: инициализировать ленту: %v", err)
			writeJSONError(w, http.StatusInternalServerError, "Не удалось подготовить опрос.")
			return
		}
	}

	writeJSON(w, http.StatusOK, sess.state())
}

// POST /api/chat/answer — ответ пользователя, проверка красных флагов и шаг вперёд.
func (a *app) handleChatAnswer(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	sess, ok := a.requireSession(w, r)
	if !ok {
		return
	}
	payload, ok := decodeAnswer(w, r)
	if !ok {
		return
	}

	state, err := a.answer(r.Context(), sess, payload.Answer)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

// POST /api/chat/back — возврат на предыдущий шаг.
func (a *app) handleChatBack(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	sess, ok := a.requireSession(w, r)
	if !ok {
		return
	}

	state, err := a.back(sess)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

// POST /api/chat/reset — новый опрос с чистого листа.
func (a *app) handleChatReset(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	sess, ok := a.requireSession(w, r)
	if !ok {
		return
	}

	state, err := a.reset(sess)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

// GET /api/history — прошлые консультации пользователя.
func (a *app) handleHistory(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	sess, ok := a.requireSession(w, r)
	if !ok {
		return
	}

	items, err := a.store.listConsultations(sess.CoreUserID, 50)
	if err != nil {
		log.Printf("history: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "Не удалось загрузить историю консультаций.")
		return
	}
	writeJSON(w, http.StatusOK, items)
}
