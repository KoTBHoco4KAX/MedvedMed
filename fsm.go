package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	stepDisclaimer         = "disclaimer"
	stepChildAge           = "child_age"
	stepComplaintCategory  = "complaint_category"
	stepComplaintLocation  = "complaint_location"
	stepSymptomCharacter   = "symptom_character"
	stepComplaintDuration  = "complaint_duration"
	stepComplaintSeverity  = "complaint_severity"
	stepAdditionalSymptoms = "additional_symptoms"
	stepChronic            = "chronic"
	stepMedsStatus         = "meds_status"
	stepMedsDetails        = "meds_details"
	stepAllergies          = "allergies"
	stepAllergiesDetails   = "allergies_details"
	stepHospitalizations   = "hospitalizations"
	stepHeredity           = "heredity"
	stepPregnancy          = "pregnancy"
	stepHabits             = "habits"
	stepPaymentType        = "payment_type"
	stepResult             = "result"
	stepEmergency          = "emergency"
)

// Варианты шага meds_status: от них зависит наличие шага meds_details.
const (
	medsNone       = "Не принимаю вообще"
	medsRegular    = "Принимаю системно (постоянно)"
	medsOccasional = "Принимаю периодически (по ситуации)"
)

// Варианты шагов блока жалоб, которые важны для детектора красных флагов.
const (
	durationSudden     = "Только что (меньше часа назад)"
	durationToday      = "Сегодня (несколько часов)"
	severityStrong     = "Сильно"
	severityUnbearable = "Нестерпимо / острая"
	additionalNone     = "Нет, больше ничего"
	additionalDone     = "На этом всё"
	chronicNone        = "Нет хронических"
	chronicDone        = "На этом всё"
)

// Варианты шага аллергий и цикла вредных привычек.
const (
	allergyNone = "Аллергий нет"

	habitsNone             = "Нет вредных привычек"
	habitsDone             = "На этом всё"
	habitsIntroQuestion    = "Есть ли у вас вредные привычки или факторы образа жизни из списка ниже?"
	habitsContinueQuestion = "Есть ли ещё какие-то вредные привычки?"
)

// Режим «заполняю за ребёнка»: вторая кнопка дисклеймера и возраст ребёнка.
const (
	disclaimerStartOption = "Понятно, начать опрос"
	disclaimerChildOption = "Заполняю за ребёнка (до 18 лет)"
)

var childAgeOptions = []string{"До 1 года", "1–3 года", "4–12 лет", "13–17 лет"}

const (
	disclaimerText = "Бот не ставит диагноз и не заменяет врача. Я задам несколько вопросов о самочувствии, чтобы подсказать, к какому специалисту и как срочно лучше обратиться."

	finalDisclaimer = "При ухудшении состояния немедленно вызывайте скорую помощь (103/112)."

	doneText = "Готово — я собрал рекомендации по вашему состоянию."
)

// stepSpec — один шаг опроса. Label используется в истории обращений.
type stepSpec struct {
	ID            string
	Label         string
	Question      string
	Options       []string
	AllowFreeText bool
}

var (
	complaintCategories = []string{"Боль", "Температура", "Дыхание", "ЖКТ", "Кожа", "Травма", "Головокружение, онемение, сон"}

	complaintDurations = []string{
		"Только что (меньше часа назад)",
		"Сегодня (несколько часов)",
		"1–3 дня",
		"Больше 3 дней",
		"Периодически повторяется",
	}

	severityOptions = []string{
		"Слабо, почти не мешает",
		"Умеренно",
		severityStrong,
		severityUnbearable,
	}

	painLocations = []string{"Голова", "Грудь / грудная клетка", "Горло / ухо", "Живот", "Спина / поясница", "Суставы / конечности"}

	temperatureOptions = []string{
		"37.0–37.9 °C",
		"38.0–38.9 °C",
		"39.0–39.5 °C",
		"Выше 39.5 °C (39,8 °C)",
	}

	symptomCharacterPain = []string{
		"Тяжесть / давящая боль после сна, усталости или стресса",
		"Ноющая / тупая, нарастала постепенно",
		"Пульсирующая",
		"Резкая, острая, возникла внезапно как удар",
		"После травмы / ушиба",
	}

	symptomCharacterTemperature = []string{
		"Жар и ломота во всём теле",
		"Озноб, трясёт",
		"Температура с кашлем и насморком",
		"Температура с сыпью",
		"Только температура, других симптомов нет",
	}

	symptomCharacterBreathing = []string{
		"После переохлаждения / простуды",
		"Сухой, приступообразный",
		"Влажный (с мокротой / выделениями)",
		"Усиливается при контакте с пылью/аллергенами",
		"Появилось внезапно без видимой причины",
	}

	symptomCharacterDigestive = []string{
		"После еды или новой/некачественной пищи",
		"Натощак или ночью",
		"Спазмы / режущая боль",
		"Тянущая, ноющая тяжесть",
		"Кровь в стуле или рвотных массах",
	}

	symptomCharacterSkin = []string{
		"После новой еды, лекарств или косметики",
		"После укуса насекомого или контакта с растением",
		"Связано с температурой / простудой",
		"Меняет цвет, форму или быстро растёт",
		"Появилось постепенно без видимой причины",
	}

	symptomCharacterNeuro = []string{
		"После недосыпа, стресса или усталости",
		"При резком вставании или повороте головы",
		"Постоянное, мешает обычным делам",
		"Появилось внезапно, резко усиливается",
	}

	symptomCharacterTrauma = []string{
		"Сильный отёк или невозможно наступить/пошевелить",
		"Небольшая боль, кожа не повреждена",
		"Кровотечение не останавливается",
		"Падение с высоты",
		"После удара кружится голова или тошнит",
	}

	symptomCharacterOther = []string{
		"Общая слабость или усталость",
		"Снижение веса",
		"Повышенная потливость",
		"Боль без чёткой локализации",
	}

	chronicOptions = []string{chronicNone, "Гипертония / ССЗ", "Диабет", "Астма / ХОБЛ", "Онкология"}
	chronicFactors = []string{"Гипертония / ССЗ", "Диабет", "Астма / ХОБЛ", "Онкология"}

	medsStatusOptions = []string{medsNone, medsRegular, medsOccasional}

	allergyOptions = []string{allergyNone, "Есть аллергия на лекарства", "Пищевая / бытовая аллергия"}

	// Атомарные факторы риска по 1НФ: один вариант — одна привычка.
	habitRiskFactors = []string{
		"Курение / вейп",
		"Употребление алкоголя",
		"Употребление наркотиков / ПАВ",
		"Недосып / хронический стресс",
		"Малоподвижный образ жизни",
	}

	hospitalizationOptions = []string{"Нет, не было", "Да, была операция / госпитализация"}

	heredityOptions = []string{"Нет, наследственность не отягощена", "Да, есть (ССЗ / диабет / онкология)", "Не знаю"}

	pregnancyOptions = []string{"Нет", "Беременность", "Грудное вскармливание"}

	paymentOptions = []string{"ОМС (бесплатно по полису)", "ДМС", "Платно"}
)

