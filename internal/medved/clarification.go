package medved

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	stepAIClarification = "ai_clarification"
	// aiClarificationKey — служебный ключ в answers_json: там лежит вопрос и кнопки,
	// которые сгенерировал DeepSeek. Сам ответ пользователя — в Answers["ai_clarification"].
	aiClarificationKey   = "__ai_clarification"
	clarificationTimeout = 5 * time.Second
)

// aiClarification — один адаптивный уточняющий вопрос от DeepSeek.
type aiClarification struct {
	Need     bool     `json:"need_clarification"`
	Question string   `json:"question"`
	Options  []string `json:"options"`
}

const clarificationSystemPrompt = `Ты — медицинский триаж-ассистент сервиса «МедВед». По ответам о текущей жалобе реши, достаточно ли данных для безопасной маршрутизации или нужен ОДИН короткий уточняющий вопрос, чтобы отличить бытовое недомогание от опасного состояния.

Правила:
1. Не ставь диагноз, не назначай лечение, лекарства и дозировки.
2. Вопрос задавай простым языком, на «вы», без сложных медицинских терминов.
3. Дай 3–4 коротких взаимоисключающих варианта ответа.
4. Если данных достаточно для маршрутизации — need_clarification=false, question пустой, options пустой массив.
5. Отвечай строго валидным JSON без markdown:
{"need_clarification": true, "question": "короткий вопрос", "options": ["вариант 1", "вариант 2", "вариант 3"]}
или
{"need_clarification": false, "question": "", "options": []}`

func clarificationStep(cl *aiClarification) stepSpec {
	return stepSpec{
		ID:            stepAIClarification,
		Label:         "Уточняющий вопрос",
		Question:      cl.Question,
		Options:       cl.Options,
		AllowFreeText: true,
	}
}

// parseStoredClarification достаёт сохранённый вопрос DeepSeek из answers_json.
func parseStoredClarification(answers map[string]string) *aiClarification {
	raw := strings.TrimSpace(answers[aiClarificationKey])
	if raw == "" {
		return nil
	}

	var cl aiClarification
	if err := json.Unmarshal([]byte(raw), &cl); err != nil {
		return nil
	}
	return normalizeClarification(&cl)
}

func storeClarification(sess *chatSession, cl *aiClarification) {
	if cl == nil || !cl.Need {
		clearClarification(sess)
		return
	}
	normalized := normalizeClarification(cl)
	if normalized == nil {
		clearClarification(sess)
		return
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		clearClarification(sess)
		return
	}
	sess.Answers[aiClarificationKey] = string(payload)
}

// clearClarification сбрасывает уточняющий вопрос и ответ на него:
// при изменении базовых ответов старый вопрос становится неактуальным.
func clearClarification(sess *chatSession) {
	delete(sess.Answers, aiClarificationKey)
	delete(sess.Answers, stepAIClarification)
}

// normalizeClarification проверяет и подчищает ответ LLM:
// нужен непустой вопрос и 3–4 варианта кнопок.
func normalizeClarification(cl *aiClarification) *aiClarification {
	if cl == nil || !cl.Need {
		return nil
	}

	question := strings.TrimSpace(cl.Question)
	if question == "" {
		return nil
	}

	options := make([]string, 0, 4)
	for _, option := range cl.Options {
		option = strings.TrimSpace(option)
		if option == "" || containsString(options, option) {
			continue
		}
		options = append(options, option)
		if len(options) == 4 {
			break
		}
	}
	if len(options) < 3 {
		return nil
	}

	return &aiClarification{Need: true, Question: question, Options: options}
}

// parseClarificationContent разбирает ответ DeepSeek. nil, nil означает «уточнение не нужно».
func parseClarificationContent(content string) (*aiClarification, error) {
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return nil, errors.New("в ответе модели нет JSON-объекта")
	}

	var cl aiClarification
	if err := json.Unmarshal([]byte(content[start:end+1]), &cl); err != nil {
		return nil, fmt.Errorf("некорректный JSON уточнения: %w", err)
	}
	if !cl.Need {
		return nil, nil
	}
	normalized := normalizeClarification(&cl)
	if normalized == nil {
		return nil, errors.New("модель вернула неполный уточняющий вопрос")
	}
	return normalized, nil
}

// requestClarification просит у DeepSeek один уточняющий вопрос с лимитом 5 секунд.
func (a *app) requestClarification(ctx context.Context, sess *chatSession) (*aiClarification, error) {
	if a == nil || strings.TrimSpace(a.cfg.DeepSeekAPIKey) == "" {
		return nil, errors.New("DEEPSEEK_API_KEY не задан")
	}
	if a.llm == nil {
		return nil, errors.New("HTTP-клиент DeepSeek не инициализирован")
	}

	ctx, cancel := context.WithTimeout(ctx, clarificationTimeout)
	defer cancel()

	payload := deepSeekRequest{
		Model: deepSeekModel,
		Messages: []deepSeekMessage{
			{Role: "system", Content: clarificationSystemPrompt},
			{Role: "user", Content: clarificationUserPrompt(sess)},
		},
		ResponseFormat: map[string]string{"type": "json_object"},
		Temperature:    0.2,
		MaxTokens:      300,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("собрать запрос уточнения: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, deepSeekEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("собрать запрос уточнения: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+a.cfg.DeepSeekAPIKey)

	response, err := a.llm.Do(request)
	if err != nil {
		return nil, fmt.Errorf("DeepSeek недоступен: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return nil, fmt.Errorf("DeepSeek вернул %d: %s", response.StatusCode, strings.TrimSpace(string(snippet)))
	}

	raw, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil {
		return nil, fmt.Errorf("прочитать уточнение: %w", err)
	}

	var parsed deepSeekResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("разобрать ответ DeepSeek: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, errors.New("DeepSeek вернул пустой choices")
	}

	clarification, err := parseClarificationContent(parsed.Choices[0].Message.Content)
	if err != nil {
		return nil, err
	}
	if clarification != nil {
		log.Printf("clarification: задаю уточняющий вопрос (%d вариантов)", len(clarification.Options))
	}
	return clarification, nil
}

// clarificationUserPrompt передаёт только жалобу, без имени и контактов.
func clarificationUserPrompt(sess *chatSession) string {
	lines := make([]string, 0, 8)
	if sess.isChild() {
		lines = append(lines, "Анкета заполняется на ребёнка (возраст: "+sess.childAge()+")")
	} else {
		lines = append(lines, fmt.Sprintf("Возраст: %d", sess.Age))
	}

	fields := []struct{ step, label string }{
		{stepComplaintCategory, "Категория жалобы"},
		{stepComplaintLocation, "Локализация / уточнение"},
		{stepSymptomCharacter, "Характер и провоцирующий фактор"},
		{stepComplaintDuration, "Как давно началось"},
		{stepComplaintSeverity, "Выраженность"},
	}
	for _, field := range fields {
		if value := strings.TrimSpace(sess.Answers[field.step]); value != "" {
			lines = append(lines, field.label+": "+value)
		}
	}

	lines = append(lines, "Оцени, достаточно ли этих данных для безопасной маршрутизации. Верни JSON по инструкции.")
	return strings.Join(lines, "\n")
}
