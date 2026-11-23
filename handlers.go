package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// Моковый пользователь для локальной разработки (DEV_MODE=true).
const (
	devUserID   = "dev-1"
	devUserName = "Богдан"
	devUserAge  = 18
	devUserSex  = 1
)

type authRequest struct {
	Token string `json:"token"`
}

type authResponse struct {
	SessionToken string `json:"sessionToken"`
	FirstName    string `json:"firstName"`
	Age          int    `json:"age"`
	Sex          int    `json:"sex"`
}

type meResponse struct {
	UserID    string    `json:"_id"`
	FirstName string    `json:"firstName"`
	Age       int       `json:"age"`
	Sex       int       `json:"sex"`
	CreatedAt time.Time `json:"createdAt"`
}

func (a *app) handleAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSONError(w, http.StatusMethodNotAllowed, "используйте POST /api/auth")
		return
	}

	var payload authRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
		writeJSONError(w, http.StatusBadRequest, `тело запроса должно быть JSON вида {"token": "..."}`)
		return
	}

	token := strings.TrimSpace(payload.Token)

	// Пустой токен: в dev-режиме сразу отдаём мокового пользователя,
	// иначе — понятная ошибка «откройте сервис через бота».
	if token == "" {
		if !a.cfg.DevMode {
			writeJSONError(w, http.StatusUnauthorized, "Токен не передан. Откройте сервис через бота в MAX.")
			return
		}
		a.issueSession(w, devUserID, devUserName, devUserAge, devUserSex)
		return
	}

	user, err := a.fetchCoreUser(r.Context(), token)
	if err != nil {
		writeAPIError(w, err)
		return
	}

	age, err := fullAge(user.BirthDate, time.Now())
	if err != nil {
		log.Printf("auth: %v", err)
		writeJSONError(w, http.StatusBadGateway, "Core вернул некорректную дату рождения")
		return
	}

	// firstName из Core может быть пустым — сохраняем пустую строку,
	// чтобы фронт не рисовал «Привет, !».
	a.issueSession(w, user.ID, user.firstName(), age, user.Sex)
}

// issueSession создаёт серверную сессию в SQLite и возвращает фронту её токен.
func (a *app) issueSession(w http.ResponseWriter, coreUserID, firstName string, age, sex int) {
	sess, err := a.store.createSession(coreUserID, firstName, age, sex)
	if err != nil {
		log.Printf("auth: создать сессию: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "не удалось создать сессию")
		return
	}

	writeJSON(w, http.StatusOK, authResponse{
		SessionToken: sess.ID,
		FirstName:    sess.FirstName,
		Age:          sess.Age,
		Sex:          sess.Sex,
	})
}

// handleMe проверяет, что сессия в SQLite ещё жива.
func (a *app) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeJSONError(w, http.StatusMethodNotAllowed, "используйте GET /api/me")
		return
	}

	token := strings.TrimSpace(r.Header.Get("X-Session-Token"))
	if token == "" {
		writeJSONError(w, http.StatusUnauthorized, "Заголовок X-Session-Token не передан.")
		return
	}

	sess, err := a.store.loadSession(token)
	if err != nil {
		if errors.Is(err, errSessionNotFound) {
			writeJSONError(w, http.StatusUnauthorized, "Сессия не найдена или истекла. Авторизуйтесь заново.")
			return
		}
		log.Printf("me: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "не удалось загрузить сессию")
		return
	}

	writeJSON(w, http.StatusOK, meResponse{
		UserID:    sess.CoreUserID,
		FirstName: sess.FirstName,
		Age:       sess.Age,
		Sex:       sess.Sex,
		CreatedAt: sess.UpdatedAt,
	})
}

func writeAPIError(w http.ResponseWriter, err error) {
	var apiErr *apiError
	if errors.As(err, &apiErr) {
		writeJSONError(w, apiErr.Status, apiErr.Message)
		return
	}
	log.Printf("internal: %v", err)
	writeJSONError(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("writeJSON: %v", err)
	}
}