// stepsFor строит маршрут опроса для конкретного пациента.
// Условные шаги: meds_details (зависит от meds_status) и pregnancy (пол и возраст).
func stepsFor(sex, age int, answers map[string]string) []stepSpec {
	steps := []stepSpec{
		{
			ID:       stepDisclaimer,
			Label:    "Начало опроса",
			Question: disclaimerText,
			Options:  []string{disclaimerStartOption, disclaimerChildOption},
		},
		{
			ID:            stepComplaintCategory,
			Label:         "Основная жалоба",
			Question:      "Что беспокоит вас больше всего? Выберите основную жалобу или опишите её своими словами.",
			Options:       complaintCategories,
			AllowFreeText: true,
		},
		complaintLocationStep(answers[stepComplaintCategory]),
		symptomCharacterStep(answers[stepComplaintCategory]),
		{
			ID:            stepComplaintDuration,
			Label:         "Как давно началось",
			Question:      "Как давно это началось?",
			Options:       complaintDurations,
			AllowFreeText: true,
		},
		{
			ID:            stepComplaintSeverity,
			Label:         "Насколько сильно выражено",
			Question:      "Насколько сильно это выражено?",
			Options:       severityOptions,
			AllowFreeText: true,
		},
		additionalSymptomsStep(answers),
		chronicStep(answers),
		{
			ID:       stepMedsStatus,
			Label:    "Приём лекарств",
			Question: "Принимаете ли вы сейчас какие-либо лекарства?",
			Options:  medsStatusOptions,
		},
	}

	// При температуре вопрос «насколько сильно это выражено» не имеет смысла:
	// после давности сразу переходим к сопутствующим симптомам.
	if isTemperatureComplaint(answers[stepComplaintCategory]) {
		steps = removeStepByID(steps, stepComplaintSeverity)
	}

	// Режим «заполняю за ребёнка»: сразу после дисклеймера уточняем возраст ребёнка.
	if isChildProfile(answers) {
		childStep := stepSpec{
			ID:       stepChildAge,
			Label:    "Возраст ребёнка",
			Question: "Укажите возраст ребёнка:",
			Options:  childAgeOptions,
		}
		steps = append([]stepSpec{steps[0], childStep}, steps[1:]...)
	}

	if medsDetailsNeeded(answers) {
		steps = append(steps, stepSpec{
			ID:            stepMedsDetails,
			Label:         "Какие лекарства",
			Question:      "Какие именно препараты вы принимаете?",
			Options:       []string{"Не помню название"},
			AllowFreeText: true,
		})
	}

	steps = append(steps,
		stepSpec{
			ID:            stepAllergies,
			Label:         "Аллергии",
			Question:      "Есть ли у вас аллергия на лекарства, продукты или что-то ещё?",
			Options:       allergyOptions,
			AllowFreeText: true,
		},
	)

	if allergiesDetailsNeeded(answers) {
		steps = append(steps, stepSpec{
			ID:            stepAllergiesDetails,
			Label:         "На что аллергия",
			Question:      "На что именно у вас аллергия и как она проявляется?",
			Options:       []string{"Не помню / затрудняюсь ответить"},
			AllowFreeText: true,
		})
	}

	steps = append(steps,
		stepSpec{
			ID:            stepHospitalizations,
			Label:         "Операции и госпитализации",
			Question:      "Были ли у вас операции или госпитализации за последний год?",
			Options:       hospitalizationOptions,
			AllowFreeText: true,
		},
		stepSpec{
			ID:            stepHeredity,
			Label:         "Наследственность",
			Question:      "Есть ли у близких родственников тяжёлые наследственные заболевания (инфаркт/инсульт, диабет, онкология)?",
			Options:       heredityOptions,
			AllowFreeText: true,
		},
	)

	if sex == 2 && age >= 14 && age <= 50 && !isChildProfile(answers) {
		steps = append(steps, stepSpec{
			ID:       stepPregnancy,
			Label:    "Беременность / лактация",
			Question: "Уточните, пожалуйста, есть ли сейчас беременность или грудное вскармливание?",
			Options:  pregnancyOptions,
		})
	}

	steps = append(steps,
		habitsStep(answers),
		stepSpec{
			ID:       stepPaymentType,
			Label:    "Тип оплаты",
			Question: "Как вы планируете обращаться за помощью?",
			Options:  paymentOptions,
		},
		stepSpec{
			ID:       stepResult,
			Label:    "Результат",
			Question: finalDisclaimer,
		},
	)

	// Адаптивный уточняющий вопрос DeepSeek встаёт сразу после базовых вопросов о жалобе.
	if clarification := parseStoredClarification(answers); clarification != nil {
		anchor := stepComplaintSeverity
		if _, ok := findStep(steps, anchor); !ok {
			anchor = stepComplaintDuration
		}
		steps = insertStepAfter(steps, anchor, clarificationStep(clarification))
	}

	return steps
}

// complaintLocationStep — шаг уточнения: для температуры спрашиваем градусы,
// для боли — локализацию, для остальных категорий — подсказки под жалобу.
func complaintLocationStep(category string) stepSpec {
	if isTemperatureComplaint(category) {
		return stepSpec{
			ID:            stepComplaintLocation,
			Label:         "Температура тела",
			Question:      "Какая сейчас температура тела? Укажите в °C или выберите вариант:",
			Options:       temperatureOptions,
			AllowFreeText: true,
		}
	}

	if isPainComplaint(category) {
		return stepSpec{
			ID:            stepComplaintLocation,
			Label:         "Где болит",
			Question:      "Где именно болит?",
			Options:       painLocations,
			AllowFreeText: true,
		}
	}

	return stepSpec{
		ID:            stepComplaintLocation,
		Label:         "Уточнение симптома",
		Question:      "Уточните, что именно и где беспокоит?",
		Options:       locationHintsFor(category),
		AllowFreeText: true,
	}
}

