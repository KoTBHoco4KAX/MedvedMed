package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var errSessionNotFound = errors.New("session not found")

const schemaSQL = `
CREATE TABLE IF NOT EXISTS sessions (
    id                 TEXT PRIMARY KEY,
    core_user_id       TEXT NOT NULL DEFAULT '',
    first_name         TEXT NOT NULL DEFAULT '',
    age                INTEGER NOT NULL DEFAULT 0,
    sex                INTEGER NOT NULL DEFAULT 0,
    current_step       TEXT NOT NULL DEFAULT 'disclaimer',
    history_stack_json TEXT NOT NULL DEFAULT '[]',
    answers_json       TEXT NOT NULL DEFAULT '{}',
    messages_json      TEXT NOT NULL DEFAULT '[]',
    is_emergency       BOOL NOT NULL DEFAULT 0,
    emergency_reason   TEXT NOT NULL DEFAULT '',
    updated_at         DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS consultations (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    core_user_id TEXT NOT NULL,
    summary_json TEXT NOT NULL,
    created_at   DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_consultations_core_user
    ON consultations (core_user_id, created_at DESC);
`

// store — доступ к SQLite. В таблицах сессий нет ПДн: только core _id,
// имя для приветствия, возраст и пол.
type store struct {
	db *sql.DB
}

func openStore(path string) (*store, error) {
	if strings.TrimSpace(path) == "" {
		path = "./medbot.db"
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("создать каталог БД %s: %w", dir, err)
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("открыть sqlite %s: %w", path, err)
	}
	// Один writer-коннект: WAL даёт параллельное чтение, а запись не упирается в SQLITE_BUSY.
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)

	if err := applyPragmas(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("создать схему БД: %w", err)
	}

	log.Printf("БД SQLite готова: %s (journal_mode=WAL)", path)
	return &store{db: db}, nil
}

func applyPragmas(db *sql.DB) error {
	var mode string
	if err := db.QueryRow("PRAGMA journal_mode=WAL").Scan(&mode); err != nil {
		return fmt.Errorf("включить WAL: %w", err)
	}
	if !strings.EqualFold(mode, "wal") {
		log.Printf("БД: journal_mode=%q, WAL включить не удалось", mode)
	}
	for _, pragma := range []string{
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("%s: %w", pragma, err)
		}
	}
	return nil
}

func (s *store) close() error {
	return s.db.Close()
}

// loadSession читает сессию по sessionToken (он же sessions.id).
func (s *store) loadSession(id string) (*chatSession, error) {
	row := s.db.QueryRow(`
		SELECT id, core_user_id, first_name, age, sex, current_step,
		       history_stack_json, answers_json, messages_json,
		       is_emergency, emergency_reason, updated_at
		FROM sessions WHERE id = ?`, id)

	var (
		sess         chatSession
		historyJSON  string
		answersJSON  string
		messagesJSON string
		emergency    int
		updatedAt    string
	)
	err := row.Scan(
		&sess.ID, &sess.CoreUserID, &sess.FirstName, &sess.Age, &sess.Sex, &sess.CurrentStep,
		&historyJSON, &answersJSON, &messagesJSON,
		&emergency, &sess.EmergencyReason, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("прочитать сессию: %w", err)
	}

	sess.IsEmergency = emergency != 0
	sess.UpdatedAt = parseStoredTime(updatedAt)
	sess.History = decodeJSONArray(historyJSON)
	sess.Answers = decodeJSONMap(answersJSON)
	sess.Messages = decodeJSONMessages(messagesJSON)
	return &sess, nil
}

