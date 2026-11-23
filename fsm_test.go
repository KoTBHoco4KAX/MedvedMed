package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestBaseRouteOrder(t *testing.T) {
	steps := stepsFor(1, 30, nil)

	want := []string{
		stepDisclaimer,
		stepComplaintCategory,
		stepComplaintLocation,
		stepSymptomCharacter,
		stepComplaintDuration,
		stepComplaintSeverity,
		stepAdditionalSymptoms,
		stepChronic,
		stepMedsStatus,
		stepAllergies,
		stepHospitalizations,
		stepHeredity,
		stepHabits,
		stepPaymentType,
		stepResult,
	}
	if len(steps) != len(want) {
		t.Fatalf("всего шагов %d, ожидалось %d: %+v", len(steps), len(want), steps)
	}
	for i, id := range want {
		if steps[i].ID != id {
			t.Fatalf("шаг %d: %q, ожидался %q", i, steps[i].ID, id)
		}
		if steps[i].Question == "" || steps[i].Label == "" {
			t.Fatalf("у шага %q пустой вопрос или label", steps[i].ID)
		}
	}
	if _, ok := findStep(steps, stepMedsDetails); ok {
		t.Fatal("meds_details не должен попадать в маршрут без ответа meds_status")
	}
}

func TestStepOptionsMatchSpec(t *testing.T) {
	steps := stepsFor(1, 30, nil)

	cases := []struct {
		step    string
		options []string
	}{
		{stepComplaintCategory, []string{"Боль", "Температура", "Дыхание", "ЖКТ", "Кожа", "Травма", "Головокружение, онемение, сон"}},
		{stepSymptomCharacter, symptomCharacterOther},
		{stepComplaintDuration, []string{"Только что (меньше часа назад)", "Сегодня (несколько часов)", "1–3 дня", "Больше 3 дней", "Периодически повторяется"}},
		{stepComplaintSeverity, []string{"Слабо, почти не мешает", "Умеренно", "Сильно", "Нестерпимо / острая"}},
		{stepAdditionalSymptoms, []string{additionalNone, "Боль", "Температура", "Дыхание", "ЖКТ", "Кожа", "Травма", "Головокружение, онемение, сон"}},
		{stepChronic, []string{"Нет хронических", "Гипертония / ССЗ", "Диабет", "Астма / ХОБЛ", "Онкология"}},
		{stepMedsStatus, []string{"Не принимаю вообще", "Принимаю системно (постоянно)", "Принимаю периодически (по ситуации)"}},
		{stepAllergies, []string{"Аллергий нет", "Есть аллергия на лекарства", "Пищевая / бытовая аллергия"}},
		{stepHospitalizations, []string{"Нет, не было", "Да, была операция / госпитализация"}},
		{stepHeredity, []string{"Нет, наследственность не отягощена", "Да, есть (ССЗ / диабет / онкология)", "Не знаю"}},
		{stepHabits, []string{habitsNone, "Курение / вейп", "Употребление алкоголя", "Употребление наркотиков / ПАВ", "Недосып / хронический стресс", "Малоподвижный образ жизни"}},
		{stepPaymentType, []string{"ОМС (бесплатно по полису)", "ДМС", "Платно"}},
	}

	for _, tc := range cases {
		spec, ok := findStep(steps, tc.step)
		if !ok {
			t.Fatalf("шаг %q не найден", tc.step)
		}
		if strings.Join(spec.Options, "|") != strings.Join(tc.options, "|") {
			t.Fatalf("варианты %q: %v, ожидались %v", tc.step, spec.Options, tc.options)
		}
	}

	// meds_status — единственный из новых шагов без свободного ввода.
	meds, _ := findStep(steps, stepMedsStatus)
	if meds.AllowFreeText {
		t.Fatal("на шаге meds_status не должно быть свободного ввода")
	}
}

func TestAdditionalSymptomsExcludeChosen(t *testing.T) {
	steps := stepsFor(1, 30, map[string]string{stepComplaintCategory: "Боль"})
	spec, ok := findStep(steps, stepAdditionalSymptoms)
	if !ok {
		t.Fatal("шаг additional_symptoms не найден")
	}
	for _, option := range spec.Options {
		if option == "Боль" {
			t.Fatalf("выбранная категория не должна повторяться: %v", spec.Options)
		}
	}
	if spec.Options[0] != additionalNone {
		t.Fatalf("первым вариантом должно быть %q: %v", additionalNone, spec.Options)
	}
	if !containsAny(strings.Join(spec.Options, " "), "Дыхание", "Температура", "Травма") {
		t.Fatalf("в вариантах нет остальных категорий: %v", spec.Options)
	}
	if !spec.AllowFreeText {
		t.Fatal("сопутствующие симптомы должны допускать свободный ввод")
	}
}

func TestComplaintLocationDependsOnCategory(t *testing.T) {
	pain := stepsFor(1, 30, map[string]string{stepComplaintCategory: "Боль"})
	location, ok := findStep(pain, stepComplaintLocation)
	if !ok {
		t.Fatal("шаг complaint_location не найден")
	}
	if location.Question != "Где именно болит?" {
		t.Fatalf("для боли ожидался вопрос про локализацию, получено %q", location.Question)
	}
	if strings.Join(location.Options, "|") != strings.Join(painLocations, "|") {
		t.Fatalf("варианты локализации боли: %v", location.Options)
	}
	if !location.AllowFreeText {
		t.Fatal("локализация должна допускать свободный ввод")
	}

	freeText := stepsFor(2, 40, map[string]string{stepComplaintCategory: "сильно болит голова"})
	location, _ = findStep(freeText, stepComplaintLocation)
	if location.Question != "Где именно болит?" {
		t.Fatalf("свободный текст про боль: %q", location.Question)
	}

	fever := stepsFor(1, 30, map[string]string{stepComplaintCategory: "Температура"})
	location, _ = findStep(fever, stepComplaintLocation)
	if location.Question != "Какая сейчас температура тела? Укажите в °C или выберите вариант:" {
		t.Fatalf("для температуры ожидался вопрос про градусы, получено %q", location.Question)
	}
	if strings.Join(location.Options, "|") != strings.Join(temperatureOptions, "|") {
		t.Fatalf("варианты температуры: %v", location.Options)
	}
	if !location.AllowFreeText {
		t.Fatal("температура должна поддерживать свободный ввод градусов")
	}
	if _, ok := findStep(fever, stepComplaintSeverity); ok {
		t.Fatal("для температуры шаг complaint_severity не нужен")
	}

	feverFreeText := stepsFor(1, 30, map[string]string{stepComplaintCategory: "высокая температура и жар"})
	location, _ = findStep(feverFreeText, stepComplaintLocation)
	if location.Question != "Какая сейчас температура тела? Укажите в °C или выберите вариант:" {
		t.Fatalf("свободный текст про температуру: %q", location.Question)
	}
}

func TestConditionalSteps(t *testing.T) {
	cases := []struct {
		name              string
		sex, age          int
		answers           map[string]string
		wantPregnancy     bool
		wantMedsDetail    bool
		wantAllergyDetail bool
		wantTotal         int
	}{
		{"мужчина, лекарств нет", 1, 30, nil, false, false, false, 15},
		{"женщина 30, лекарств нет", 2, 30, nil, true, false, false, 16},
		{"женщина 30, принимает системно", 2, 30, map[string]string{stepMedsStatus: medsRegular}, true, true, false, 17},
		{"женщина 13, не принимает", 2, 13, map[string]string{stepMedsStatus: medsNone}, false, false, false, 15},
		{"женщина 51, принимает периодически", 2, 51, map[string]string{stepMedsStatus: medsOccasional}, false, true, false, 16},
		{"мужчина, принимает системно", 1, 30, map[string]string{stepMedsStatus: medsRegular}, false, true, false, 16},
		{"пол не указан", 0, 30, nil, false, false, false, 15},
		{"мужчина, аллергия на лекарства", 1, 30, map[string]string{stepAllergies: "Есть аллергия на лекарства"}, false, false, true, 16},
		{"мужчина, аллергий нет", 1, 30, map[string]string{stepAllergies: allergyNone}, false, false, false, 15},
		{"мужчина, свободный текст про аллергию", 1, 30, map[string]string{stepAllergies: "аллергия на пенициллин"}, false, false, true, 16},
		{"женщина 30, аллергия и системные лекарства", 2, 30, map[string]string{stepMedsStatus: medsRegular, stepAllergies: "Пищевая / бытовая аллергия"}, true, true, true, 18},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			steps := stepsFor(tc.sex, tc.age, tc.answers)
			if len(steps) != tc.wantTotal {
				t.Fatalf("всего шагов %d, ожидалось %d", len(steps), tc.wantTotal)
			}
			if _, ok := findStep(steps, stepPregnancy); ok != tc.wantPregnancy {
				t.Fatalf("pregnancy в маршруте = %v, ожидалось %v", ok, tc.wantPregnancy)
			}
			if _, ok := findStep(steps, stepMedsDetails); ok != tc.wantMedsDetail {
				t.Fatalf("meds_details в маршруте = %v, ожидалось %v", ok, tc.wantMedsDetail)
			}
			if _, ok := findStep(steps, stepAllergiesDetails); ok != tc.wantAllergyDetail {
				t.Fatalf("allergies_details в маршруте = %v, ожидалось %v", ok, tc.wantAllergyDetail)
			}
			if steps[0].ID != stepDisclaimer || steps[len(steps)-1].ID != stepResult {
				t.Fatal("маршрут должен начинаться с disclaimer и заканчиваться result")
			}
		})
	}
}