// symptomCharacterStep — характер симптома и триггер; кнопки зависят от категории.
func symptomCharacterStep(category string) stepSpec {
	return stepSpec{
		ID:            stepSymptomCharacter,
		Label:         "Характер симптома",
		Question:      "Какой характер у симптома и что его провоцирует?",
		Options:       symptomCharacterOptions(category),
		AllowFreeText: true,
	}
}

func symptomCharacterOptions(category string) []string {
	switch {
	case isTemperatureComplaint(category):
		return symptomCharacterTemperature
	case isPainComplaint(category):
		return symptomCharacterPain
	}

	switch strings.ToLower(strings.TrimSpace(category)) {
	case "дыхание":
		return symptomCharacterBreathing
	case "жкт":
		return symptomCharacterDigestive
	case "кожа":
		return symptomCharacterSkin
	case "головокружение, онемение, сон":
		return symptomCharacterNeuro
	case "травма":
		return symptomCharacterTrauma
	default:
		return symptomCharacterOther
	}
}

// isPainComplaint распознаёт жалобу про боль, в том числе в свободном тексте.
func isPainComplaint(category string) bool {
	normalized := normalizeText(category)
	return strings.Contains(normalized, "боль") ||
		strings.Contains(normalized, "болит") ||
		strings.Contains(normalized, "болев")
}

// isTemperatureComplaint распознаёт жалобу про температуру/жар,
// чтобы спросить градусы и не задавать вопрос про «остроту».
func isTemperatureComplaint(category string) bool {
	normalized := normalizeText(category)
	return strings.Contains(normalized, "температур") ||
		strings.Contains(normalized, "жар") ||
		strings.Contains(normalized, "лихорадк") ||
		strings.Contains(normalized, "градус")
}

func locationHintsFor(category string) []string {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "дыхание":
		return []string{"Кашель", "Заложенность носа / насморк", "Першение или боль в горле", "Одышка при нагрузке"}
	case "жкт":
		return []string{"Тошнота или рвота", "Диарея", "Запор или вздутие", "Боль / тяжесть в животе"}
	case "кожа":
		return []string{"Сыпь", "Зуд или покраснение", "Пятно или новообразование", "Волдыри / шелушение"}
	case "головокружение, онемение, сон":
		return []string{"Головокружение", "Онемение или покалывание", "Нарушение сна / бессонница", "Слабость в руке/ноге или нарушение речи"}
	case "травма":
		return []string{"Ушиб", "Растяжение / подвернул сустав", "Порез или рана", "Подозрение на перелом", "Удар по голове"}
	default:
		return []string{"Общая слабость", "Утомляемость", "Снижение веса"}
	}
}

// medsDetailsNeeded включает шаг meds_details только для тех, кто принимает лекарства.
func medsDetailsNeeded(answers map[string]string) bool {
	status := answers[stepMedsStatus]
	return status == medsRegular || status == medsOccasional
}

// allergiesDetailsNeeded включает шаг allergies_details для любого ответа, кроме «Аллергий нет».
func allergiesDetailsNeeded(answers map[string]string) bool {
	answer := strings.TrimSpace(answers[stepAllergies])
	return answer != "" && !strings.EqualFold(answer, allergyNone)
}

// habitsStep собирает шаг привычек: первый выбор из атомарных факторов,
// дальше — итеративный добор с «На этом всё».
func habitsStep(answers map[string]string) stepSpec {
	habits := parseHabitList(answers[stepHabits])
	if len(habits) == 0 {
		options := make([]string, 0, len(habitRiskFactors)+1)
		options = append(options, habitsNone)
		options = append(options, habitRiskFactors...)
		return stepSpec{
			ID:            stepHabits,
			Label:         "Вредные привычки",
			Question:      habitsIntroQuestion,
			Options:       options,
			AllowFreeText: true,
		}
	}

	options := make([]string, 0, len(habitRiskFactors))
	options = append(options, habitsDone)
	options = append(options, remainingHabitOptions(habits)...)
	return stepSpec{
		ID:            stepHabits,
		Label:         "Вредные привычки",
		Question:      habitsContinueQuestion,
		Options:       options,
		AllowFreeText: true,
	}
}

// parseHabitList разбирает сохранённый список привычек (JSON) с фоллбэком
// на старый формат через запятую.
func parseHabitList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == habitsNone {
		return nil
	}

	var list []string
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		list = strings.Split(raw, ", ")
	}

	out := make([]string, 0, len(list))
	for _, item := range list {
		item = strings.TrimSpace(item)
		if item != "" && item != habitsNone && item != habitsDone && !containsString(out, item) {
			out = append(out, item)
		}
	}
	return out
}

func (s *chatSession) habitList() []string {
	return parseHabitList(s.Answers[stepHabits])
}

// habitsDisplay — человекочитаемая строка привычек для истории и промпта LLM.
func (s *chatSession) habitsDisplay() string {
	raw := strings.TrimSpace(s.Answers[stepHabits])
	if raw == "" {
		return ""
	}
	if raw == habitsNone {
		return habitsNone
	}
	if habits := s.habitList(); len(habits) > 0 {
		return strings.Join(habits, ", ")
	}
	return raw
}

func remainingHabitOptions(selected []string) []string {
	out := make([]string, 0, len(habitRiskFactors))
	for _, factor := range habitRiskFactors {
		if !containsString(selected, factor) {
			out = append(out, factor)
		}
	}
	return out
}

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func encodeHabitList(habits []string) string {
	buf, err := json.Marshal(habits)
	if err != nil {
		return strings.Join(habits, ", ")
	}
	return string(buf)
}

func (s *chatSession) steps() []stepSpec {
	return stepsFor(s.Sex, s.Age, s.Answers)
}

// isChildProfile — анкета заполняется на ребёнка: выбрана вторая кнопка дисклеймера
// или уже указан возраст ребёнка.
func isChildProfile(answers map[string]string) bool {
	return strings.TrimSpace(answers[stepChildAge]) != "" || answers[stepDisclaimer] == disclaimerChildOption
}

