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
	"unicode"
	"unicode/utf8"
)

const (
	deepSeekEndpoint = "https://api.deepseek.com/chat/completions"
	deepSeekModel    = "deepseek-chat"
	llmTimeout       = 20 * time.Second
)

// finalCard — итоговая карточка рекомендаций (структура из ТЗ).
type finalCard struct {
	Specialty       string `json:"specialty"`
	Urgency         string `json:"urgency"`
	Place           string `json:"place"`
	Reasoning       string `json:"reasoning"`
	PaymentNote     string `json:"paymentNote"`
	PreparationTips string `json:"preparationTips"`
}

// Коды срочности: frontend показывает по ним бейдж и тревожный стиль.
const (
	urgencyEmergency = "emergency"
	urgency24h       = "urgent_24h"
	urgencyPlanned   = "planned"
	urgencySelfCare  = "self_care"
)

type deepSeekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type deepSeekRequest struct {
	Model          string            `json:"model"`
	Messages       []deepSeekMessage `json:"messages"`
	ResponseFormat map[string]string `json:"response_format"`
	Temperature    float64           `json:"temperature"`
	MaxTokens      int               `json:"max_tokens"`
	Stream         bool              `json:"stream"`
}

type deepSeekResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

const systemPrompt = `Ты — дружелюбный медицинский ассистент-маршрутизатор сервиса «МедВед». Ты помогаешь человеку понять, к какому врачу и как срочно обратиться, и объясняешь, что подготовить к приёму.

Строгие правила:
1. НИКОГДА не ставь окончательный диагноз и не называй конкретные болезни как установленный факт. Допустимы формулировки «возможно», «может указывать на», «стоит исключить».
2. НЕ назначай лекарства, дозировки, схемы лечения и не рекомендуй новые препараты. Обследования и лечение — только на приёме у врача.
3. Не заменяй врача и скорую помощь. Если в данных есть признаки неотложного состояния — советуй звонить 103/112.
4. Пиши живым, ясным и убедительным языком врача, который объясняет пациенту суть угрозы без канцелярита и запугивания. Сложные термины расшифровывай, конкретные риски называй прямо.
5. Учитывай возраст и пол. Если анкета заполняется на ребёнка, рекомендуй педиатра или детского профильного специалиста, а не взрослого врача.
6. Поле "reasoning" — короткое, спокойное объяснение для пациента, максимум 450–550 символов.
   Обращайся только на «вы»: не пиши «Пациент принимает…» в третьем лице — пиши «Вы принимаете…», «Сочетание ваших препаратов…».
   Структура:
   - первый абзац: 1–2 коротких предложения — главный вывод простыми словами, почему сейчас важно обратиться к врачу;
   - затем 2–3 коротких абзаца с новой строки (разделяй их символом \n): разбери конкретные связки из анкеты — совместимость групп препаратов, перекрёст с аллергией и вредными привычками. Не перечисляй анкету заново, если фактор не влияет на ситуацию.
   Тон — спокойный и поддерживающий, без запугивания. Запрещены формулировки «угрожающее состояние», «источник кровотечения», «опасность для жизни». Вместо них объясняй по делу: «Такое сочетание препаратов сильно нагружает желудок и влияет на свёртываемость крови, поэтому лучше не откладывать осмотр врача».
   Фармакологическая точность сохраняется: называй фармакологические группы (антикоагулянты, НПВС и т. п.), учитывай перекрёстную аллергию (включая вспомогательные компоненты — лецитин, соя, красители, салицилаты) и влияние алкоголя, но переводи это на понятный человеку язык.
7. Поле "specialty" начинай с заглавной буквы: «Врач скорой помощи / токсиколог», «Врач-терапевт», «Детский невролог».
8. Отвечай СТРОГО валидным JSON-объектом без markdown-обёрток и без комментариев, ровно с полями:
{"specialty": "врач или специалист, к которому стоит обратиться", "urgency": "emergency | urgent_24h | planned | self_care", "place": "куда обращаться", "reasoning": "450–550 символов, абзацы через \n", "paymentNote": "совет с учётом ОМС / ДМС / платно", "preparationTips": "два блока через \n: что делать до визита к врачу и что подготовить к приёму"}
Поле "urgency" — строго один из кодов:
- "emergency" — только когда состояние требует немедленного вызова скорой помощи (103/112) прямо сейчас;
- "urgent_24h" — нужен дежурный врач, травмпункт или срочный приём в течение 24 часов;
- "planned" — плановый приём;
- "self_care" — лёгкое бытовое недомогание без красных флагов, которое можно наблюдать дома; лекарства и дозировки не назначай.
Не ставь "emergency" для случаев, которые можно решить у дежурного врача или в травмпункте сегодня — для них нужен "urgent_24h".
9. Если пациент выбрал тяжесть или давящую боль и в анкете нет красных флагов (резкого удара, потери сознания, перекоса лица, нарушения речи или зрения, рвоты, травмы), это НЕ экстренный вызов 103/112. Ставь "self_care", если состояние лёгкое и нет ухудшения, либо "planned" с терапевтом/неврологом; "emergency" — только при реальных признаках неотложного состояния.
10. Поле "preparationTips" — строго два блока через \n:
   1) «Что делать до визита к врачу:» (если urgency="emergency" — «Что делать до приезда скорой:») — конкретные безопасные действия: спокойный режим, достаточное питьё, фиксация температуры/давления, при каких ухудшениях срочно звонить 103/112. Без назначения лекарств и дозировок.
   2) «Что подготовить к приёму:» — документы (паспорт, полис), список принимаемых лекарств, дневник симптомов и результаты обследований.
Оба заголовка пиши дословно, с двоеточием и новой строкой между блоками.`