func TestProgressCountsConditionalSteps(t *testing.T) {
	male := stepsFor(1, 30, nil)
	if got := progressFor(male, stepDisclaimer, 0); got != (progress{Current: 1, Total: 15}) {
		t.Fatalf("мужчина, disclaimer: %+v", got)
	}
	if got := progressFor(male, stepResult, 14); got != (progress{Current: 15, Total: 15}) {
		t.Fatalf("мужчина, result: %+v", got)
	}
	if got := progressFor(male, stepComplaintSeverity, 5); got != (progress{Current: 6, Total: 15}) {
		t.Fatalf("мужчина, severity: %+v", got)
	}

	female := stepsFor(2, 30, map[string]string{stepMedsStatus: medsRegular})
	if got := progressFor(female, stepMedsDetails, 9); got != (progress{Current: 10, Total: 17}) {
		t.Fatalf("женщина, meds_details: %+v", got)
	}
	if got := progressFor(female, stepPregnancy, 13); got != (progress{Current: 14, Total: 17}) {
		t.Fatalf("женщина, pregnancy: %+v", got)
	}
}

func TestOptionMatching(t *testing.T) {
	options := []string{"ОМС (бесплатно по полису)", "ДМС", "Платно"}

	canonical, ok := matchOption(options, "  дмс ")
	if !ok || canonical != "ДМС" {
		t.Fatalf("matchOption не привёл ответ к точному варианту: %q, %v", canonical, ok)
	}
	if _, ok := matchOption(options, "по дружбе"); ok {
		t.Fatalf("произвольный текст не должен совпадать с вариантом")
	}
}

func TestAnswerPairs(t *testing.T) {
	sess := &chatSession{
		Age: 30,
		Sex: 1,
		Answers: map[string]string{
			stepDisclaimer:         "Понятно, начать опрос",
			stepComplaintCategory:  "Боль",
			stepComplaintLocation:  "Голова",
			stepSymptomCharacter:   "Тяжесть / давящая боль после сна, усталости или стресса",
			stepComplaintDuration:  "1–3 дня",
			stepComplaintSeverity:  "Умеренно",
			stepAdditionalSymptoms: additionalNone,
			stepChronic:            "Нет хронических",
			stepMedsStatus:         medsNone,
			stepAllergies:          "Аллергий нет",
			stepHospitalizations:   "Нет, не было",
			stepHeredity:           "Не знаю",
			stepHabits:             "Нет вредных привычек",
			stepPaymentType:        "ДМС",
		},
	}

	pairs := sess.answerPairs()
	if len(pairs) != 14 {
		t.Fatalf("пар ответов %d, ожидалось 14: %+v", len(pairs), pairs)
	}
	if pairs[0].Question != "Начало опроса" || pairs[1].Question != "Основная жалоба" {
		t.Fatalf("порядок вопросов нарушен: %+v", pairs)
	}
	if !containsPair(pairs, "Характер симптома", "Тяжесть / давящая боль после сна, усталости или стресса") {
		t.Fatalf("нет пары про характер симптома: %+v", pairs)
	}
	if !containsPair(pairs, "Насколько сильно выражено", "Умеренно") {
		t.Fatalf("нет пары про интенсивность: %+v", pairs)
	}
	if !containsPair(pairs, "Сопутствующие симптомы", additionalNone) {
		t.Fatalf("нет пары про сопутствующие симптомы: %+v", pairs)
	}
	for _, pair := range pairs {
		if pair.Question == "Какие лекарства" {
			t.Fatalf("meds_details не должен попадать в историю при meds_status=%q", medsNone)
		}
	}

	sess.Answers[stepMedsStatus] = medsRegular
	sess.Answers[stepMedsDetails] = "Аспирин 100 мг"
	pairs = sess.answerPairs()
	if len(pairs) != 15 || !containsPair(pairs, "Какие лекарства", "Аспирин 100 мг") {
		t.Fatalf("meds_details не попал в историю: %+v", pairs)
	}
}

func TestDynamicRouteSkipsMedsDetails(t *testing.T) {
	st, err := openStore(filepath.Join(t.TempDir(), "fsm.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.close()

	a := &app{store: st}

	walk := func(sess *chatSession, answers ...string) chatState {
		t.Helper()
		var state chatState
		for _, answer := range answers {
			var err error
			state, err = a.answer(context.Background(), sess, answer)
			if err != nil {
				t.Fatalf("answer(%q): %v", answer, err)
			}
		}
		return state
	}

	sess, err := st.createSession("u-skip", "Тест", 30, 1)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}
	state := walk(sess,
		"Понятно, начать опрос",
		"Боль",
		"Голова",
		"Ноющая / тупая, нарастала постепенно",
		"1–3 дня",
		"Умеренно",
		additionalNone,
		"Нет хронических",
		medsNone,
	)
	if state.Step != stepAllergies {
		t.Fatalf("при «%s» ожидался шаг allergies, получено %q", medsNone, state.Step)
	}
	if state.Progress.Total != 15 {
		t.Fatalf("total без meds_details = %d, ожидалось 15", state.Progress.Total)
	}

	sess2, err := st.createSession("u-details", "Тест", 30, 1)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}
	state = walk(sess2,
		"Понятно, начать опрос",
		"Температура",
		"38.0–38.9 °C",
		"Жар и ломота во всём теле",
		"1–3 дня",
		additionalNone,
		"Нет хронических",
		medsRegular,
	)
	if state.Step != stepMedsDetails {
		t.Fatalf("при «%s» ожидался шаг meds_details, получено %q", medsRegular, state.Step)
	}
	if state.Progress.Total != 15 {
		t.Fatalf("total с meds_details на температуре = %d, ожидалось 15", state.Progress.Total)
	}
	if !state.AllowFreeText || len(state.Options) != 1 || state.Options[0] != "Не помню название" {
		t.Fatalf("шаг meds_details собрал неверные кнопки: %+v", state)
	}

	state = walk(sess2, "Не помню название")
	if state.Step != stepAllergies || state.Progress.Total != 15 {
		t.Fatalf("после meds_details ожидался allergies, получено %+v", state)
	}
}