func (s *chatSession) isChild() bool {
	return isChildProfile(s.Answers)
}

func (s *chatSession) childAge() string {
	return strings.TrimSpace(s.Answers[stepChildAge])
}

func findStep(steps []stepSpec, id string) (stepSpec, bool) {
	for _, step := range steps {
		if step.ID == id {
			return step, true
		}
	}
	return stepSpec{}, false
}

func nextStepID(steps []stepSpec, current string) string {
	for i, step := range steps {
		if step.ID == current && i+1 < len(steps) {
			return steps[i+1].ID
		}
	}
	return ""
}

// removeStepByID убирает условный шаг из маршрута, сохраняя порядок остальных.
func removeStepByID(steps []stepSpec, id string) []stepSpec {
	filtered := make([]stepSpec, 0, len(steps))
	for _, step := range steps {
		if step.ID != id {
			filtered = append(filtered, step)
		}
	}
	return filtered
}

// insertStepAfter вставляет шаг сразу после указанного.
func insertStepAfter(steps []stepSpec, anchorID string, step stepSpec) []stepSpec {
	for index, current := range steps {
		if current.ID != anchorID {
			continue
		}
		inserted := make([]stepSpec, 0, len(steps)+1)
		inserted = append(inserted, steps[:index+1]...)
		inserted = append(inserted, step)
		inserted = append(inserted, steps[index+1:]...)
		return inserted
	}
	return steps
}

// initSession переводит сессию в начало опроса.
func initSession(sess *chatSession) {
	sess.CurrentStep = stepDisclaimer
	sess.History = []string{}
	sess.Answers = map[string]string{}
	sess.IsEmergency = false
	sess.EmergencyReason = ""
	sess.Messages = []chatMessage{botMessage(stepDisclaimer, disclaimerText)}
}

func botMessage(step, text string) chatMessage {
	return chatMessage{Role: "bot", Step: step, Text: text, At: time.Now()}
}

// progress — «шаг X из Y» с учётом условных шагов (meds_details, pregnancy).
type progress struct {
	Current int `json:"current"`
	Total   int `json:"total"`
}

func progressFor(steps []stepSpec, step string, answered int) progress {
	total := len(steps)
	for i, s := range steps {
		if s.ID == step {
			return progress{Current: i + 1, Total: total}
		}
	}
	// emergency: показываем, сколько шагов успели пройти.
	current := answered + 1
	if current > total {
		current = total
	}
	if current < 1 {
		current = 1
	}
	return progress{Current: current, Total: total}
}

// chatState — ответ всех /api/chat/* эндпоинтов.
type chatState struct {
	SessionToken         string        `json:"sessionToken"`
	FirstName            string        `json:"firstName"`
	Age                  int           `json:"age"`
	Sex                  int           `json:"sex"`
	Step                 string        `json:"step"`
	Question             string        `json:"question"`
	Options              []string      `json:"options"`
	AllowFreeText        bool          `json:"allowFreeText"`
	CanGoBack            bool          `json:"canGoBack"`
	Progress             progress      `json:"progress"`
	Messages             []chatMessage `json:"messages"`
	IsEmergency          bool          `json:"isEmergency"`
	EmergencyReason      string        `json:"emergencyReason,omitempty"`
	EmergencyInstruction string        `json:"emergencyInstruction,omitempty"`
	IsChild              bool          `json:"isChild"`
	ChildAge             string        `json:"childAge,omitempty"`
	Completed            bool          `json:"completed"`
	Card                 *finalCard    `json:"card,omitempty"`
}

func (s *chatSession) state() chatState {
	steps := s.steps()
	st := chatState{
		SessionToken: s.ID,
		FirstName:    s.FirstName,
		Age:          s.Age,
		Sex:          s.Sex,
		Step:         s.CurrentStep,
		Progress:     progressFor(steps, s.CurrentStep, len(s.Answers)),
		Messages:     s.Messages,
		IsEmergency:  s.IsEmergency,
		IsChild:      s.isChild(),
		ChildAge:     s.childAge(),
	}
	if st.Messages == nil {
		st.Messages = []chatMessage{}
	}

	if s.IsEmergency {
		st.Step = stepEmergency
		st.Question = emergencyInstruction
		st.Options = []string{}
		st.EmergencyInstruction = emergencyInstruction
		st.EmergencyReason = s.EmergencyReason
		st.CanGoBack = false
		return st
	}

	st.EmergencyReason = ""

	if s.CurrentStep == stepResult {
		st.Completed = true
		st.CanGoBack = len(s.History) > 0
		st.Question = finalDisclaimer
		st.Options = []string{}
		st.Card = s.lastCard()
		return st
	}

	if spec, ok := findStep(steps, s.CurrentStep); ok {
		st.Question = spec.Question
		if spec.Options != nil {
			st.Options = spec.Options
		} else {
			st.Options = []string{}
		}
		st.AllowFreeText = spec.AllowFreeText
	} else {
		st.Options = []string{}
	}
	st.CanGoBack = len(s.History) > 0
	return st
}

