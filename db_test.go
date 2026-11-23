package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSchemaMatchesSpec(t *testing.T) {
	st, err := openStore(filepath.Join(t.TempDir(), "schema.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.close()

	wantSessions := []string{
		"id", "core_user_id", "first_name", "age", "sex", "current_step",
		"history_stack_json", "answers_json", "messages_json",
		"is_emergency", "emergency_reason", "updated_at",
	}
	if got := tableColumns(t, st, "sessions"); strings.Join(got, ",") != strings.Join(wantSessions, ",") {
		t.Fatalf("колонки sessions: %v, ожидались %v", got, wantSessions)
	}

	wantConsultations := []string{"id", "core_user_id", "summary_json", "created_at"}
	if got := tableColumns(t, st, "consultations"); strings.Join(got, ",") != strings.Join(wantConsultations, ",") {
		t.Fatalf("колонки consultations: %v, ожидались %v", got, wantConsultations)
	}
}

func TestStorePersistsSessionAndConsultations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "medbot.db")
	st, err := openStore(path)
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}

	var mode string
	if err := st.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("journal_mode: %v", err)
	}
	if !strings.EqualFold(mode, "wal") {
		t.Fatalf("journal_mode = %q, ожидался WAL", mode)
	}

	sess, err := st.createSession("core-42", "Иван", 40, 1)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}
	if sess.ID == "" || sess.CurrentStep != stepDisclaimer || len(sess.Messages) != 1 {
		t.Fatalf("новая сессия создана неверно: %+v", sess)
	}

	// Двигаем опрос на один шаг вперёд.
	a := &app{store: st}
	if _, err := a.answer(context.Background(), sess, "Понятно, начать опрос"); err != nil {
		t.Fatalf("answer: %v", err)
	}
	if sess.CurrentStep != stepComplaintCategory {
		t.Fatalf("шаг после ответа: %q", sess.CurrentStep)
	}

	// Закрываем БД и открываем заново — сессия должна пережить рестарт.
	if err := st.close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	st2, err := openStore(path)
	if err != nil {
		t.Fatalf("повторный openStore: %v", err)
	}
	defer st2.close()

	loaded, err := st2.loadSession(sess.ID)
	if err != nil {
		t.Fatalf("loadSession после рестарта: %v", err)
	}
	if loaded.CoreUserID != "core-42" || loaded.FirstName != "Иван" || loaded.Age != 40 || loaded.Sex != 1 {
		t.Fatalf("данные пользователя потерялись: %+v", loaded)
	}
	if loaded.CurrentStep != stepComplaintCategory {
		t.Fatalf("шаг не сохранился: %q", loaded.CurrentStep)
	}
	if len(loaded.History) != 1 || loaded.History[0] != stepDisclaimer {
		t.Fatalf("history_stack_json не сохранился: %v", loaded.History)
	}
	if loaded.Answers[stepDisclaimer] != "Понятно, начать опрос" {
		t.Fatalf("answers_json не сохранился: %v", loaded.Answers)
	}
	if len(loaded.Messages) != 3 {
		t.Fatalf("messages_json: %d сообщений, ожидалось 3", len(loaded.Messages))
	}

	// Консультации: карточка + человекочитаемые ответы пользователя.
	summary := consultationSummary{
		finalCard: finalCard{Specialty: "Врач-терапевт", Urgency: "Планово (в течение 3–5 дней)"},
		Answers: []qaPair{
			{Question: "Основная жалоба", Answer: "Боль"},
			{Question: "Где болит", Answer: "Голова"},
		},
	}
	if _, err := st2.insertConsultation("core-42", summary); err != nil {
		t.Fatalf("insertConsultation: %v", err)
	}
	items, err := st2.listConsultations("core-42", 50)
	if err != nil {
		t.Fatalf("listConsultations: %v", err)
	}
	if len(items) != 1 || items[0].Card.Specialty != "Врач-терапевт" || items[0].CreatedAt.IsZero() {
		t.Fatalf("история консультаций: %+v", items)
	}
	if len(items[0].Answers) != 2 {
		t.Fatalf("ответы пользователя не сохранились: %+v", items[0].Answers)
	}
	if items[0].Answers[0].Question != "Основная жалоба" || items[0].Answers[1].Answer != "Голова" {
		t.Fatalf("пары вопрос-ответ повреждены: %+v", items[0].Answers)
	}

	// Старые записи без answers должны читаться с пустым списком.
	if _, err := st2.db.Exec(
		`INSERT INTO consultations (core_user_id, summary_json, created_at) VALUES (?, ?, ?)`,
		"core-legacy", `{"specialty":"Врач-терапевт","urgency":"Планово (в течение 3–5 дней)"}`, formatStoredTime(time.Now()),
	); err != nil {
		t.Fatalf("вставить legacy-консультацию: %v", err)
	}
	legacy, err := st2.listConsultations("core-legacy", 50)
	if err != nil {
		t.Fatalf("listConsultations legacy: %v", err)
	}
	if len(legacy) != 1 || legacy[0].Answers == nil || len(legacy[0].Answers) != 0 {
		t.Fatalf("legacy-консультация должна читаться с пустыми answers: %+v", legacy)
	}

	other, err := st2.listConsultations("core-other", 50)
	if err != nil {
		t.Fatalf("listConsultations другого пользователя: %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("чужие консультации не должны возвращаться: %+v", other)
	}
}

func tableColumns(t *testing.T, st *store, table string) []string {
	t.Helper()

	rows, err := st.db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		t.Fatalf("table_info(%s): %v", table, err)
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var (
			cid      int
			name     string
			ctype    string
			notNull  int
			defaultV sql.NullString
			primary  int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &defaultV, &primary); err != nil {
			t.Fatalf("scan table_info: %v", err)
		}
		columns = append(columns, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("table_info rows: %v", err)
	}
	return columns
}