func (s *store) saveSession(sess *chatSession) error {
	sess.UpdatedAt = time.Now()
	emergency := 0
	if sess.IsEmergency {
		emergency = 1
	}

	_, err := s.db.Exec(`
		INSERT INTO sessions (
			id, core_user_id, first_name, age, sex, current_step,
			history_stack_json, answers_json, messages_json,
			is_emergency, emergency_reason, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			core_user_id       = excluded.core_user_id,
			first_name         = excluded.first_name,
			age                = excluded.age,
			sex                = excluded.sex,
			current_step       = excluded.current_step,
			history_stack_json = excluded.history_stack_json,
			answers_json       = excluded.answers_json,
			messages_json      = excluded.messages_json,
			is_emergency       = excluded.is_emergency,
			emergency_reason   = excluded.emergency_reason,
			updated_at         = excluded.updated_at`,
		sess.ID, sess.CoreUserID, sess.FirstName, sess.Age, sess.Sex, sess.CurrentStep,
		mustMarshal(sess.History), mustMarshal(sess.Answers), mustMarshal(sess.Messages),
		emergency, sess.EmergencyReason, formatStoredTime(sess.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("сохранить сессию: %w", err)
	}
	return nil
}

// createSession заводит новую сессию и возвращает её вместе со случайным sessionToken.
func (s *store) createSession(coreUserID, firstName string, age, sex int) (*chatSession, error) {
	token, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	sess := &chatSession{
		ID:         token,
		CoreUserID: coreUserID,
		FirstName:  firstName,
		Age:        age,
		Sex:        sex,
	}
	initSession(sess)
	if err := s.saveSession(sess); err != nil {
		return nil, err
	}
	return sess, nil
}

// qaPair — один вопрос и ответ пациента для истории обращений.
type qaPair struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// consultationSummary — то, что лежит в consultations.summary_json:
// поля итоговой карточки плюс ответы пользователя из опроса.
type consultationSummary struct {
	finalCard
	Answers []qaPair `json:"answers"`
}

// consultationRecord — одна прошлая консультация для GET /api/history.
type consultationRecord struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	Card      finalCard `json:"card"`
	Answers   []qaPair  `json:"answers"`
}

func (s *store) insertConsultation(coreUserID string, summary consultationSummary) (int64, error) {
	if summary.Answers == nil {
		summary.Answers = []qaPair{}
	}

	payload, err := json.Marshal(summary)
	if err != nil {
		return 0, fmt.Errorf("сериализовать карточку: %w", err)
	}
	res, err := s.db.Exec(
		`INSERT INTO consultations (core_user_id, summary_json, created_at) VALUES (?, ?, ?)`,
		coreUserID, string(payload), formatStoredTime(time.Now()),
	)
	if err != nil {
		return 0, fmt.Errorf("сохранить консультацию: %w", err)
	}
	return res.LastInsertId()
}

func (s *store) listConsultations(coreUserID string, limit int) ([]consultationRecord, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT id, summary_json, created_at
		FROM consultations
		WHERE core_user_id = ?
		ORDER BY created_at DESC, id DESC
		LIMIT ?`, coreUserID, limit)
	if err != nil {
		return nil, fmt.Errorf("прочитать консультации: %w", err)
	}
	defer rows.Close()

	items := make([]consultationRecord, 0, 8)
	for rows.Next() {
		var (
			rec     consultationRecord
			summary string
			created string
		)
		if err := rows.Scan(&rec.ID, &summary, &created); err != nil {
			return nil, fmt.Errorf("разобрать консультацию: %w", err)
		}
		rec.CreatedAt = parseStoredTime(created)

		var stored consultationSummary
		if err := json.Unmarshal([]byte(summary), &stored); err != nil {
			log.Printf("consultations #%d: битый summary_json: %v", rec.ID, err)
			continue
		}
		rec.Card = stored.finalCard
		rec.Answers = stored.Answers
		if rec.Answers == nil {
			rec.Answers = []qaPair{}
		}
		items = append(items, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("прочитать консультации: %w", err)
	}
	return items, nil
}

const storedTimeLayout = time.RFC3339Nano

func formatStoredTime(t time.Time) string {
	return t.UTC().Format(storedTimeLayout)
}

// parseStoredTime терпим к форматам, которые может вернуть драйвер SQLite.
func parseStoredTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		storedTimeLayout,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05",
	} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}
	return time.Time{}
}

func mustMarshal(v any) string {
	buf, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(buf)
}

func decodeJSONArray(raw string) []string {
	var out []string
	if strings.TrimSpace(raw) == "" || raw == "null" {
		return []string{}
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return []string{}
	}
	return out
}

func decodeJSONMap(raw string) map[string]string {
	out := map[string]string{}
	if strings.TrimSpace(raw) == "" || raw == "null" {
		return out
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return map[string]string{}
	}
	if out == nil {
		return map[string]string{}
	}
	return out
}

func decodeJSONMessages(raw string) []chatMessage {
	var out []chatMessage
	if strings.TrimSpace(raw) == "" || raw == "null" {
		return []chatMessage{}
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return []chatMessage{}
	}
	return out
}