// answer принимает ответ пользователя, проверяет красные флаги, двигает FSM
// и на финале собирает карточку через LLM (или локальный фоллбэк).
func (a *app) answer(ctx context.Context, sess *chatSession, raw string) (chatState, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return chatState{}, newAPIError(400, "Ответ не может быть пустым.")
	}
	if utf8.RuneCountInString(text) > 2000 {
		return chatState{}, newAPIError(400, "Ответ слишком длинный — до 2000 символов.")
	}

	if sess.IsEmergency {
		return chatState{}, newAPIError(409,
			"Опрос прерван из-за неотложного состояния. Начните новый опрос (POST /api/chat/reset).")
	}
	if sess.CurrentStep == stepResult {
		return chatState{}, newAPIError(409,
			"Опрос уже завершён. Начните новый опрос (POST /api/chat/reset).")
	}

	spec, ok := findStep(sess.steps(), sess.CurrentStep)
	if !ok {
		return chatState{}, newAPIError(500, "Неизвестный шаг опроса.")
	}

	now := time.Now()

	// 1) Мгновенные красные флаги проверяем только на шагах текущих жалоб:
	//    описание прошлых аллергий, болезней и операций не должно обрывать опрос —
	//    эти данные уходят в финальный промпт DeepSeek для комплексной оценки.
	if isComplaintBlockStep(sess.CurrentStep) {
		emergencyText := text
		// На уточнении DeepSeek ответ «Да, кратковременно» сам по себе не содержит
		// слов красного флага — оцениваем его вместе с заданным вопросом.
		if sess.CurrentStep == stepAIClarification && isAffirmativeClarificationAnswer(text) {
			if clarification := parseStoredClarification(sess.Answers); clarification != nil {
				emergencyText = clarification.Question + " " + text
			}
		}
		if hit, found := detectEmergency(emergencyText, sess.Age, sess.isChild()); found {
			sess.Messages = append(sess.Messages, chatMessage{
				Role: "user", Step: sess.CurrentStep, Text: text, At: now,
			})
			return a.enterEmergency(sess, hit, now)
		}
	}

	// 2) Обычный ответ: на шагах без свободного ввода принимаем только варианты.
	if !spec.AllowFreeText {
		canonical, ok := matchOption(spec.Options, text)
		if !ok {
			return chatState{}, newAPIError(400,
				fmt.Sprintf("Выберите один из вариантов: %s.", strings.Join(spec.Options, "; ")))
		}
		text = canonical
	}

	// Шаг привычек — цикл: ответ не двигает шаг линейно, а копит привычки.
	if sess.CurrentStep == stepHabits {
		return a.answerHabits(sess, text, now)
	}
	// Фоновый анамнез и сопутствующие симптомы — мультивыбор с «На этом всё».
	if sess.CurrentStep == stepAdditionalSymptoms {
		return a.answerAdditional(sess, text, now)
	}
	if sess.CurrentStep == stepChronic {
		return a.answerChronic(sess, text, now)
	}

	sess.Answers[sess.CurrentStep] = text
	if sess.CurrentStep == stepComplaintCategory && isTemperatureComplaint(text) {
		// У температуры нет шага «насколько сильно» — старый ответ не должен остаться в промпте.
		delete(sess.Answers, stepComplaintSeverity)
	}
	if isBasicComplaintStep(sess.CurrentStep) {
		// Базовые ответы изменились — сгенерированный ранее вопрос устарел.
		clearClarification(sess)
	}
	sess.History = append(sess.History, sess.CurrentStep)
	sess.Messages = append(sess.Messages, chatMessage{
		Role: "user", Step: sess.CurrentStep, Text: text, At: now,
	})

	// 3) Контекстные красные флаги в блоке жалоб: иррадиация + усиливающий фактор.
	//    Ответ уже сохранён, поэтому детектор видит весь накопленный контекст.
	if isComplaintBlockStep(sess.CurrentStep) {
		if hit, found := detectComplaintEmergency(sess); found {
			return a.enterEmergency(sess, hit, now)
		}
	}

	// 4) Один адаптивный вопрос DeepSeek после базовых вопросов о жалобе.
	//    Если модель недоступна или ответила дольше 5 секунд — шаг молча пропускается.
	if isBasicComplaintStep(sess.CurrentStep) && shouldAskClarification(sess) &&
		strings.TrimSpace(a.cfg.DeepSeekAPIKey) != "" {
		clarification, err := a.requestClarification(ctx, sess)
		if err != nil {
			log.Printf("clarification: %v — пропускаю уточняющий вопрос", err)
		} else if clarification != nil {
			storeClarification(sess, clarification)
		}
	}

	// Маршрут может измениться после ответа (meds_status → meds_details, уточнение ИИ),
	// поэтому следующий шаг считаем по обновлённым ответам.
	nextSteps := sess.steps()
	next := nextStepID(nextSteps, sess.CurrentStep)
	if next == "" {
		return chatState{}, newAPIError(500, "Не удалось определить следующий шаг опроса.")
	}
	sess.CurrentStep = next

	if next == stepResult {
		card := a.generateCard(ctx, sess)
		summary := consultationSummary{finalCard: card, Answers: sess.answerPairs()}
		if _, err := a.store.insertConsultation(sess.CoreUserID, summary); err != nil {
			log.Printf("chat: сохранить консультацию: %v", err)
		}
		sess.Messages = append(sess.Messages,
			chatMessage{Role: "bot", Step: stepResult, Text: doneText, At: now},
			chatMessage{Role: "card", Step: stepResult, Text: finalDisclaimer, Card: &card, At: now},
		)
	} else if nextSpec, ok := findStep(nextSteps, next); ok {
		sess.Messages = append(sess.Messages, botMessage(next, nextSpec.Question))
	}

	if err := a.store.saveSession(sess); err != nil {
		log.Printf("chat: сохранить сессию: %v", err)
		return chatState{}, newAPIError(500, "Не удалось сохранить состояние опроса.")
	}
	return sess.state(), nil
}

// answerAdditional накапливает сопутствующие симптомы и проверяет контекстные флаги.
func (a *app) answerAdditional(sess *chatSession, text string, now time.Time) (chatState, error) {
	sess.Messages = append(sess.Messages, chatMessage{
		Role: "user", Step: stepAdditionalSymptoms, Text: text, At: now,
	})

	complete := false
	switch text {
	case additionalNone:
		sess.Answers[stepAdditionalSymptoms] = additionalNone
		complete = true
	case additionalDone:
		if len(parseListAnswer(sess.Answers[stepAdditionalSymptoms])) == 0 {
			return chatState{}, newAPIError(400, "Сначала выберите хотя бы один сопутствующий симптом.")
		}
		complete = true
	default:
		selected := parseListAnswer(sess.Answers[stepAdditionalSymptoms])
		if !containsString(selected, text) {
			selected = append(selected, text)
		}
		sess.Answers[stepAdditionalSymptoms] = encodeHabitList(selected)
		if len(remainingAdditionalOptions(sess.Answers)) == 0 {
			complete = true
		}
	}

	// Контекстные красные флаги: ОКС, острый живот, травма головы с последствиями.
	if hit, found := detectComplaintEmergency(sess); found {
		return a.enterEmergency(sess, hit, now)
	}

	if complete {
		sess.History = append(sess.History, stepAdditionalSymptoms)
		sess.CurrentStep = stepChronic
		spec, _ := findStep(sess.steps(), stepChronic)
		sess.Messages = append(sess.Messages, botMessage(stepChronic, spec.Question))
	} else {
		sess.Messages = append(sess.Messages, botMessage(stepAdditionalSymptoms, "Есть ли что-то ещё помимо этого?"))
	}

	if err := a.store.saveSession(sess); err != nil {
		log.Printf("chat: сохранить сессию: %v", err)
		return chatState{}, newAPIError(500, "Не удалось сохранить состояние опроса.")
	}
	return sess.state(), nil
}

