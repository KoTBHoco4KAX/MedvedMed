package main

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// chatSession — состояние авторизации и опроса одного пользователя.
// В БД лежат только core_user_id, firstName (для UI), возраст, пол и ответы опроса.
type chatSession struct {
	ID              string
	CoreUserID      string
	FirstName       string
	Age             int
	Sex             int
	CurrentStep     string
	History         []string
	Answers         map[string]string
	Messages        []chatMessage
	IsEmergency     bool
	EmergencyReason string
	UpdatedAt       time.Time
}

// chatMessage — элемент ленты диалога. В messages_json попадает полностью.
type chatMessage struct {
	Role string     `json:"role"` // bot | user | emergency | card
	Step string     `json:"step"`
	Text string     `json:"text"`
	At   time.Time  `json:"at"`
	Card *finalCard `json:"card,omitempty"`
}

// lastCard возвращает последнюю сгенерированную карточку (шаг result).
func (s *chatSession) lastCard() *finalCard {
	for i := len(s.Messages) - 1; i >= 0; i-- {
		if s.Messages[i].Role == "card" && s.Messages[i].Card != nil {
			return s.Messages[i].Card
		}
	}
	return nil
}

func randomToken(nBytes int) (string, error) {
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
