package medved

import (
	"context"
	"strings"
	"testing"
)

func TestParseCardJSON(t *testing.T) {
	content := "```json\n{\n  \"specialty\": \"Врач-терапевт\",\n  \"urgency\": \"Срочно (в течение 24 часов)\",\n  \"place\": \"Поликлиника / дежурный врач\",\n  \"reasoning\": \"Возможно, стоит исключить воспаление.\",\n  \"paymentNote\": \"По полису ОМС бесплатно.\",\n  \"preparationTips\": \"Возьмите полис и список лекарств.\"\n}\n```"

	card, err := parseCardJSON(content)
	if err != nil {
		t.Fatalf("parseCardJSON: %v", err)
	}
	if card.Specialty != "Врач-терапевт" || card.PaymentNote == "" || card.PreparationTips == "" {
		t.Fatalf("карточка разобрана неполно: %+v", card)
	}
}

func TestParseCardJSONRejectsGarbage(t *testing.T) {
	if _, err := parseCardJSON("извините, не могу"); err == nil {
		t.Fatal("ожидалась ошибка на ответе без JSON")
	}
	if _, err := parseCardJSON(`{"urgency": "срочно"}`); err == nil {
		t.Fatal("ожидалась ошибка: в карточке нет specialty")
	}
}

func TestMergeCardFillsEmptyFields(t *testing.T) {
	fallback := finalCard{
		Specialty:       "Врач-терапевт",
		Urgency:         urgencyPlanned,
		Place:           "Приём в поликлинике по записи",
		Reasoning:       "локальное обоснование",
		PaymentNote:     "локальный совет",
		PreparationTips: "локальные советы",
	}
	merged := mergeCard(finalCard{Specialty: "Врач-невролог"}, fallback)

	if merged.Specialty != "Врач-невролог" || merged.Urgency != urgencyPlanned || merged.Reasoning != fallback.Reasoning {
		t.Fatalf("mergeCard собрал карточку неверно: %+v", merged)
	}
}

func TestLocalCardForPregnantWoman(t *testing.T) {
	sess := &chatSession{
		Age: 30,
		Sex: 2,
		Answers: map[string]string{
			stepComplaintCategory: "Боль",
			stepComplaintLocation: "Спина / поясница",
			stepComplaintDuration: "1–3 дня",
			stepComplaintSeverity: "Умеренно",
			stepChronic:           "Нет хронических",
			stepPregnancy:         "Беременность",
			stepPaymentType:       "ОМС (бесплатно по полису)",
		},
	}

	card := localCard(sess)
	if card.Specialty == "" || card.Place == "" || card.Reasoning == "" || card.PaymentNote == "" || card.PreparationTips == "" {
		t.Fatalf("локальная карточка неполная: %+v", card)
	}
	if card.Urgency != urgency24h {
		t.Fatalf("для беременной ожидалась срочность %q, получено %q", urgency24h, card.Urgency)
	}
	if !strings.Contains(card.Reasoning, "не диагноз") {
		t.Fatalf("в обосновании должен быть дисклеймер «это не диагноз»: %q", card.Reasoning)
	}
}

func TestNormalizeUrgency(t *testing.T) {
	cases := map[string]string{
		"emergency": urgencyEmergency,
		"Экстренно (в течение 1–2 часов)": urgencyEmergency,
		"Срочно (в течение 24 часов)":     urgency24h,
		"urgent_24h": urgency24h,
		"planned":    urgencyPlanned,
		"Планово (в течение 3–5 дней)": urgencyPlanned,
		"self_care": urgencySelfCare,
		"Самопомощь / наблюдение дома": urgencySelfCare,
		"Наблюдение дома":              urgencySelfCare,
	}
	for source, want := range cases {
		if got := normalizeUrgency(source); got != want {
			t.Fatalf("normalizeUrgency(%q) = %q, ожидалось %q", source, got, want)
		}
	}
}