// answerChronic накапливает хронические заболевания до «На этом всё».
func (a *app) answerChronic(sess *chatSession, text string, now time.Time) (chatState, error) {
	sess.Messages = append(sess.Messages, chatMessage{
		Role: "user", Step: stepChronic, Text: text, At: now,
	})

	complete := false
	switch text {
	case chronicNone:
		sess.Answers[stepChronic] = chronicNone
		complete = true
	case chronicDone:
		if len(parseListAnswer(sess.Answers[stepChronic])) == 0 {
			return chatState{}, newAPIError(400, "Сначала выберите хотя бы одно хроническое заболевание.")
		}
		complete = true
	default:
		selected := parseListAnswer(sess.Answers[stepChronic])
		if !containsString(selected, text) {
			selected = append(selected, text)
		}
		sess.Answers[stepChronic] = encodeHabitList(selected)
		if len(remainingChronicOptions(sess.Answers)) == 0 {
			complete = true
		}
	}

	if complete {
		sess.History = append(sess.History, stepChronic)
		sess.CurrentStep = stepMedsStatus
		spec, _ := findStep(sess.steps(), stepMedsStatus)
		sess.Messages = append(sess.Messages, botMessage(stepMedsStatus, spec.Question))
	} else {
		sess.Messages = append(sess.Messages, botMessage(stepChronic, "Есть ли ещё хронические заболевания?"))
	}

	if err := a.store.saveSession(sess); err != nil {
		log.Printf("chat: сохранить сессию: %v", err)
		return chatState{}, newAPIError(500, "Не удалось сохранить состояние опроса.")
	}
	return sess.state(), nil
}

// answerHabits обрабатывает итеративный выбор привычек:
// «Нет вредных привычек» и «На этом всё» завершают шаг, остальные ответы копятся.
func (a *app) answerHabits(sess *chatSession, text string, now time.Time) (chatState, error) {
	sess.Messages = append(sess.Messages, chatMessage{
		Role: "user", Step: stepHabits, Text: text, At: now,
	})

	complete := false
	switch text {
	case habitsNone:
		sess.Answers[stepHabits] = habitsNone
		complete = true
	case habitsDone:
		if len(sess.habitList()) == 0 {
			return chatState{}, newAPIError(400, "Сначала выберите хотя бы одну вредную привычку.")
		}
		complete = true
	default:
		habits := sess.habitList()
		if !containsString(habits, text) {
			habits = append(habits, text)
		}
		sess.Answers[stepHabits] = encodeHabitList(habits)

		// Все атомарные факторы выбраны — дальше идти некуда, завершаем цикл.
		if len(remainingHabitOptions(habits)) == 0 {
			complete = true
		}
	}

	if complete {
		sess.History = append(sess.History, stepHabits)
		sess.CurrentStep = stepPaymentType
		spec, _ := findStep(sess.steps(), stepPaymentType)
		sess.Messages = append(sess.Messages, botMessage(stepPaymentType, spec.Question))
	} else {
		// Остаёмся на шаге и предлагаем добавить ещё одну привычку.
		sess.Messages = append(sess.Messages, botMessage(stepHabits, habitsContinueQuestion))
	}

	if err := a.store.saveSession(sess); err != nil {
		log.Printf("chat: сохранить сессию: %v", err)
		return chatState{}, newAPIError(500, "Не удалось сохранить состояние опроса.")
	}
	return sess.state(), nil
}

// enterEmergency переводит сессию в аварийный режим, добавляет инструкцию и сохраняет состояние.
func (a *app) enterEmergency(sess *chatSession, hit *emergencyHit, now time.Time) (chatState, error) {
	sess.IsEmergency = true
	sess.EmergencyReason = hit.Reason
	sess.CurrentStep = stepEmergency
	sess.Messages = append(sess.Messages, chatMessage{
		Role: "emergency", Step: stepEmergency, Text: emergencyMessage(hit), At: now,
	})
	if err := a.store.saveSession(sess); err != nil {
		log.Printf("chat: сохранить сессию: %v", err)
		return chatState{}, newAPIError(500, "Не удалось сохранить состояние опроса.")
	}
	return sess.state(), nil
}

// isComplaintBlockStep — шаги, после которых проверяется контекстный риск ОКС.
func isComplaintBlockStep(step string) bool {
	switch step {
	case stepComplaintCategory, stepComplaintLocation, stepSymptomCharacter,
		stepComplaintDuration, stepComplaintSeverity, stepAdditionalSymptoms, stepAIClarification:
		return true
	default:
		return false
	}
}

// isAffirmativeClarificationAnswer отличает «Да / была / кратковременно»
// от «Нет / не было / не помню», чтобы не ловить красный флаг в отрицании.
func isAffirmativeClarificationAnswer(answer string) bool {
	normalized := normalizeText(answer)
	if normalized == "" {
		return false
	}
	for _, negative := range []string{"нет", "не было", "не была", "ничего", "не помню", "не знаю", "отсутств"} {
		if strings.HasPrefix(normalized, negative) {
			return false
		}
	}
	for _, positive := range []string{"да", "была", "было", "были", "есть", "кратковременно", "один раз", "несколько", "точно", "подтвержда"} {
		if strings.Contains(normalized, positive) {
			return true
		}
	}
	return false
}

// isBasicComplaintStep — вопросы, из которых DeepSeek собирает уточнение.
func isBasicComplaintStep(step string) bool {
	switch step {
	case stepComplaintCategory, stepComplaintLocation, stepSymptomCharacter,
		stepComplaintDuration, stepComplaintSeverity:
		return true
	default:
		return false
	}
}

// shouldAskClarification: уточнение запрашивается после последнего базового вопроса
// (у температуры шаг выраженности пропущен, поэтому триггер — давность).
func shouldAskClarification(sess *chatSession) bool {
	switch sess.CurrentStep {
	case stepComplaintSeverity:
		return true
	case stepComplaintDuration:
		_, hasSeverity := findStep(sess.steps(), stepComplaintSeverity)
		return !hasSeverity
	default:
		return false
	}
}