// generateCard вызывает LLM и при любой проблеме отдаёт карточку из локальных правил.
func (a *app) generateCard(ctx context.Context, sess *chatSession) finalCard {
	fallback := localCard(sess)

	if strings.TrimSpace(a.cfg.DeepSeekAPIKey) == "" {
		log.Printf("llm: DEEPSEEK_API_KEY не задан — карточка собрана локальными правилами")
		return fallback
	}

	card, err := a.requestDeepSeek(ctx, sess)
	if err != nil {
		log.Printf("llm: %v — карточка собрана локальными правилами", err)
		return fallback
	}
	return mergeCard(card, fallback)
}

func (a *app) requestDeepSeek(ctx context.Context, sess *chatSession) (finalCard, error) {
	ctx, cancel := context.WithTimeout(ctx, llmTimeout)
	defer cancel()

	payload := deepSeekRequest{
		Model: deepSeekModel,
		Messages: []deepSeekMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: buildUserPrompt(sess)},
		},
		ResponseFormat: map[string]string{"type": "json_object"},
		Temperature:    0.2,
		MaxTokens:      900,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return finalCard{}, fmt.Errorf("собрать запрос: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deepSeekEndpoint, bytes.NewReader(body))
	if err != nil {
		return finalCard{}, fmt.Errorf("собрать запрос: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.cfg.DeepSeekAPIKey)

	resp, err := a.llm.Do(req)
	if err != nil {
		return finalCard{}, fmt.Errorf("DeepSeek недоступен: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return finalCard{}, fmt.Errorf("DeepSeek вернул %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if err != nil {
		return finalCard{}, fmt.Errorf("прочитать ответ DeepSeek: %w", err)
	}

	var parsed deepSeekResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return finalCard{}, fmt.Errorf("разобрать ответ DeepSeek: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return finalCard{}, errors.New("DeepSeek вернул пустой список choices")
	}

	return parseCardJSON(parsed.Choices[0].Message.Content)
}

// parseCardJSON вытаскивает JSON-объект карточки, даже если модель обернула его в текст.
func parseCardJSON(content string) (finalCard, error) {
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return finalCard{}, errors.New("в ответе модели нет JSON-объекта")
	}

	var card finalCard
	if err := json.Unmarshal([]byte(content[start:end+1]), &card); err != nil {
		return finalCard{}, fmt.Errorf("некорректный JSON карточки: %w", err)
	}
	if strings.TrimSpace(card.Specialty) == "" {
		return finalCard{}, errors.New("в карточке отсутствует specialty")
	}
	return card, nil
}

// mergeCard добивает пустые поля ответа LLM значениями из локальных правил.
func mergeCard(card, fallback finalCard) finalCard {
	merged := finalCard{
		Specialty:       firstNonEmpty(card.Specialty, fallback.Specialty),
		Urgency:         firstNonEmpty(card.Urgency, fallback.Urgency),
		Place:           firstNonEmpty(card.Place, fallback.Place),
		Reasoning:       firstNonEmpty(card.Reasoning, fallback.Reasoning),
		PaymentNote:     firstNonEmpty(card.PaymentNote, fallback.PaymentNote),
		PreparationTips: firstNonEmpty(card.PreparationTips, fallback.PreparationTips),
	}
	merged.Specialty = normalizeSpecialty(merged.Specialty)
	merged.Specialty = truncateRunes(merged.Specialty, 120)
	merged.Urgency = normalizeUrgency(merged.Urgency)
	merged.Urgency = truncateRunes(merged.Urgency, 120)
	merged.Place = truncateRunes(merged.Place, 200)
	merged.Reasoning = truncateRunes(merged.Reasoning, 1500)
	merged.PaymentNote = truncateRunes(merged.PaymentNote, 600)
	merged.PreparationTips = truncateRunes(merged.PreparationTips, 900)
	return merged
}

// normalizeSpecialty делает первую букву названия специальности заглавной:
// «врач скорой помощи / токсиколог» → «Врач скорой помощи / токсиколог».
func normalizeSpecialty(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	runes := []rune(trimmed)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// normalizeUrgency приводит ответ LLM к кодам emergency / urgent_24h / planned,
// даже если модель вернула человекочитаемую формулировку.
func normalizeUrgency(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch {
	case normalized == urgencyEmergency,
		strings.Contains(normalized, "экстрен"),
		strings.Contains(normalized, "немедлен"):
		return urgencyEmergency
	case normalized == urgency24h,
		strings.Contains(normalized, "срочн"),
		strings.Contains(normalized, "24"):
		return urgency24h
	case normalized == urgencySelfCare,
		strings.Contains(normalized, "самопомощ"),
		strings.Contains(normalized, "наблюдение дома"):
		return urgencySelfCare
	case strings.Contains(normalized, "план"):
		return urgencyPlanned
	default:
		return strings.TrimSpace(value)
	}
}

// pediatricSpecialty подбирает детского специалиста под взрослую маршрутизацию.
func pediatricSpecialty(specialty string) string {
	switch specialty {
	case "Врач-невролог":
		return "Детский невролог"
	case "Врач-дерматолог":
		return "Детский дерматолог"
	case "Врач-гастроэнтеролог":
		return "Детский гастроэнтеролог"
	case "Врач-пульмонолог":
		return "Детский пульмонолог"
	case "Врач-оториноларинголог":
		return "Детский оториноларинголог"
	case "Врач-офтальмолог":
		return "Детский офтальмолог"
	case "Врач-травматолог-ортопед":
		return "Детский травматолог-ортопед"
	case "Врач-стоматолог":
		return "Детский стоматолог"
	default:
		return "Врач-педиатр"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func truncateRunes(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	runes := []rune(s)
	return strings.TrimSpace(string(runes[:limit])) + "…"
}

// buildUserPrompt передаёт в LLM только клинически значимые данные, без ФИО и контактов.
func buildUserPrompt(sess *chatSession) string {
	gender := "не указан"
	switch sess.Sex {
	case 1:
		gender = "мужской"
	case 2:
		gender = "женский"
	}

	var lines []string
	if sess.isChild() {
		lines = []string{
			"Анкета заполняется на ребёнка (возраст: " + sess.childAge() + ")",
			"Пол ребёнка: не указан",
			"Заполняет родитель со своего взрослого аккаунта MAX",
		}
	} else {
		lines = []string{
			fmt.Sprintf("Возраст: %d", sess.Age),
			"Пол: " + gender,
		}
	}

	fields := []struct{ step, label string }{
		{stepComplaintCategory, "Основная жалоба"},
		{stepComplaintLocation, "Локализация / уточнение симптома"},
		{stepSymptomCharacter, "Характер симптома и провоцирующий фактор"},
		{stepComplaintDuration, "Как давно началось"},
		{stepComplaintSeverity, "Насколько сильно выражено"},
		{stepAdditionalSymptoms, "Сопутствующие симптомы"},
		{stepChronic, "Хронические заболевания"},
		{stepMedsStatus, "Приём лекарств"},
		{stepMedsDetails, "Какие лекарства"},
		{stepAllergies, "Аллергии"},
		{stepAllergiesDetails, "Детали аллергии"},
		{stepHospitalizations, "Операции и госпитализации"},
		{stepHeredity, "Наследственность"},
		{stepPregnancy, "Беременность / лактация"},
		{stepHabits, "Вредные привычки"},
		{stepPaymentType, "Тип оплаты"},
	}
	for _, field := range fields {
		value := sess.Answers[field.step]
		if field.step == stepHabits {
			value = sess.habitsDisplay()
		}
		if field.step == stepAdditionalSymptoms || field.step == stepChronic {
			value = displayListAnswer(value)
		}
		if strings.TrimSpace(value) != "" {
			lines = append(lines, field.label+": "+value)
		}
	}
	if clarification := parseStoredClarification(sess.Answers); clarification != nil {
		if answer := strings.TrimSpace(sess.Answers[stepAIClarification]); answer != "" {
			lines = append(lines, "Уточняющий вопрос: "+clarification.Question)
			lines = append(lines, "Ответ на уточнение: "+answer)
		}
	}
	lines = append(lines, "Сформируй карточку-маршрут в формате JSON.")
	if sess.isChild() {
		lines = append(lines, "Нужен педиатр или детский профильный специалист: учитывай детский возраст и не переноси взрослые назначения.")
	}
	return strings.Join(lines, "\n")
}

// localCard — детерминированные локальные правила. Гарантирует карточку без LLM.
func localCard(sess *chatSession) finalCard {
	category := sess.Answers[stepComplaintCategory]
	details := sess.complaintDetails()
	chronic := displayListAnswer(sess.Answers[stepChronic])

	specialty := specialtyFor(category, details)
	if sess.isChild() {
		specialty = pediatricSpecialty(specialty)
	}
	urgency := urgencyFor(category, chronic, sess)
	place := placeFor(urgency)
	if sess.isChild() {
		switch urgency {
		case urgency24h:
			place = "Детская поликлиника / дежурный педиатр"
		case urgencyPlanned:
			place = "Детская поликлиника по записи"
		}
	}

	return finalCard{
		Specialty:       specialty,
		Urgency:         urgency,
		Place:           place,
		Reasoning:       reasoningFor(sess, category, details, chronic),
		PaymentNote:     paymentNoteFor(sess.Answers[stepPaymentType]),
		PreparationTips: preparationTipsFor(urgency),
	}
}

// preparationTipsFor собирает два обязательных блока памятки без лекарств и дозировок.
func preparationTipsFor(urgency string) string {
	header := "Что делать до визита к врачу:"
	if urgency == urgencyEmergency {
		header = "Что делать до приезда скорой:"
	}
	return header + "\n" +
		"- Соблюдайте спокойный режим и избегайте физических нагрузок.\n" +
		"- Пейте воду небольшими порциями, если врач не ограничивал питьё.\n" +
		"- Измеряйте температуру и давление, записывайте показатели.\n" +
		"- При ухудшении состояния вызывайте скорую помощь (103/112).\n\n" +
		"Что подготовить к приёму:\n" +
		"- Паспорт и полис ОМС/ДМС.\n" +
		"- Список принимаемых лекарств (названия и дозировки, если знаете).\n" +
		"- Дневник симптомов: когда начались, что усиливает и как менялись.\n" +
		"- Результаты обследований за последний год, если есть."
}

func specialtyFor(category, details string) string {
	text := strings.ToLower(category + " " + details)

	switch {
	case containsAny(text, "зуб", "десн", "стоматолог"):
		return "Врач-стоматолог"
	case containsAny(text, "глаз", "зрен", "офтальм"):
		return "Врач-офтальмолог"
	case containsAny(text, "ухо", "горл", "нос", "отит", "ангин", "лор"):
		return "Врач-оториноларинголог"
	case containsAny(text, "сыпь", "кож", "зуд", "дермат"):
		return "Врач-дерматолог"
	case containsAny(text, "живот", "тошн", "рвот", "понос", "жкт", "желуд", "кишеч"):
		return "Врач-гастроэнтеролог"
	case containsAny(text, "кашел", "дыхан", "бронх", "астм", "одышк"):
		return "Врач-пульмонолог"
	case containsAny(text, "голов", "невр", "онемен", "головокруж", "мигрен"):
		return "Врач-невролог"
	case strings.EqualFold(category, "Травма") || containsAny(text, "травм", "перелом", "растяж", "ушиб"):
		return "Врач-травматолог-ортопед"
	default:
		return "Врач-терапевт"
	}
}

func urgencyFor(category, chronic string, sess *chatSession) string {
	if sess.Answers[stepPregnancy] == "Беременность" {
		return urgency24h
	}
	if strings.Contains(strings.ToLower(chronic), "онколог") {
		return urgency24h
	}
	if strings.EqualFold(category, "Температура") || strings.EqualFold(category, "Травма") {
		return urgency24h
	}
	if strings.EqualFold(category, "Дыхание") && strings.Contains(strings.ToLower(chronic), "астм") {
		return urgency24h
	}
	if isSelfCareCase(sess) {
		return urgencySelfCare
	}
	return urgencyPlanned
}

// isSelfCareCase — бытовая тяжесть/давящая боль без сильной выраженности.
func isSelfCareCase(sess *chatSession) bool {
	location := normalizeText(sess.Answers[stepComplaintLocation])
	character := normalizeText(sess.Answers[stepSymptomCharacter])
	severity := normalizeText(sess.Answers[stepComplaintSeverity])
	if !strings.Contains(location, "голов") {
		return false
	}
	if !strings.Contains(character, "тяжест") || !strings.Contains(character, "давящ") {
		return false
	}
	return !strings.Contains(severity, "сильно") && !strings.Contains(severity, "нестерпим")
}

func placeFor(urgency string) string {
	switch urgency {
	case urgencyEmergency:
		return "Приёмное отделение стационара"
	case urgency24h:
		return "Поликлиника / дежурный врач"
	case urgencySelfCare:
		return "Дома / аптека при необходимости"
	default:
		return "Приём в поликлинике по записи"
	}
}

func reasoningFor(sess *chatSession, category, details, chronic string) string {
	var builder strings.Builder

	if strings.TrimSpace(category) == "" {
		builder.WriteString("Вы описали жалобы, которые стоит обсудить с врачом очно.")
	} else {
		builder.WriteString("Основная жалоба — «" + strings.TrimSpace(category) + "».")
	}
	if strings.TrimSpace(details) != "" {
		builder.WriteString(" Дополнительно вы указали: «" + strings.TrimSpace(details) + "».")
	}
	if chronic != "" && !strings.EqualFold(chronic, chronicNone) {
		builder.WriteString(" В анамнезе важно учесть: " + strings.TrimSpace(chronic) + ".")
	}
	if sess.isChild() {
		builder.WriteString(" Анкета заполнена на ребёнка (" + sess.childAge() + ").")
	} else if sess.Age > 0 {
		builder.WriteString(fmt.Sprintf(" Вам %d %s, и это тоже влияет на выбор врача.", sess.Age, yearsWord(sess.Age)))
	}
	builder.WriteString(" Это не диагноз: такой набор симптомов лучше обсудить с врачом на очном приёме, чтобы он осмотрел вас и при необходимости назначил обследования.")
	return builder.String()
}

func paymentNoteFor(payment string) string {
	switch payment {
	case "ОМС (бесплатно по полису)":
		return "По полису ОМС приём врача бесплатный. Если понадобится профильный специалист, терапевт выпишет направление."
	case "ДМС":
		return "Проверьте условия вашего полиса ДМС: клинику и направление к специалисту обычно согласуют со страховой компанией."
	case "Платно":
		return "Можно записаться напрямую в частную клинику — уточните стоимость приёма и возможных обследований заранее."
	default:
		return "Уточните условия обращения по вашему полису или в выбранной клинике."
	}
}

func containsAny(text string, substrings ...string) bool {
	for _, substring := range substrings {
		if strings.Contains(text, substring) {
			return true
		}
	}
	return false
}

// joinNonEmpty склеивает непустые фрагменты через запятую.
func joinNonEmpty(values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, ", ")
}

func yearsWord(age int) string {
	mod100 := age % 100
	mod10 := age % 10
	switch {
	case mod100 >= 11 && mod100 <= 14:
		return "лет"
	case mod10 == 1:
		return "год"
	case mod10 >= 2 && mod10 <= 4:
		return "года"
	default:
		return "лет"
	}
}