func TestBackRemovesConditionalStep(t *testing.T) {
	st, err := openStore(filepath.Join(t.TempDir(), "fsm-back.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.close()

	a := &app{store: st}
	sess, err := st.createSession("u-back", "Тест", 30, 1)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	for _, answer := range []string{
		"Понятно, начать опрос",
		"Температура",
		"38.0–38.9 °C",
		"Жар и ломота во всём теле",
		"1–3 дня",
		additionalNone,
		"Нет хронических",
		medsRegular,
		"Парацетамол",
	} {
		if _, err := a.answer(context.Background(), sess, answer); err != nil {
			t.Fatalf("answer(%q): %v", answer, err)
		}
	}
	if sess.CurrentStep != stepAllergies {
		t.Fatalf("ожидался allergies, получено %q", sess.CurrentStep)
	}

	if _, err := a.back(sess); err != nil {
		t.Fatalf("back на meds_details: %v", err)
	}
	if sess.CurrentStep != stepMedsDetails {
		t.Fatalf("первый back должен вернуть на meds_details, получено %q", sess.CurrentStep)
	}

	state, err := a.back(sess)
	if err != nil {
		t.Fatalf("back на meds_status: %v", err)
	}
	if state.Step != stepMedsStatus {
		t.Fatalf("второй back должен вернуть на meds_status, получено %q", state.Step)
	}
	if state.Progress.Total != 14 {
		t.Fatalf("после удаления ответа meds_details total = %d, ожидалось 14", state.Progress.Total)
	}

	state, err = a.answer(context.Background(), sess, medsNone)
	if err != nil {
		t.Fatalf("answer(medsNone): %v", err)
	}
	if state.Step != stepAllergies {
		t.Fatalf("после смены статуса лекарств ожидался allergies, получено %q", state.Step)
	}
}

func TestRadiatingPainStopsSurveyAtSuddenOnset(t *testing.T) {
	st, err := openStore(filepath.Join(t.TempDir(), "acs-sudden.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.close()

	a := &app{store: st}
	sess, err := st.createSession("u-acs-1", "Тест", 45, 1)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	for _, answer := range []string{"Понятно, начать опрос", "Боль", "ключица левая", "Ноющая / тупая, нарастала постепенно"} {
		if _, err := a.answer(context.Background(), sess, answer); err != nil {
			t.Fatalf("answer(%q): %v", answer, err)
		}
	}
	if sess.IsEmergency {
		t.Fatal("без усиливающего фактора красный флаг не должен срабатывать")
	}

	// Внезапное начало — уже достаточный усиливающий фактор.
	state, err := a.answer(context.Background(), sess, durationSudden)
	if err != nil {
		t.Fatalf("answer(durationSudden): %v", err)
	}
	if !state.IsEmergency || state.Step != stepEmergency {
		t.Fatalf("ожидался красный флаг на шаге давности, получено %+v", state)
	}
	if !strings.Contains(state.EmergencyReason, "экстренной") {
		t.Fatalf("причина не про экстренную помощь: %q", state.EmergencyReason)
	}
	if state.CanGoBack {
		t.Fatal("после красного флага возврат запрещён")
	}

	if _, err := a.answer(context.Background(), sess, severityStrong); err == nil {
		t.Fatal("после красного флага новые ответы запрещены")
	}
}

func TestAdditionalSymptomTriggersEmergency(t *testing.T) {
	st, err := openStore(filepath.Join(t.TempDir(), "acs-additional.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.close()

	a := &app{store: st}
	sess, err := st.createSession("u-acs-2", "Тест", 52, 1)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	for _, answer := range []string{
		"Понятно, начать опрос",
		"Боль",
		"левое плечо",
		"Ноющая / тупая, нарастала постепенно",
		"1–3 дня",
		"Умеренно",
	} {
		if _, err := a.answer(context.Background(), sess, answer); err != nil {
			t.Fatalf("answer(%q): %v", answer, err)
		}
	}
	if sess.CurrentStep != stepAdditionalSymptoms {
		t.Fatalf("ожидался шаг сопутствующих симптомов, получено %q", sess.CurrentStep)
	}

	// Дыхание как сопутствующая жалоба + иррадиация = красный флаг.
	state, err := a.answer(context.Background(), sess, "Дыхание")
	if err != nil {
		t.Fatalf("answer(Дыхание): %v", err)
	}
	if !state.IsEmergency {
		t.Fatalf("дыхание + иррадиация должны давать красный флаг: %+v", state)
	}
	if !strings.Contains(state.EmergencyReason, "экстренной") {
		t.Fatalf("причина не про экстренную помощь: %q", state.EmergencyReason)
	}
}

func TestNoEmergencyForNonRadiatingPain(t *testing.T) {
	st, err := openStore(filepath.Join(t.TempDir(), "acs-negative.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.close()

	a := &app{store: st}
	sess, err := st.createSession("u-acs-3", "Тест", 30, 1)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	for _, answer := range []string{
		"Понятно, начать опрос",
		"Боль",
		"Голова",
		"Тяжесть / давящая боль после сна, усталости или стресса",
		"1–3 дня",
		severityStrong,
		additionalNone,
	} {
		if _, err := a.answer(context.Background(), sess, answer); err != nil {
			t.Fatalf("answer(%q): %v", answer, err)
		}
	}
	if sess.IsEmergency {
		t.Fatalf("головная боль без иррадиации не должна быть красным флагом: %q", sess.EmergencyReason)
	}
	if sess.CurrentStep != stepChronic {
		t.Fatalf("после сопутствующих симптомов ожидался chronic, получено %q", sess.CurrentStep)
	}
}

func TestStateResultCarriesCard(t *testing.T) {
	card := finalCard{Specialty: "Врач-терапевт", Urgency: "Планово (в течение 3–5 дней)"}
	sess := &chatSession{
		ID:          "token",
		Age:         30,
		Sex:         1,
		CurrentStep: stepResult,
		History:     []string{stepDisclaimer},
		Answers:     map[string]string{stepComplaintCategory: "Боль"},
		Messages: []chatMessage{
			{Role: "card", Step: stepResult, Text: finalDisclaimer, Card: &card},
		},
	}

	state := sess.state()
	if !state.Completed || state.Card == nil || state.Card.Specialty != "Врач-терапевт" {
		t.Fatalf("состояние result должно содержать карточку: %+v", state)
	}
	if !strings.Contains(state.Question, "103/112") {
		t.Fatalf("в финальном состоянии нет дисклеймера про скорую: %q", state.Question)
	}
}

func TestStateEmergencyDisablesBack(t *testing.T) {
	sess := &chatSession{
		ID:              "token",
		CurrentStep:     stepEmergency,
		IsEmergency:     true,
		EmergencyReason: "причина",
		Messages:        []chatMessage{},
	}

	state := sess.state()
	if !state.IsEmergency || state.CanGoBack || state.EmergencyInstruction == "" {
		t.Fatalf("аварийное состояние собрано неверно: %+v", state)
	}
}

func TestChronicHasNoOtherOption(t *testing.T) {
	steps := stepsFor(1, 30, nil)
	spec, ok := findStep(steps, stepChronic)
	if !ok {
		t.Fatal("шаг chronic не найден")
	}
	if containsString(spec.Options, "Другое") {
		t.Fatalf("кнопки «Другое» быть не должно: %v", spec.Options)
	}
	if len(spec.Options) != 5 {
		t.Fatalf("ожидалось 5 категорий, получено %v", spec.Options)
	}
	if !spec.AllowFreeText {
		t.Fatal("хронические заболевания должны допускать свободный ввод")
	}
}

func TestAllergiesDetailsStep(t *testing.T) {
	steps := stepsFor(1, 30, map[string]string{stepAllergies: "Есть аллергия на лекарства"})
	spec, ok := findStep(steps, stepAllergiesDetails)
	if !ok {
		t.Fatal("allergies_details не появился при наличии аллергии")
	}
	if spec.Question != "На что именно у вас аллергия и как она проявляется?" {
		t.Fatalf("вопрос шага: %q", spec.Question)
	}
	if len(spec.Options) != 1 || spec.Options[0] != "Не помню / затрудняюсь ответить" {
		t.Fatalf("варианты шага: %v", spec.Options)
	}
	if !spec.AllowFreeText {
		t.Fatal("allergies_details должен допускать свободный ввод")
	}

	none := stepsFor(1, 30, map[string]string{stepAllergies: allergyNone})
	if _, ok := findStep(none, stepAllergiesDetails); ok {
		t.Fatal("при «Аллергий нет» шаг аллергий не нужен")
	}
}

func TestAllergiesFlowOpensDetails(t *testing.T) {
	st, err := openStore(filepath.Join(t.TempDir(), "allergies.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.close()

	a := &app{store: st}
	sess, err := st.createSession("u-allergy", "Тест", 30, 1)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	for _, answer := range []string{
		"Понятно, начать опрос",
		"Боль",
		"Голова",
		"Ноющая / тупая, нарастала постепенно",
		"1–3 дня",
		"Умеренно",
		additionalNone,
		"Нет хронических",
		medsNone,
	} {
		if _, err := a.answer(context.Background(), sess, answer); err != nil {
			t.Fatalf("answer(%q): %v", answer, err)
		}
	}
	if sess.CurrentStep != stepAllergies {
		t.Fatalf("ожидался шаг аллергий, получено %q", sess.CurrentStep)
	}

	state, err := a.answer(context.Background(), sess, "Есть аллергия на лекарства")
	if err != nil {
		t.Fatalf("answer(allergies): %v", err)
	}
	if state.Step != stepAllergiesDetails {
		t.Fatalf("ожидался allergies_details, получено %q", state.Step)
	}
	if state.Progress.Total != 16 {
		t.Fatalf("total с allergies_details = %d, ожидалось 16", state.Progress.Total)
	}

	state, err = a.answer(context.Background(), sess, "Не помню / затрудняюсь ответить")
	if err != nil {
		t.Fatalf("answer(details): %v", err)
	}
	if state.Step != stepHospitalizations {
		t.Fatalf("после деталей аллергии ожидался hospitalizations, получено %q", state.Step)
	}
}

func TestHabitsCycle(t *testing.T) {
	a, st, sess := setupHabitsSession(t)
	defer st.close()

	state := sess.state()
	if state.Step != stepHabits || state.Question != habitsIntroQuestion {
		t.Fatalf("старт цикла привычек: %+v", state)
	}
	if len(state.Options) == 0 || state.Options[0] != habitsNone {
		t.Fatalf("первый вариант должен быть %q: %v", habitsNone, state.Options)
	}
	if len(state.Options) != len(habitRiskFactors)+1 {
		t.Fatalf("вариантов первого шага привычек: %v", state.Options)
	}

	state, err := a.answer(context.Background(), sess, "Курение / вейп")
	if err != nil {
		t.Fatalf("answer(Курение): %v", err)
	}
	if state.Step != stepHabits || state.Question != habitsContinueQuestion {
		t.Fatalf("после привычки нужно остаться в цикле: %+v", state)
	}
	wantOptions := []string{habitsDone, "Употребление алкоголя", "Употребление наркотиков / ПАВ", "Недосып / хронический стресс", "Малоподвижный образ жизни"}
	if strings.Join(state.Options, "|") != strings.Join(wantOptions, "|") {
		t.Fatalf("варианты второй итерации: %v, ожидались %v", state.Options, wantOptions)
	}
	if containsString(state.Options, habitsNone) || containsString(state.Options, "Курение / вейп") {
		t.Fatalf("выбранные и «нет привычек» не должны повторяться: %v", state.Options)
	}

	state, err = a.answer(context.Background(), sess, "Употребление алкоголя")
	if err != nil {
		t.Fatalf("answer(Алкоголь): %v", err)
	}
	if state.Step != stepHabits {
		t.Fatalf("после второй привычки всё ещё цикл: %q", state.Step)
	}
	if containsString(state.Options, "Употребление алкоголя") {
		t.Fatalf("выбранная привычка осталась в кнопках: %v", state.Options)
	}

	state, err = a.answer(context.Background(), sess, habitsDone)
	if err != nil {
		t.Fatalf("answer(На этом всё): %v", err)
	}
	if state.Step != stepPaymentType {
		t.Fatalf("после «На этом всё» ожидался payment_type, получено %q", state.Step)
	}

	habits := sess.habitList()
	if len(habits) != 2 || habits[0] != "Курение / вейп" || habits[1] != "Употребление алкоголя" {
		t.Fatalf("накопленные привычки: %v", habits)
	}
	pairs := sess.answerPairs()
	if !containsPair(pairs, "Вредные привычки", "Курение / вейп, Употребление алкоголя") {
		t.Fatalf("в историю попали не все привычки: %+v", pairs)
	}
}

func TestHabitsNoneSkipsCycle(t *testing.T) {
	a, st, sess := setupHabitsSession(t)
	defer st.close()

	state, err := a.answer(context.Background(), sess, habitsNone)
	if err != nil {
		t.Fatalf("answer(habitsNone): %v", err)
	}
	if state.Step != stepPaymentType {
		t.Fatalf("«Нет вредных привычек» должен вести сразу на оплату, получено %q", state.Step)
	}
	if sess.Answers[stepHabits] != habitsNone {
		t.Fatalf("ответ не сохранён: %q", sess.Answers[stepHabits])
	}
	if !containsPair(sess.answerPairs(), "Вредные привычки", habitsNone) {
		t.Fatalf("нет пары про привычки: %+v", sess.answerPairs())
	}
}

func TestHabitsAutoCompletesWhenExhausted(t *testing.T) {
	a, st, sess := setupHabitsSession(t)
	defer st.close()

	var state chatState
	for index, factor := range habitRiskFactors {
		var err error
		state, err = a.answer(context.Background(), sess, factor)
		if err != nil {
			t.Fatalf("answer(%q): %v", factor, err)
		}
		if index < len(habitRiskFactors)-1 && state.Step != stepHabits {
			t.Fatalf("после %q ожидался цикл, получено %q", factor, state.Step)
		}
	}
	if state.Step != stepPaymentType {
		t.Fatalf("после всех факторов ожидался payment_type, получено %q", state.Step)
	}
	if len(sess.habitList()) != len(habitRiskFactors) {
		t.Fatalf("не все факторы сохранены: %v", sess.habitList())
	}
}

func TestHabitsBack(t *testing.T) {
	a, st, sess := setupHabitsSession(t)
	defer st.close()

	for _, answer := range []string{"Курение / вейп", "Употребление алкоголя", habitsDone} {
		if _, err := a.answer(context.Background(), sess, answer); err != nil {
			t.Fatalf("answer(%q): %v", answer, err)
		}
	}
	if sess.CurrentStep != stepPaymentType {
		t.Fatalf("ожидался payment_type, получено %q", sess.CurrentStep)
	}

	// Из оплаты — назад в цикл, привычки сохраняются.
	state, err := a.back(sess)
	if err != nil {
		t.Fatalf("back в цикл: %v", err)
	}
	if state.Step != stepHabits || len(sess.habitList()) != 2 {
		t.Fatalf("возврат в цикл сохранил привычки: %+v, %v", state, sess.habitList())
	}
	if state.Question != habitsContinueQuestion {
		t.Fatalf("вопрос возврата: %q", state.Question)
	}

	// Внутри цикла назад отменяет последнюю привычку.
	state, err = a.back(sess)
	if err != nil {
		t.Fatalf("back привычки: %v", err)
	}
	if state.Step != stepHabits || len(sess.habitList()) != 1 || sess.habitList()[0] != "Курение / вейп" {
		t.Fatalf("после отмены: %+v, %v", state, sess.habitList())
	}
	if !containsString(state.Options, "Употребление алкоголя") {
		t.Fatalf("отменённая привычка должна вернуться в кнопки: %v", state.Options)
	}

	state, err = a.back(sess)
	if err != nil {
		t.Fatalf("back последней привычки: %v", err)
	}
	if state.Step != stepHabits || len(sess.habitList()) != 0 {
		t.Fatalf("после очистки цикла: %+v", state)
	}
	if len(state.Options) == 0 || state.Options[0] != habitsNone {
		t.Fatalf("вернулись не к первому вопросу привычек: %v", state.Options)
	}

	// Ещё назад — выходим из цикла к предыдущему шагу.
	state, err = a.back(sess)
	if err != nil {
		t.Fatalf("back из цикла: %v", err)
	}
	if state.Step != stepHeredity {
		t.Fatalf("выход из цикла должен вести на heredity, получено %q", state.Step)
	}
}

func TestHabitsBackFromNoneReturnsToStart(t *testing.T) {
	a, st, sess := setupHabitsSession(t)
	defer st.close()

	if _, err := a.answer(context.Background(), sess, habitsNone); err != nil {
		t.Fatalf("answer(habitsNone): %v", err)
	}
	state, err := a.back(sess)
	if err != nil {
		t.Fatalf("back: %v", err)
	}
	if state.Step != stepHabits || state.Question != habitsIntroQuestion {
		t.Fatalf("возврат после «нет привычек»: %+v", state)
	}
	if len(state.Options) == 0 || state.Options[0] != habitsNone {
		t.Fatalf("должны быть все варианты: %v", state.Options)
	}
}

// setupHabitsSession доводит сессию до шага habits.
func setupHabitsSession(t *testing.T) (*app, *store, *chatSession) {
	t.Helper()

	st, err := openStore(filepath.Join(t.TempDir(), "habits.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}

	a := &app{store: st}
	sess, err := st.createSession("u-habits", "Тест", 30, 1)
	if err != nil {
		st.close()
		t.Fatalf("createSession: %v", err)
	}

	for _, answer := range []string{
		"Понятно, начать опрос",
		"Боль",
		"Голова",
		"Ноющая / тупая, нарастала постепенно",
		"1–3 дня",
		"Умеренно",
		additionalNone,
		"Нет хронических",
		medsNone,
		"Аллергий нет",
		"Нет, не было",
		"Не знаю",
	} {
		if _, err := a.answer(context.Background(), sess, answer); err != nil {
			st.close()
			t.Fatalf("answer(%q): %v", answer, err)
		}
	}

	if sess.CurrentStep != stepHabits {
		st.close()
		t.Fatalf("не дошли до habits: %q", sess.CurrentStep)
	}
	return a, st, sess
}

func TestDisclaimerChildModeRoute(t *testing.T) {
	steps := stepsFor(1, 35, nil)
	disclaimer, ok := findStep(steps, stepDisclaimer)
	if !ok {
		t.Fatal("шаг disclaimer не найден")
	}
	if strings.Join(disclaimer.Options, "|") != disclaimerStartOption+"|"+disclaimerChildOption {
		t.Fatalf("кнопки дисклеймера: %v", disclaimer.Options)
	}

	childSteps := stepsFor(1, 35, map[string]string{stepDisclaimer: disclaimerChildOption})
	if len(childSteps) != 16 {
		t.Fatalf("всего шагов в детском режиме %d, ожидалось 16", len(childSteps))
	}
	if childSteps[1].ID != stepChildAge {
		t.Fatalf("после дисклеймера ожидался child_age, получено %q", childSteps[1].ID)
	}
	childAge, _ := findStep(childSteps, stepChildAge)
	if childAge.Question != "Укажите возраст ребёнка:" {
		t.Fatalf("вопрос возраста ребёнка: %q", childAge.Question)
	}
	if strings.Join(childAge.Options, "|") != strings.Join(childAgeOptions, "|") {
		t.Fatalf("варианты возраста ребёнка: %v", childAge.Options)
	}

	// Беременность в детском профиле не спрашиваем.
	femaleChild := stepsFor(2, 30, map[string]string{
		stepDisclaimer: disclaimerChildOption,
		stepChildAge:   "4–12 лет",
	})
	if _, ok := findStep(femaleChild, stepPregnancy); ok {
		t.Fatal("в режиме за ребёнка шаг pregnancy не нужен")
	}
}

func TestChildModeFlowAndBack(t *testing.T) {
	st, err := openStore(filepath.Join(t.TempDir(), "child.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.close()

	a := &app{store: st}
	sess, err := st.createSession("u-child", "Тест", 38, 1)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	state, err := a.answer(context.Background(), sess, disclaimerChildOption)
	if err != nil {
		t.Fatalf("answer(child mode): %v", err)
	}
	if state.Step != stepChildAge {
		t.Fatalf("ожидался child_age, получено %q", state.Step)
	}
	if !state.IsChild || state.Progress.Total != 16 {
		t.Fatalf("детский режим в состоянии: %+v", state)
	}

	state, err = a.answer(context.Background(), sess, "4–12 лет")
	if err != nil {
		t.Fatalf("answer(child age): %v", err)
	}
	if state.Step != stepComplaintCategory || !state.IsChild || state.ChildAge != "4–12 лет" {
		t.Fatalf("после возраста ребёнка: %+v", state)
	}

	state, err = a.back(sess)
	if err != nil {
		t.Fatalf("back на child_age: %v", err)
	}
	if state.Step != stepChildAge || !state.IsChild {
		t.Fatalf("возврат к возрасту ребёнка: %+v", state)
	}

	state, err = a.back(sess)
	if err != nil {
		t.Fatalf("back на disclaimer: %v", err)
	}
	if state.Step != stepDisclaimer || state.IsChild {
		t.Fatalf("возврат к дисклеймеру: %+v", state)
	}

	state, err = a.answer(context.Background(), sess, disclaimerStartOption)
	if err != nil {
		t.Fatalf("answer(start): %v", err)
	}
	if state.Step != stepComplaintCategory || state.IsChild || state.Progress.Total != 15 {
		t.Fatalf("после отказа от детского режима: %+v", state)
	}
}

func TestChildFeverRedFlagFlow(t *testing.T) {
	st, err := openStore(filepath.Join(t.TempDir(), "child-fever.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.close()

	a := &app{store: st}
	// Взрослый аккаунт родителя — именно поэтому нужен явный детский профиль.
	sess, err := st.createSession("u-child-fever", "Родитель", 40, 1)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	for _, answer := range []string{disclaimerChildOption, "4–12 лет"} {
		if _, err := a.answer(context.Background(), sess, answer); err != nil {
			t.Fatalf("answer(%q): %v", answer, err)
		}
	}

	state, err := a.answer(context.Background(), sess, "температура 40")
	if err != nil {
		t.Fatalf("answer(fever): %v", err)
	}
	if !state.IsEmergency {
		t.Fatalf("температура 40 у ребёнка должна давать красный флаг: %+v", state)
	}
	if !strings.Contains(state.EmergencyReason, "Температура выше 39,5") {
		t.Fatalf("причина не про температуру ребёнка: %q", state.EmergencyReason)
	}
}

func TestHabitsIntroText(t *testing.T) {
	steps := stepsFor(1, 30, nil)
	spec, ok := findStep(steps, stepHabits)
	if !ok {
		t.Fatal("шаг habits не найден")
	}
	want := "Есть ли у вас вредные привычки или факторы образа жизни из списка ниже?"
	if spec.Question != want {
		t.Fatalf("текст шага habits: %q, ожидался %q", spec.Question, want)
	}
}

func TestAllergyHistoryDoesNotTriggerEmergency(t *testing.T) {
	st, err := openStore(filepath.Join(t.TempDir(), "allergy-history.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.close()

	a := &app{store: st}
	sess, err := st.createSession("u-allergy-history", "Тест", 30, 1)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	for _, answer := range []string{
		"Понятно, начать опрос",
		"Боль",
		"Голова",
		"Ноющая / тупая, нарастала постепенно",
		"1–3 дня",
		"Умеренно",
		additionalNone,
		"Нет хронических",
		medsNone,
		"Есть аллергия на лекарства",
	} {
		if _, err := a.answer(context.Background(), sess, answer); err != nil {
			t.Fatalf("answer(%q): %v", answer, err)
		}
	}
	if sess.CurrentStep != stepAllergiesDetails {
		t.Fatalf("ожидался allergies_details, получено %q", sess.CurrentStep)
	}

	history := "аллергия на арахис и сою — начинается отёк губ и крапивница"
	// Сам детектор на таком тексте видит анафилаксию, но на шаге анамнеза он не должен обрывать опрос.
	if _, found := detectEmergency(history, 30); !found {
		t.Fatal("проверка теста: detectEmergency должен распознавать отёк губ как анафилаксию")
	}

	state, err := a.answer(context.Background(), sess, history)
	if err != nil {
		t.Fatalf("answer(allergy history): %v", err)
	}
	if state.IsEmergency || sess.IsEmergency {
		t.Fatalf("описание прошлой аллергии не должно прерывать опрос: %+v", state)
	}
	if state.Step != stepHospitalizations {
		t.Fatalf("после деталей аллергии ожидался hospitalizations, получено %q", state.Step)
	}
}

func TestTemperatureRouteSkipsSeverity(t *testing.T) {
	steps := stepsFor(1, 30, map[string]string{stepComplaintCategory: "Температура"})
	if _, ok := findStep(steps, stepComplaintSeverity); ok {
		t.Fatal("для температуры шаг «насколько сильно» не нужен")
	}
	if len(steps) != 14 {
		t.Fatalf("шагов в маршруте температуры: %d, ожидалось 14", len(steps))
	}

	st, err := openStore(filepath.Join(t.TempDir(), "temperature.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.close()

	a := &app{store: st}
	sess, err := st.createSession("u-temp", "Тест", 30, 1)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	var state chatState
	for _, answer := range []string{
		"Понятно, начать опрос",
		"Температура",
		"38.0–38.9 °C",
		"Жар и ломота во всём теле",
		"1–3 дня",
	} {
		state, err = a.answer(context.Background(), sess, answer)
		if err != nil {
			t.Fatalf("answer(%q): %v", answer, err)
		}
	}
	if state.Step != stepAdditionalSymptoms {
		t.Fatalf("после давности температуры ожидался additional_symptoms, получено %q", state.Step)
	}
	if state.Progress.Total != 14 {
		t.Fatalf("total на температуре = %d, ожидалось 14", state.Progress.Total)
	}

	// Назад из сопутствующих симптомов ведёт сразу на давность, минуя остроту.
	state, err = a.back(sess)
	if err != nil {
		t.Fatalf("back на duration: %v", err)
	}
	if state.Step != stepComplaintDuration {
		t.Fatalf("back должен вернуть на complaint_duration, получено %q", state.Step)
	}
}

func TestChildHighFeverButtonTriggersEmergency(t *testing.T) {
	option := "Выше 39.5 °C (39,8 °C)"

	if _, found := detectEmergency(option, 40); found {
		t.Fatal("на взрослом аккаунте кнопка высокой температуры не должна давать красный флаг")
	}
	if _, found := detectEmergency(option, 40, true); !found {
		t.Fatal("в детском режиме кнопка «Выше 39.5 °C» должна распознаваться как гипертермия")
	}
	if _, found := detectEmergency("39.0–39.5 °C", 40, true); found {
		t.Fatal("диапазон 39.0–39.5 °C не должен давать красный флаг")
	}

	st, err := openStore(filepath.Join(t.TempDir(), "child-temp.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.close()

	a := &app{store: st}
	sess, err := st.createSession("u-child-temp", "Родитель", 40, 1)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	var state chatState
	for _, answer := range []string{disclaimerChildOption, "4–12 лет", "Температура", option} {
		state, err = a.answer(context.Background(), sess, answer)
		if err != nil {
			t.Fatalf("answer(%q): %v", answer, err)
		}
	}
	if !state.IsEmergency || state.Step != stepEmergency {
		t.Fatalf("ожидался красный флаг гипертермии, получено %+v", state)
	}
	if !strings.Contains(state.EmergencyReason, "Температура выше 39,5") {
		t.Fatalf("причина не про температуру ребёнка: %q", state.EmergencyReason)
	}
}

func TestSymptomCharacterOptionsByCategory(t *testing.T) {
	cases := []struct {
		category string
		want     []string
	}{
		{"Боль", symptomCharacterPain},
		{"Температура", symptomCharacterTemperature},
		{"Дыхание", symptomCharacterBreathing},
		{"ЖКТ", symptomCharacterDigestive},
		{"Кожа", symptomCharacterSkin},
		{"Головокружение, онемение, сон", symptomCharacterNeuro},
		{"Травма", symptomCharacterTrauma},
		{"Неизвестная жалоба", symptomCharacterOther},
	}

	for _, tc := range cases {
		t.Run(tc.category, func(t *testing.T) {
			steps := stepsFor(1, 30, map[string]string{stepComplaintCategory: tc.category})
			spec, ok := findStep(steps, stepSymptomCharacter)
			if !ok {
				t.Fatal("шаг symptom_character не найден")
			}
			if spec.Question != "Какой характер у симптома и что его провоцирует?" {
				t.Fatalf("вопрос шага: %q", spec.Question)
			}
			if strings.Join(spec.Options, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("кнопки для %q: %v", tc.category, spec.Options)
			}
			if !spec.AllowFreeText {
				t.Fatal("характер симптома должен допускать свободный ввод")
			}
		})
	}

	steps := stepsFor(1, 30, nil)
	duration, _ := findStep(steps, stepComplaintDuration)
	wantDuration := []string{
		"Только что (меньше часа назад)",
		"Сегодня (несколько часов)",
		"1–3 дня",
		"Больше 3 дней",
		"Периодически повторяется",
	}
	if strings.Join(duration.Options, "|") != strings.Join(wantDuration, "|") {
		t.Fatalf("варианты давности: %v", duration.Options)
	}
}

// Тяжесть/«чугунная» голова после сна — бытовое недомогание, а не повод для 103.
func TestHeavyHeadAfterSleepIsNotEmergency(t *testing.T) {
	character := "Тяжесть / давящая боль после сна, усталости или стресса"
	if _, found := detectEmergency(character, 30); found {
		t.Fatal("описание тяжести после сна не должно быть красным флагом")
	}

	st, err := openStore(filepath.Join(t.TempDir(), "heavy-head.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.close()

	a := &app{store: st}
	sess, err := st.createSession("u-heavy-head", "Тест", 30, 1)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	var state chatState
	for _, answer := range []string{
		"Понятно, начать опрос",
		"Боль",
		"Голова",
		character,
		"Постепенно в течение дня / после сна",
		"Умеренно",
		additionalNone,
	} {
		state, err = a.answer(context.Background(), sess, answer)
		if err != nil {
			t.Fatalf("answer(%q): %v", answer, err)
		}
		if state.IsEmergency {
			t.Fatalf("ложный красный флаг на %q: %s", answer, state.EmergencyReason)
		}
	}
	if state.Step != stepChronic {
		t.Fatalf("после бытовой жалобы ожидался chronic, получено %q", state.Step)
	}
}

func TestAIClarificationRoute(t *testing.T) {
	marker := clarificationJSON(t, "Была ли потеря сознания или резкий удар в голову?", []string{
		"Нет, просто тяжесть",
		"Была кратковременная потеря сознания",
		"Не помню",
	})

	answers := map[string]string{
		stepComplaintCategory: "Боль",
		stepComplaintLocation: "Голова",
		stepSymptomCharacter:  "Тяжесть / давящая боль после сна, усталости или стресса",
		stepComplaintDuration: durationToday,
		stepComplaintSeverity: "Умеренно",
		aiClarificationKey:    marker,
	}

	steps := stepsFor(1, 30, answers)
	severityIndex := stepIndex(steps, stepComplaintSeverity)
	clarificationIndex := stepIndex(steps, stepAIClarification)
	if severityIndex < 0 || clarificationIndex != severityIndex+1 {
		t.Fatalf("уточнение должно идти сразу после severity: %v", stepIDs(steps))
	}
	if len(steps) != 16 {
		t.Fatalf("шагов с уточнением: %d, ожидалось 16", len(steps))
	}

	spec, _ := findStep(steps, stepAIClarification)
	if spec.Question != "Была ли потеря сознания или резкий удар в голову?" {
		t.Fatalf("вопрос уточнения: %q", spec.Question)
	}
	if len(spec.Options) != 3 || !spec.AllowFreeText {
		t.Fatalf("кнопки уточнения: %+v", spec)
	}
	if got := progressFor(steps, stepAIClarification, 6); got != (progress{Current: 7, Total: 16}) {
		t.Fatalf("прогресс на уточнении: %+v", got)
	}
}

func TestAIClarificationAnswerAdvances(t *testing.T) {
	st, err := openStore(filepath.Join(t.TempDir(), "ai-clarification.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.close()

	a := &app{store: st}
	sess, err := st.createSession("u-ai", "Тест", 30, 1)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}
	sess.Answers = map[string]string{
		stepComplaintCategory: "Боль",
		stepComplaintLocation: "Голова",
		stepSymptomCharacter:  "Тяжесть / давящая боль после сна, усталости или стресса",
		stepComplaintDuration: durationToday,
		stepComplaintSeverity: "Умеренно",
		aiClarificationKey:    clarificationJSON(t, "Когда возникла тяжесть?", []string{"После сна", "После стресса", "Постоянно"}),
	}
	sess.CurrentStep = stepAIClarification
	sess.History = []string{}
	if err := st.saveSession(sess); err != nil {
		t.Fatalf("saveSession: %v", err)
	}

	state, err := a.answer(context.Background(), sess, "После сна")
	if err != nil {
		t.Fatalf("answer(clarification): %v", err)
	}
	if state.Step != stepAdditionalSymptoms {
		t.Fatalf("после уточнения ожидался additional_symptoms, получено %q", state.Step)
	}
	pairs := sess.answerPairs()
	if !containsPair(pairs, "Когда возникла тяжесть?", "После сна") {
		t.Fatalf("ответ на уточнение не попал в историю: %+v", pairs)
	}
}

func TestAIClarificationSkippedWithoutMarker(t *testing.T) {
	steps := stepsFor(1, 30, nil)
	if _, ok := findStep(steps, stepAIClarification); ok {
		t.Fatal("без ответа DeepSeek шаг уточнения не должен появляться")
	}

	st, err := openStore(filepath.Join(t.TempDir(), "no-clarification.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.close()

	a := &app{store: st}
	sess, err := st.createSession("u-no-ai", "Тест", 30, 1)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	var state chatState
	for _, answer := range []string{
		"Понятно, начать опрос",
		"Боль",
		"Голова",
		"Ноющая / тупая, нарастала постепенно",
		"1–3 дня",
		"Умеренно",
	} {
		state, err = a.answer(context.Background(), sess, answer)
		if err != nil {
			t.Fatalf("answer(%q): %v", answer, err)
		}
	}
	if state.Step != stepAdditionalSymptoms {
		t.Fatalf("без ключа DeepSeek шаг уточнения пропускается, получено %q", state.Step)
	}
}

func TestBackAndProgressWithClarificationAndPregnancy(t *testing.T) {
	marker := clarificationJSON(t, "Когда возникла тяжесть?", []string{"После сна", "После стресса", "Постоянно"})
	answers := map[string]string{
		stepDisclaimer:        disclaimerStartOption,
		stepComplaintCategory: "Боль",
		stepComplaintLocation: "Голова",
		stepSymptomCharacter:  "Тяжесть / давящая боль после сна, усталости или стресса",
		stepComplaintDuration: durationToday,
		stepComplaintSeverity: "Умеренно",
		aiClarificationKey:    marker,
	}

	steps := stepsFor(2, 30, answers)
	if len(steps) != 17 {
		t.Fatalf("шагов с беременностью и уточнением: %d, ожидалось 17", len(steps))
	}
	if _, ok := findStep(steps, stepPregnancy); !ok {
		t.Fatal("для женщины 30 лет ожидался шаг беременности")
	}
	if _, ok := findStep(steps, stepAIClarification); !ok {
		t.Fatal("ожидался шаг уточнения DeepSeek")
	}
	if got := progressFor(steps, stepPregnancy, 12); got != (progress{Current: 14, Total: 17}) {
		t.Fatalf("прогресс на беременности с уточнением: %+v", got)
	}

	st, err := openStore(filepath.Join(t.TempDir(), "ai-back.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.close()

	a := &app{store: st}
	sess, err := st.createSession("u-ai-back", "Тест", 30, 2)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}
	sess.Answers = answers
	sess.Answers[stepAIClarification] = "После сна"
	sess.CurrentStep = stepAdditionalSymptoms
	sess.History = []string{stepComplaintSeverity, stepAIClarification}
	if err := st.saveSession(sess); err != nil {
		t.Fatalf("saveSession: %v", err)
	}

	state, err := a.back(sess)
	if err != nil {
		t.Fatalf("back на уточнение: %v", err)
	}
	if state.Step != stepAIClarification || state.Progress.Total != 17 {
		t.Fatalf("back должен вернуть на уточнение с total 17: %+v", state)
	}

	state, err = a.answer(context.Background(), sess, "После сна")
	if err != nil {
		t.Fatalf("повторный answer(уточнение): %v", err)
	}
	if state.Step != stepAdditionalSymptoms || state.Progress.Total != 17 {
		t.Fatalf("после уточнения ожидался additional_symptoms: %+v", state)
	}
}

func TestParseClarificationContent(t *testing.T) {
	content := "```json\n{\"need_clarification\": true, \"question\": \"Когда усиливается?\", \"options\": [\"Утром\", \"Вечером\", \"Постоянно\", \"Не знаю\"]}\n```"
	clarification, err := parseClarificationContent(content)
	if err != nil {
		t.Fatalf("parseClarificationContent: %v", err)
	}
	if clarification == nil || clarification.Question != "Когда усиливается?" || len(clarification.Options) != 4 {
		t.Fatalf("уточнение разобрано неверно: %+v", clarification)
	}

	noNeed, err := parseClarificationContent(`{"need_clarification": false, "question": "", "options": []}`)
	if err != nil || noNeed != nil {
		t.Fatalf("need_clarification=false должен пропускать шаг: %+v, %v", noNeed, err)
	}

	if _, err := parseClarificationContent(`{"need_clarification": true, "question": "", "options": []}`); err == nil {
		t.Fatal("ожидалась ошибка на пустой вопрос")
	}
	if _, err := parseClarificationContent(`{"need_clarification": true, "question": "Уточните?", "options": ["Да", "Нет"]}`); err == nil {
		t.Fatal("ожидалась ошибка: для уточнения нужно 3–4 варианта")
	}
}

func TestClarificationMarkerRoundTrip(t *testing.T) {
	sess := &chatSession{Answers: map[string]string{}}
	storeClarification(sess, &aiClarification{
		Need:     true,
		Question: "Была ли рвота?",
		Options:  []string{"Нет", "Один раз", "Несколько раз"},
	})

	stored := parseStoredClarification(sess.Answers)
	if stored == nil || stored.Question != "Была ли рвота?" || len(stored.Options) != 3 {
		t.Fatalf("уточнение не сохранилось: %+v", stored)
	}

	clearClarification(sess)
	if parseStoredClarification(sess.Answers) != nil {
		t.Fatal("clearClarification не удалил уточнение")
	}
}

func clarificationJSON(t *testing.T, question string, options []string) string {
	t.Helper()
	payload, err := json.Marshal(aiClarification{Need: true, Question: question, Options: options})
	if err != nil {
		t.Fatalf("marshal clarification: %v", err)
	}
	return string(payload)
}

func stepIndex(steps []stepSpec, id string) int {
	for index, step := range steps {
		if step.ID == id {
			return index
		}
	}
	return -1
}

func stepIDs(steps []stepSpec) []string {
	ids := make([]string, len(steps))
	for index, step := range steps {
		ids[index] = step.ID
	}
	return ids
}

func TestChestLocationTriggersEmergency(t *testing.T) {
	st, err := openStore(filepath.Join(t.TempDir(), "chest-button.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.close()

	a := &app{store: st}
	sess, err := st.createSession("u-chest", "Тест", 40, 1)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	var state chatState
	for _, answer := range []string{"Понятно, начать опрос", "Боль", "Грудь / грудная клетка"} {
		state, err = a.answer(context.Background(), sess, answer)
		if err != nil {
			t.Fatalf("answer(%q): %v", answer, err)
		}
	}
	if !state.IsEmergency || state.Step != stepEmergency {
		t.Fatalf("кнопка «Грудь / грудная клетка» должна сразу давать emergency: %+v", state)
	}
	if !strings.Contains(state.EmergencyReason, "груди") {
		t.Fatalf("ожидалась причина про боль в груди: %q", state.EmergencyReason)
	}
}

func TestAcuteAbdomenContextTriggersEmergency(t *testing.T) {
	st, err := openStore(filepath.Join(t.TempDir(), "acute-abdomen.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.close()

	a := &app{store: st}

	t.Run("резкий характер", func(t *testing.T) {
		sess, err := st.createSession("u-abdomen-1", "Тест", 35, 1)
		if err != nil {
			t.Fatalf("createSession: %v", err)
		}
		var state chatState
		for _, answer := range []string{
			"Понятно, начать опрос",
			"Боль",
			"Живот",
			"Резкая, острая, возникла внезапно как удар",
		} {
			state, err = a.answer(context.Background(), sess, answer)
			if err != nil {
				t.Fatalf("answer(%q): %v", answer, err)
			}
		}
		if !state.IsEmergency || !strings.Contains(state.EmergencyReason, "животе") {
			t.Fatalf("резкая боль в животе должна давать emergency: %+v", state)
		}
	})

	t.Run("нестерпимая выраженность", func(t *testing.T) {
		sess, err := st.createSession("u-abdomen-2", "Тест", 35, 1)
		if err != nil {
			t.Fatalf("createSession: %v", err)
		}
		var state chatState
		for _, answer := range []string{
			"Понятно, начать опрос",
			"Боль",
			"Живот",
			"Ноющая / тупая, нарастала постепенно",
			"1–3 дня",
			severityUnbearable,
		} {
			state, err = a.answer(context.Background(), sess, answer)
			if err != nil {
				t.Fatalf("answer(%q): %v", answer, err)
			}
		}
		if !state.IsEmergency || !strings.Contains(state.EmergencyReason, "животе") {
			t.Fatalf("нестерпимая боль в животе должна давать emergency: %+v", state)
		}
	})
}

func TestAIClarificationRedFlag(t *testing.T) {
	st, err := openStore(filepath.Join(t.TempDir(), "ai-red-flag.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.close()

	a := &app{store: st}
	sess, err := st.createSession("u-ai-red", "Тест", 40, 1)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}
	sess.Answers = map[string]string{
		stepComplaintCategory: "Боль",
		stepComplaintLocation: "Голова",
		stepSymptomCharacter:  "Тяжесть / давящая боль после сна, усталости или стресса",
		stepComplaintDuration: durationToday,
		stepComplaintSeverity: "Умеренно",
		aiClarificationKey:    clarificationJSON(t, "Была ли потеря сознания?", []string{"Нет", "Да, кратковременно", "Не помню"}),
	}
	sess.CurrentStep = stepAIClarification
	if err := st.saveSession(sess); err != nil {
		t.Fatalf("saveSession: %v", err)
	}

	state, err := a.answer(context.Background(), sess, "Да, кратковременно")
	if err != nil {
		t.Fatalf("answer(ai red flag): %v", err)
	}
	if !state.IsEmergency || state.Step != stepEmergency {
		t.Fatalf("потеря сознания на ai_clarification должна обрывать опрос: %+v", state)
	}
}

func TestHeadTraumaContextOnAIClarification(t *testing.T) {
	st, err := openStore(filepath.Join(t.TempDir(), "ai-head-trauma.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.close()

	a := &app{store: st}
	sess, err := st.createSession("u-ai-head", "Тест", 40, 1)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}
	sess.Answers = map[string]string{
		stepComplaintCategory: "Травма",
		stepComplaintLocation: "Удар по голове",
		stepSymptomCharacter:  "Небольшая боль, кожа не повреждена",
		stepComplaintDuration: durationToday,
		stepComplaintSeverity: "Умеренно",
		aiClarificationKey:    clarificationJSON(t, "Появилась ли рвота или тошнота?", []string{"Нет", "Да, была рвота", "Только тошнота"}),
	}
	sess.CurrentStep = stepAIClarification
	if err := st.saveSession(sess); err != nil {
		t.Fatalf("saveSession: %v", err)
	}

	state, err := a.answer(context.Background(), sess, "Да")
	if err != nil {
		t.Fatalf("answer(head trauma context): %v", err)
	}
	if !state.IsEmergency || !strings.Contains(state.EmergencyReason, "травмы головы") {
		t.Fatalf("рвота после удара по голове должна давать emergency: %+v", state)
	}
}

func TestSelfCareForHeavyHeadAfterSleep(t *testing.T) {
	sess := &chatSession{
		Age: 30,
		Sex: 1,
		Answers: map[string]string{
			stepComplaintCategory: "Боль",
			stepComplaintLocation: "Голова",
			stepSymptomCharacter:  "Тяжесть / давящая боль после сна, усталости или стресса",
			stepComplaintDuration: durationToday,
			stepComplaintSeverity: "Умеренно",
		},
	}

	card := localCard(sess)
	if card.Urgency != urgencySelfCare {
		t.Fatalf("для бытовой тяжести ожидался self_care, получено %q", card.Urgency)
	}
	if card.Place != "Дома / аптека при необходимости" {
		t.Fatalf("место для self_care: %q", card.Place)
	}
}

func TestAdditionalAndChronicMultiSelect(t *testing.T) {
	st, err := openStore(filepath.Join(t.TempDir(), "multi-select.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.close()

	a := &app{store: st}
	sess, err := st.createSession("u-multi", "Тест", 30, 1)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	var state chatState
	for _, answer := range []string{
		"Понятно, начать опрос",
		"Боль",
		"Голова",
		"Ноющая / тупая, нарастала постепенно",
		"1–3 дня",
		"Умеренно",
	} {
		state, err = a.answer(context.Background(), sess, answer)
		if err != nil {
			t.Fatalf("answer(%q): %v", answer, err)
		}
	}
	if state.Step != stepAdditionalSymptoms {
		t.Fatalf("ожидался additional_symptoms, получено %q", state.Step)
	}

	state, err = a.answer(context.Background(), sess, "Дыхание")
	if err != nil {
		t.Fatalf("answer(Дыхание): %v", err)
	}
	if state.Step != stepAdditionalSymptoms || !containsString(state.Options, additionalDone) {
		t.Fatalf("после выбора симптома ожидался цикл добора: %+v", state)
	}
	if containsString(state.Options, "Дыхание") || containsString(state.Options, "Боль") {
		t.Fatalf("выбранные симптомы и основная жалоба не должны повторяться: %v", state.Options)
	}

	state, err = a.answer(context.Background(), sess, "Кожа")
	if err != nil {
		t.Fatalf("answer(Кожа): %v", err)
	}
	state, err = a.answer(context.Background(), sess, additionalDone)
	if err != nil {
		t.Fatalf("answer(На этом всё): %v", err)
	}
	if state.Step != stepChronic {
		t.Fatalf("после мультивыбора ожидался chronic, получено %q", state.Step)
	}
	if !containsPair(sess.answerPairs(), "Сопутствующие симптомы", "Дыхание, Кожа") {
		t.Fatalf("мультивыбор симптомов не попал в историю: %+v", sess.answerPairs())
	}

	state, err = a.answer(context.Background(), sess, "Гипертония / ССЗ")
	if err != nil {
		t.Fatalf("answer(Гипертония): %v", err)
	}
	if state.Step != stepChronic || !containsString(state.Options, chronicDone) {
		t.Fatalf("после выбора болезни ожидался цикл добора: %+v", state)
	}
	state, err = a.answer(context.Background(), sess, "Диабет")
	if err != nil {
		t.Fatalf("answer(Диабет): %v", err)
	}
	state, err = a.answer(context.Background(), sess, chronicDone)
	if err != nil {
		t.Fatalf("answer(На этом всё, chronic): %v", err)
	}
	if state.Step != stepMedsStatus {
		t.Fatalf("после хронических ожидался meds_status, получено %q", state.Step)
	}
	if !containsPair(sess.answerPairs(), "Хронические заболевания", "Гипертония / ССЗ, Диабет") {
		t.Fatalf("мультивыбор хронических не попал в историю: %+v", sess.answerPairs())
	}
}

func TestBackInMultiSelectTrimsMessages(t *testing.T) {
	st, err := openStore(filepath.Join(t.TempDir(), "multi-back-messages.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.close()

	a := &app{store: st}
	sess, err := st.createSession("u-multi-back", "Тест", 30, 1)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	var state chatState
	for _, answer := range []string{
		"Понятно, начать опрос",
		"Боль",
		"Голова",
		"Ноющая / тупая, нарастала постепенно",
		"1–3 дня",
		"Умеренно",
	} {
		state, err = a.answer(context.Background(), sess, answer)
		if err != nil {
			t.Fatalf("answer(%q): %v", answer, err)
		}
	}
	if state.Step != stepAdditionalSymptoms {
		t.Fatalf("ожидался additional_symptoms, получено %q", state.Step)
	}

	if _, err := a.answer(context.Background(), sess, "Дыхание"); err != nil {
		t.Fatalf("answer(Дыхание): %v", err)
	}
	before := len(sess.Messages)
	state, err = a.back(sess)
	if err != nil {
		t.Fatalf("back в additional_symptoms: %v", err)
	}
	if state.Step != stepAdditionalSymptoms {
		t.Fatalf("ожидался additional_symptoms после back, получено %q", state.Step)
	}
	if len(sess.Messages) != before-2 {
		t.Fatalf("после back должно исчезнуть 2 сообщения: было %d, стало %d", before, len(sess.Messages))
	}
	for _, message := range sess.Messages {
		if message.Role == "user" && message.Text == "Дыхание" {
			t.Fatal("отменённый ответ «Дыхание» остался в ленте")
		}
	}

	if _, err := a.answer(context.Background(), sess, additionalNone); err != nil {
		t.Fatalf("answer(additionalNone): %v", err)
	}
	if _, err := a.answer(context.Background(), sess, "Гипертония / ССЗ"); err != nil {
		t.Fatalf("answer(Гипертония): %v", err)
	}
	before = len(sess.Messages)
	state, err = a.back(sess)
	if err != nil {
		t.Fatalf("back в chronic: %v", err)
	}
	if state.Step != stepChronic {
		t.Fatalf("ожидался chronic после back, получено %q", state.Step)
	}
	if len(sess.Messages) != before-2 {
		t.Fatalf("после back в chronic должно исчезнуть 2 сообщения: было %d, стало %d", before, len(sess.Messages))
	}
	for _, message := range sess.Messages {
		if message.Role == "user" && message.Text == "Гипертония / ССЗ" {
			t.Fatal("отменённый ответ «Гипертония / ССЗ» остался в ленте")
		}
	}
}

func containsPair(pairs []qaPair, question, answer string) bool {
	for _, pair := range pairs {
		if pair.Question == question && pair.Answer == answer {
			return true
		}
	}
	return false
}