// additionalSymptomsStep — цикл добора сопутствующих симптомов,
// как в habits: выбрал категорию → предложить ещё или «На этом всё».
func additionalSymptomsStep(answers map[string]string) stepSpec {
	question := "Беспокоит ли что-то ещё помимо этого?"
	if len(parseListAnswer(answers[stepAdditionalSymptoms])) > 0 {
		question = "Есть ли что-то ещё помимо этого?"
	}
	return stepSpec{
		ID:            stepAdditionalSymptoms,
		Label:         "Сопутствующие симптомы",
		Question:      question,
		Options:       additionalSymptomOptions(answers),
		AllowFreeText: true,
	}
}

// additionalSymptomOptions предлагает «Нет, больше ничего» на первом шаге
// и «На этом всё» в цикле добора; выбранная категория и уже отмеченные скрываются.
func additionalSymptomOptions(answers map[string]string) []string {
	selected := parseListAnswer(answers[stepAdditionalSymptoms])
	options := make([]string, 0, len(complaintCategories)+1)
	if len(selected) == 0 {
		options = append(options, additionalNone)
	} else {
		options = append(options, additionalDone)
	}

	chosenCategory := strings.TrimSpace(answers[stepComplaintCategory])
	for _, category := range complaintCategories {
		if strings.EqualFold(chosenCategory, category) || containsString(selected, category) {
			continue
		}
		options = append(options, category)
	}
	return options
}

// remainingAdditionalOptions — категории, которые ещё можно добавить.
func remainingAdditionalOptions(answers map[string]string) []string {
	selected := parseListAnswer(answers[stepAdditionalSymptoms])
	chosenCategory := strings.TrimSpace(answers[stepComplaintCategory])
	remaining := make([]string, 0, len(complaintCategories))
	for _, category := range complaintCategories {
		if strings.EqualFold(chosenCategory, category) || containsString(selected, category) {
			continue
		}
		remaining = append(remaining, category)
	}
	return remaining
}

// chronicStep — цикл добора хронических заболеваний.
func chronicStep(answers map[string]string) stepSpec {
	question := "Есть ли у вас хронические заболевания?"
	options := chronicOptions
	if len(parseListAnswer(answers[stepChronic])) > 0 {
		question = "Есть ли ещё хронические заболевания?"
		options = append([]string{chronicDone}, remainingChronicOptions(answers)...)
	}
	return stepSpec{
		ID:            stepChronic,
		Label:         "Хронические заболевания",
		Question:      question,
		Options:       options,
		AllowFreeText: true,
	}
}

func remainingChronicOptions(answers map[string]string) []string {
	selected := parseListAnswer(answers[stepChronic])
	remaining := make([]string, 0, len(chronicFactors))
	for _, factor := range chronicFactors {
		if !containsString(selected, factor) {
			remaining = append(remaining, factor)
		}
	}
	return remaining
}

// parseListAnswer разбирает JSON-список мультивыбора с фоллбэком на старый текст.
func parseListAnswer(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == additionalNone || raw == chronicNone ||
		raw == additionalDone || raw == chronicDone || raw == habitsNone || raw == habitsDone {
		return nil
	}

	var list []string
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		list = strings.Split(raw, ", ")
	}

	out := make([]string, 0, len(list))
	for _, item := range list {
		item = strings.TrimSpace(item)
		if item == "" || item == additionalNone || item == chronicNone ||
			item == additionalDone || item == chronicDone || containsString(out, item) {
			continue
		}
		out = append(out, item)
	}
	return out
}

// displayListAnswer — человекочитаемая строка мультивыбора для истории и промпта.
func displayListAnswer(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	list := parseListAnswer(raw)
	if len(list) == 0 {
		return raw
	}
	return strings.Join(list, ", ")
}

// complaintDetails — сводка жалоб для карточки и промпта LLM.
func (s *chatSession) complaintDetails() string {
	parts := []string{
		s.Answers[stepComplaintLocation],
		s.Answers[stepSymptomCharacter],
		s.Answers[stepComplaintDuration],
		s.Answers[stepComplaintSeverity],
	}
	if additional := displayListAnswer(s.Answers[stepAdditionalSymptoms]); additional != "" && additional != additionalNone {
		parts = append(parts, additional)
	}
	return joinNonEmpty(parts...)
}

// back откатывает опрос на предыдущий шаг по history_stack_json.
func (a *app) back(sess *chatSession) (chatState, error) {
	if sess.IsEmergency {
		return chatState{}, newAPIError(409,
			"Опрос прерван из-за неотложного состояния — возврат недоступен. Начните новый опрос.")
	}
	if len(sess.History) == 0 {
		return chatState{}, newAPIError(400, "Вы на первом шаге — возвращаться некуда.")
	}

	// Внутри циклов мультивыбора «назад» отменяет последний выбранный вариант.
	if sess.CurrentStep == stepAdditionalSymptoms && len(parseListAnswer(sess.Answers[stepAdditionalSymptoms])) > 0 {
		return a.backInMultiSelect(sess, stepAdditionalSymptoms)
	}
	if sess.CurrentStep == stepChronic && len(parseListAnswer(sess.Answers[stepChronic])) > 0 {
		return a.backInMultiSelect(sess, stepChronic)
	}

	// Внутри цикла привычек «назад» отменяет последнюю выбранную привычку.
	if sess.CurrentStep == stepHabits {
		if len(sess.habitList()) > 0 {
			return a.backInHabits(sess)
		}
		// Ничего не выбрано — выходим из цикла к предыдущему шагу.
		delete(sess.Answers, stepHabits)
		for len(sess.History) > 0 && sess.History[len(sess.History)-1] == stepHabits {
			sess.History = sess.History[:len(sess.History)-1]
		}
		if len(sess.History) == 0 {
			return chatState{}, newAPIError(400, "Вы на первом шаге — возвращаться некуда.")
		}
	}

	return a.backToPrevious(sess)
}

