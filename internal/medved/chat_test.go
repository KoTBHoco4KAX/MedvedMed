package medved

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestChatStateResetsLegacyStep(t *testing.T) {
	st, err := openStore(filepath.Join(t.TempDir(), "chat.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.close()

	sess, err := st.createSession("core-legacy", "Иван", 40, 1)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}
	// Такой шаг был в прошлой версии FSM и больше не существует.
	sess.CurrentStep = "meds_and_allergies"
	if err := st.saveSession(sess); err != nil {
		t.Fatalf("saveSession: %v", err)
	}

	a := &app{store: st}
	request := httptest.NewRequest(http.MethodGet, "/api/chat/state", nil)
	request.Header.Set("X-Session-Token", sess.ID)
	recorder := httptest.NewRecorder()

	a.handleChatState(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("статус %d, ожидался 200: %s", recorder.Code, recorder.Body.String())
	}

	var state chatState
	if err := json.Unmarshal(recorder.Body.Bytes(), &state); err != nil {
		t.Fatalf("разобрать ответ: %v", err)
	}
	if state.Step != stepDisclaimer {
		t.Fatalf("устаревшая сессия должна начинаться заново, получено %q", state.Step)
	}
	if state.Progress.Current != 1 || state.Progress.Total != 15 {
		t.Fatalf("прогресс после сброса: %+v", state.Progress)
	}

	reloaded, err := st.loadSession(sess.ID)
	if err != nil {
		t.Fatalf("loadSession: %v", err)
	}
	if reloaded.CurrentStep != stepDisclaimer {
		t.Fatalf("сброс не сохранён в БД: %q", reloaded.CurrentStep)
	}
}

func TestHistoryReturnsAnswers(t *testing.T) {
	st, err := openStore(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.close()

	sess, err := st.createSession("core-history", "Иван", 40, 1)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}
	if _, err := st.insertConsultation(sess.CoreUserID, consultationSummary{
		finalCard: finalCard{Specialty: "Врач-терапевт", Urgency: "Планово (в течение 3–5 дней)"},
		Answers: []qaPair{
			{Question: "Основная жалоба", Answer: "Боль"},
			{Question: "Приём лекарств", Answer: medsNone},
		},
	}); err != nil {
		t.Fatalf("insertConsultation: %v", err)
	}

	a := &app{store: st}
	request := httptest.NewRequest(http.MethodGet, "/api/history", nil)
	request.Header.Set("X-Session-Token", sess.ID)
	recorder := httptest.NewRecorder()

	a.handleHistory(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("статус %d, ожидался 200: %s", recorder.Code, recorder.Body.String())
	}

	var items []consultationRecord
	if err := json.Unmarshal(recorder.Body.Bytes(), &items); err != nil {
		t.Fatalf("разобрать ответ: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("консультаций %d, ожидалась 1", len(items))
	}
	if len(items[0].Answers) != 2 || items[0].Answers[0].Question != "Основная жалоба" {
		t.Fatalf("ответы не вернулись в историю: %+v", items[0].Answers)
	}
	if items[0].Card.Specialty != "Врач-терапевт" {
		t.Fatalf("карточка консультации потерялась: %+v", items[0].Card)
	}
}

func TestChatHabitsCycleOverHTTP(t *testing.T) {
	st, err := openStore(filepath.Join(t.TempDir(), "habits-http.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.close()

	sess, err := st.createSession("core-habits", "Иван", 30, 1)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}
	sess.CurrentStep = stepHabits
	sess.Answers = map[string]string{}
	sess.History = []string{}
	sess.Messages = []chatMessage{botMessage(stepHabits, habitsIntroQuestion)}
	if err := st.saveSession(sess); err != nil {
		t.Fatalf("saveSession: %v", err)
	}

	a := &app{store: st}

	state := getChatStateHTTP(t, a, sess.ID)
	if state.Step != stepHabits {
		t.Fatalf("стартовый шаг: %q", state.Step)
	}
	if len(state.Options) == 0 || state.Options[0] != habitsNone {
		t.Fatalf("первый вариант должен быть %q: %v", habitsNone, state.Options)
	}

	state = postChatAnswerHTTP(t, a, sess.ID, "Курение / вейп")
	if state.Step != stepHabits {
		t.Fatalf("после привычки шаг должен остаться habits, получено %q", state.Step)
	}
	if state.Question != habitsContinueQuestion {
		t.Fatalf("вопрос второй итерации: %q", state.Question)
	}
	if len(state.Options) == 0 || state.Options[0] != habitsDone {
		t.Fatalf("первой кнопкой должно быть %q: %v", habitsDone, state.Options)
	}
	if containsString(state.Options, habitsNone) || containsString(state.Options, "Курение / вейп") {
		t.Fatalf("выбранное и «нет привычек» не должны повторяться: %v", state.Options)
	}

	state = postChatAnswerHTTP(t, a, sess.ID, habitsDone)
	if state.Step != stepPaymentType {
		t.Fatalf("после «На этом всё» ожидался payment_type, получено %q", state.Step)
	}
}

func getChatStateHTTP(t *testing.T, a *app, token string) chatState {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, "/api/chat/state", nil)
	request.Header.Set("X-Session-Token", token)
	recorder := httptest.NewRecorder()
	a.handleChatState(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("статус %d, ожидался 200: %s", recorder.Code, recorder.Body.String())
	}

	var state chatState
	if err := json.Unmarshal(recorder.Body.Bytes(), &state); err != nil {
		t.Fatalf("разобрать ответ: %v", err)
	}
	return state
}

func postChatAnswerHTTP(t *testing.T, a *app, token, answer string) chatState {
	t.Helper()

	payload, err := json.Marshal(map[string]string{"answer": answer})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/chat/answer", bytes.NewReader(payload))
	request.Header.Set("X-Session-Token", token)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	a.handleChatAnswer(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("статус %d, ожидался 200: %s", recorder.Code, recorder.Body.String())
	}

	var state chatState
	if err := json.Unmarshal(recorder.Body.Bytes(), &state); err != nil {
		t.Fatalf("разобрать ответ: %v", err)
	}
	return state
}

func TestChatStateRequiresToken(t *testing.T) {
	st, err := openStore(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.close()

	a := &app{store: st}
	recorder := httptest.NewRecorder()
	a.handleChatState(recorder, httptest.NewRequest(http.MethodGet, "/api/chat/state", nil))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("статус %d, ожидался 401", recorder.Code)
	}
}
