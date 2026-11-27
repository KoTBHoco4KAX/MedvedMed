package medved

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// apiError — ошибка, которую можно отдать фронту с нужным HTTP-статусом.
type apiError struct {
	Status  int
	Message string
}

func (e *apiError) Error() string { return e.Message }

func newAPIError(status int, message string) *apiError {
	return &apiError{Status: status, Message: message}
}

// coreUser — подмножество ответа Core, которое нам реально нужно.
type coreUser struct {
	ID        string `json:"_id"`
	BirthDate string `json:"birthDate"`
	Sex       int    `json:"sex"`
	FullName  struct {
		FirstName string `json:"firstName"`
	} `json:"full_name"`
	FullNameCamel struct {
		FirstName string `json:"firstName"`
	} `json:"fullName"`
}

// firstName возвращает имя для приветствия. Если Core не отдал имя —
// возвращается пустая строка (фронт не должен рисовать «Привет, !»).
func (u *coreUser) firstName() string {
	if name := strings.TrimSpace(u.FullName.FirstName); name != "" {
		return name
	}
	return strings.TrimSpace(u.FullNameCamel.FirstName)
}

// fetchCoreUser запрашивает пользователя в Core по токену из мини-аппа MAX.
func (a *app) fetchCoreUser(ctx context.Context, token string) (*coreUser, error) {
	if a.cfg.CoreURL == "" {
		return nil, newAPIError(http.StatusInternalServerError, "CORE_URL не задан в .env")
	}

	endpoint := a.cfg.CoreURL + "/api/v3/users/user-for-external-service?token=" + url.QueryEscape(token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, newAPIError(http.StatusInternalServerError, "не удалось собрать запрос к Core")
	}
	req.Header.Set("Authorization", "Bearer "+a.cfg.CoreServiceToken)
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, newAPIError(http.StatusBadGateway, "Core недоступен, попробуйте позже")
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// дальше разбираем тело
	case http.StatusUnauthorized:
		return nil, newAPIError(http.StatusUnauthorized,
			"Токен MAX недействителен или истёк. Откройте сервис заново через бота в MAX.")
	case http.StatusForbidden:
		return nil, newAPIError(http.StatusForbidden,
			"Сервисный токен Core отклонён. Обратитесь к администратору сервиса.")
	case http.StatusNotFound:
		return nil, newAPIError(http.StatusNotFound,
			"Пользователь не найден в MAX. Откройте сервис через бота.")
	default:
		return nil, newAPIError(http.StatusBadGateway,
			fmt.Sprintf("Core вернул неожиданный статус %d", resp.StatusCode))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, newAPIError(http.StatusBadGateway, "не удалось прочитать ответ Core")
	}

	user, err := decodeCoreUser(body)
	if err != nil {
		return nil, newAPIError(http.StatusBadGateway, "Core вернул некорректный JSON")
	}
	if user.ID == "" {
		return nil, newAPIError(http.StatusBadGateway, "В ответе Core нет идентификатора пользователя")
	}
	return user, nil
}

// decodeCoreUser принимает как «плоский» ответ, так и обёртки user/data/result.
func decodeCoreUser(body []byte) (*coreUser, error) {
	var direct coreUser
	if err := json.Unmarshal(body, &direct); err != nil {
		return nil, err
	}
	if direct.ID != "" || direct.BirthDate != "" {
		return &direct, nil
	}

	var wrapped struct {
		User   *coreUser `json:"user"`
		Data   *coreUser `json:"data"`
		Result *coreUser `json:"result"`
	}
	if err := json.Unmarshal(body, &wrapped); err != nil {
		return nil, err
	}
	for _, candidate := range []*coreUser{wrapped.User, wrapped.Data, wrapped.Result} {
		if candidate != nil && (candidate.ID != "" || candidate.BirthDate != "") {
			return candidate, nil
		}
	}
	return &direct, nil
}

// fullAge считает полное число лет на момент now (дата рождения в формате YYYY-MM-DD).
func fullAge(birthDate string, now time.Time) (int, error) {
	if strings.TrimSpace(birthDate) == "" {
		return 0, nil
	}

	var born time.Time
	var err error
	for _, layout := range []string{"2006-01-02", time.RFC3339, "2006-01-02T15:04:05"} {
		if born, err = time.Parse(layout, birthDate); err == nil {
			break
		}
	}
	if err != nil {
		return 0, fmt.Errorf("некорректная дата рождения %q", birthDate)
	}

	age := now.Year() - born.Year()
	if now.Month() < born.Month() || (now.Month() == born.Month() && now.Day() < born.Day()) {
		age--
	}
	if age < 0 || age > 150 {
		return 0, errors.New("некорректная дата рождения")
	}
	return age, nil
}