// backToPrevious — обычный откат на предыдущий шаг с учётом цикла привычек.
func (a *app) backToPrevious(sess *chatSession) (chatState, error) {
	target := sess.History[len(sess.History)-1]
	sess.History = sess.History[:len(sess.History)-1]

	// Возврат в цикл мультивыбора: выбранные варианты сохраняем.
	if target == stepAdditionalSymptoms || target == stepChronic {
		sess.CurrentStep = target
		if len(parseListAnswer(sess.Answers[target])) == 0 {
			delete(sess.Answers, target)
		}
		spec, _ := findStep(sess.steps(), target)
		sess.Messages = truncateMessagesAtStep(sess.Messages, target)
		sess.CurrentStep = target
		sess.Messages = append(sess.Messages, botMessage(target, spec.Question))

		if err := a.store.saveSession(sess); err != nil {
			log.Printf("chat: сохранить сессию при возврате: %v", err)
			return chatState{}, newAPIError(500, "Не удалось сохранить состояние опроса.")
		}
		return sess.state(), nil
	}

	// Возврат из payment_type в цикл привычек: выбранные привычки сохраняем.
	if target == stepHabits {
		sess.CurrentStep = stepHabits
		if strings.TrimSpace(sess.Answers[stepHabits]) == habitsNone {
			delete(sess.Answers, stepHabits)
		}
		sess.Messages = trimHabitsReturn(sess.Messages)

		if err := a.store.saveSession(sess); err != nil {
			log.Printf("chat: сохранить сессию при возврате: %v", err)
			return chatState{}, newAPIError(500, "Не удалось сохранить состояние опроса.")
		}
		return sess.state(), nil
	}

	delete(sess.Answers, target)

	spec, ok := findStep(sess.steps(), target)
	if !ok {
		return chatState{}, newAPIError(500, "Неизвестный шаг опроса.")
	}

	sess.Messages = truncateMessagesAtStep(sess.Messages, target)
	sess.CurrentStep = target
	sess.Messages = append(sess.Messages, botMessage(target, spec.Question))

	if err := a.store.saveSession(sess); err != nil {
		log.Printf("chat: сохранить сессию при возврате: %v", err)
		return chatState{}, newAPIError(500, "Не удалось сохранить состояние опроса.")
	}
	return sess.state(), nil
}

// backInMultiSelect отменяет последний выбранный вариант и остаётся в цикле.
func (a *app) backInMultiSelect(sess *chatSession, step string) (chatState, error) {
	selected := parseListAnswer(sess.Answers[step])
	selected = selected[:len(selected)-1]
	if len(selected) == 0 {
		delete(sess.Answers, step)
	} else {
		sess.Answers[step] = encodeHabitList(selected)
	}

	sess.CurrentStep = step
	if err := a.store.saveSession(sess); err != nil {
		log.Printf("chat: сохранить сессию при возврате: %v", err)
		return chatState{}, newAPIError(500, "Не удалось сохранить состояние опроса.")
	}
	return sess.state(), nil
}

// backInHabits отменяет последнюю выбранную привычку и остаётся в цикле.
func (a *app) backInHabits(sess *chatSession) (chatState, error) {
	habits := sess.habitList()
	habits = habits[:len(habits)-1]

	if len(habits) == 0 {
		delete(sess.Answers, stepHabits)
	} else {
		sess.Answers[stepHabits] = encodeHabitList(habits)
	}

	sess.CurrentStep = stepHabits
	sess.Messages = trimLastHabitExchange(sess.Messages)

	if err := a.store.saveSession(sess); err != nil {
		log.Printf("chat: сохранить сессию при возврате: %v", err)
		return chatState{}, newAPIError(500, "Не удалось сохранить состояние опроса.")
	}
	return sess.state(), nil
}

// reset возвращает сессию в начало опроса, сохраняя авторизацию и данные пользователя.
func (a *app) reset(sess *chatSession) (chatState, error) {
	initSession(sess)
	if err := a.store.saveSession(sess); err != nil {
		log.Printf("chat: сбросить сессию: %v", err)
		return chatState{}, newAPIError(500, "Не удалось сбросить опрос.")
	}
	return sess.state(), nil
}

// answerPairs превращает ответы в человекочитаемые пары для истории обращений.
func (s *chatSession) answerPairs() []qaPair {
	pairs := make([]qaPair, 0, len(s.Answers))
	for _, step := range s.steps() {
		answer := strings.TrimSpace(s.Answers[step.ID])
		if answer == "" {
			continue
		}
		question := step.Label
		if step.ID == stepAIClarification {
			if clarification := parseStoredClarification(s.Answers); clarification != nil {
				question = clarification.Question
			}
		}
		if step.ID == stepHabits {
			answer = s.habitsDisplay()
			if answer == "" {
				continue
			}
		}
		if step.ID == stepAdditionalSymptoms || step.ID == stepChronic {
			answer = displayListAnswer(answer)
		}
		pairs = append(pairs, qaPair{Question: question, Answer: answer})
	}
	return pairs
}

// trimLastHabitExchange убирает последний вопрос-приглашение и ответ пользователя.
func trimLastHabitExchange(messages []chatMessage) []chatMessage {
	if n := len(messages); n > 0 && messages[n-1].Role == "bot" && messages[n-1].Step == stepHabits {
		messages = messages[:n-1]
	}
	if n := len(messages); n > 0 && messages[n-1].Role == "user" && messages[n-1].Step == stepHabits {
		messages = messages[:n-1]
	}
	return messages
}

// trimHabitsReturn убирает хвост «вопрос следующего шага + ответ пользователя»,
// чтобы вернуть ленту к вопросу привычек.
func trimHabitsReturn(messages []chatMessage) []chatMessage {
	if n := len(messages); n > 0 && messages[n-1].Role == "bot" && messages[n-1].Step == stepPaymentType {
		messages = messages[:n-1]
	}
	if n := len(messages); n > 0 && messages[n-1].Role == "user" && messages[n-1].Step == stepHabits {
		messages = messages[:n-1]
	}
	return messages
}

// truncateMessagesAtStep убирает ленту начиная с последнего вопроса шага target.
func truncateMessagesAtStep(messages []chatMessage, target string) []chatMessage {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "bot" && messages[i].Step == target {
			return messages[:i]
		}
	}
	return messages
}

// matchOption сравнивает ответ с вариантами без учёта регистра и лишних пробелов.
func matchOption(options []string, answer string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(answer))
	for _, option := range options {
		if strings.ToLower(option) == normalized {
			return option, true
		}
	}
	return "", false
}