func TestLocalCardForChild(t *testing.T) {
	sess := &chatSession{
		Age: 38,
		Sex: 2,
		Answers: map[string]string{
			stepDisclaimer:        disclaimerChildOption,
			stepChildAge:          "4–11 лет",
			stepComplaintCategory: "Температура",
			stepPaymentType:       "ОМС (бесплатно по полису)",
		},
	}

	card := localCard(sess)
	if card.Specialty != "Врач-педиатр" {
		t.Fatalf("для ребёнка ожидался педиатр, получено %q", card.Specialty)
	}
	if card.Urgency != urgency24h {
		t.Fatalf("для температуры ребёнка ожидалась срочность %q, получено %q", urgency24h, card.Urgency)
	}
	if !strings.Contains(card.Place, "Детская поликлиника") {
		t.Fatalf("место для ребёнка: %q", card.Place)
	}
	if !strings.Contains(card.Reasoning, "ребёнка") {
		t.Fatalf("в обосновании нет упоминания ребёнка: %q", card.Reasoning)
	}
}

func TestBuildUserPromptForChild(t *testing.T) {
	sess := &chatSession{
		Age: 38,
		Sex: 2,
		Answers: map[string]string{
			stepDisclaimer:        disclaimerChildOption,
			stepChildAge:          "12–17 лет",
			stepComplaintCategory: "Температура",
		},
	}

	prompt := buildUserPrompt(sess)
	for _, want := range []string{"на ребёнка", "12–17 лет", "педиатр"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("в промпте нет %q:\n%s", want, prompt)
		}
	}
}

func TestNormalizeSpecialtyCapitalized(t *testing.T) {
	cases := map[string]string{
		"врач скорой помощи / токсиколог": "Врач скорой помощи / токсиколог",
		"Врач-терапевт":                   "Врач-терапевт",
		"  детский невролог ":             "Детский невролог",
		"":                                "",
	}
	for source, want := range cases {
		if got := normalizeSpecialty(source); got != want {
			t.Fatalf("normalizeSpecialty(%q) = %q, ожидалось %q", source, got, want)
		}
	}

	merged := mergeCard(
		finalCard{Specialty: "врач скорой помощи / токсиколог"},
		finalCard{Specialty: "Врач-терапевт"},
	)
	if merged.Specialty != "Врач скорой помощи / токсиколог" {
		t.Fatalf("mergeCard не исправил регистр специальности: %q", merged.Specialty)
	}
}

func TestSystemPromptRequiresInteractionAnalysis(t *testing.T) {
	for _, want := range []string{
		"фармакологические группы",
		"НПВС",
		"лецитин",
		"салицилаты",
		"заглавной буквы",
		"self_care",
		"450–550",
		"Что делать до визита к врачу:",
		"Что подготовить к приёму:",
		"только на «вы»",
		"Пациент принимает",
		"угрожающее состояние",
		"опасность для жизни",
		"спокойный и поддерживающий",
		"\\n",
	} {
		if !strings.Contains(systemPrompt, want) {
			t.Fatalf("в системном промпте нет %q", want)
		}
	}
}

func TestLocalCardPreparationTipsHasTwoBlocks(t *testing.T) {
	sess := &chatSession{
		Age: 30,
		Sex: 1,
		Answers: map[string]string{
			stepComplaintCategory: "Боль",
			stepComplaintLocation: "Голова",
			stepPaymentType:       "ОМС (бесплатно по полису)",
		},
	}
	tips := localCard(sess).PreparationTips
	for _, want := range []string{"Что делать до визита к врачу:", "Что подготовить к приёму:"} {
		if !strings.Contains(tips, want) {
			t.Fatalf("в памятке нет блока %q: %q", want, tips)
		}
	}
	if strings.Contains(strings.ToLower(tips), "принимайте") || strings.Contains(strings.ToLower(tips), "назначьте") {
		t.Fatalf("памятка не должна назначать лечение: %q", tips)
	}
}

func TestGenerateCardWithoutAPIKey(t *testing.T) {
	a := &app{cfg: config{}} // ключ пустой, LLM не вызывается
	sess := &chatSession{
		Age: 32,
		Sex: 1,
		Answers: map[string]string{
			stepComplaintCategory: "Температура",
			stepPaymentType:       "Платно",
		},
	}

	card := a.generateCard(context.Background(), sess)
	if card.Specialty == "" || card.Urgency == "" || card.Place == "" || card.Reasoning == "" {
		t.Fatalf("фоллбэк без ключа вернул неполную карточку: %+v", card)
	}
	if !strings.Contains(card.PaymentNote, "частную клинику") {
		t.Fatalf("совет по оплате не учёл «Платно»: %q", card.PaymentNote)
	}
}
