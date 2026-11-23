package main

import (
	"strings"
	"testing"
)

func TestDetectEmergencyPositive(t *testing.T) {
	cases := []struct {
		name string
		text string
		age  int
	}{
		{"боль в груди", "Сильно давит в груди, отдаёт в левую руку", 50},
		{"жжение за грудиной", "Жжение за грудиной после нагрузки", 60},
		{"одышка", "Задыхаюсь, не хватает воздуха даже в покое", 40},
		{"обморок", "Утром был обморок", 30},
		{"судороги", "У меня были судороги и припадок", 25},
		{"кровотечение", "Сильное кровотечение не останавливается", 45},
		{"инсульт FAST", "Перекос лица и слабость в руке", 70},
		{"анафилаксия", "Отёк лица и губ после укуса, трудно дышать", 20},
		{"живот", "Кинжальная боль в животе", 35},
		{"температура ребёнка по упоминанию", "У ребёнка температура 40", 30},
		{"температура ребёнка по возрасту", "Температура 39,8", 7},
		{"травма головы", "Ударился головой, была потеря сознания", 22},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hit, found := detectEmergency(tc.text, tc.age)
			if !found {
				t.Fatalf("detectEmergency(%q, age=%d) = false, ожидался красный флаг", tc.text, tc.age)
			}
			if hit == nil || hit.Reason == "" {
				t.Fatalf("detectEmergency вернул пустое пояснение: %+v", hit)
			}
		})
	}
}

func TestDetectEmergencyNegative(t *testing.T) {
	cases := []struct {
		name string
		text string
		age  int
	}{
		{"обычная головная боль", "Болит голова третий день", 30},
		{"температура взрослого", "Температура 39, врач сказал пить больше воды", 30},
		{"температура ребёнка ниже порога", "У ребёнка температура 39.0", 30},
		{"наследственность: инсульт у отца", "У отца был инсульт", 40},
		{"травма головы без последствий", "Ударился головой, немного болит", 22},
		{"плановая консультация", "Хочу проверить давление и холестерин", 45},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if hit, found := detectEmergency(tc.text, tc.age); found {
				t.Fatalf("detectEmergency(%q) = true (%s), ожидался false", tc.text, hit.Title)
			}
		})
	}
}

func TestEmergencyMessageHasInstruction(t *testing.T) {
	message := emergencyMessage(&emergencyHit{
		Title:  "боль в груди",
		Reason: "может быть признаком инфаркта",
	})
	for _, want := range []string{"Вызовите скорую помощь (103 или 112)", "не ждите окончания опроса"} {
		if !containsAny(message, want) {
			t.Fatalf("в сообщении нет фразы %q:\n%s", want, message)
		}
	}
}

// Полная последовательность из ТЗ: боль -> ключица левая -> внезапное начало -> сильно.
func TestRadiatingPainEmergencyFullSequence(t *testing.T) {
	sess := &chatSession{
		Age: 48,
		Sex: 1,
		Answers: map[string]string{
			stepComplaintCategory: "Боль",
			stepComplaintLocation: "ключица левая",
			stepComplaintDuration: durationSudden,
			stepComplaintSeverity: severityStrong,
		},
	}

	hit, found := detectComplaintEmergency(sess)
	if !found {
		t.Fatal("последовательность должна давать красный флаг ОКС")
	}
	if !strings.Contains(hit.Reason, "коронар") {
		t.Fatalf("пояснение не про коронарный синдром: %q", hit.Reason)
	}
	if hit.Title == "" || hit.ID != "acs_radiating" {
		t.Fatalf("неверный красный флаг: %+v", hit)
	}
}

func TestComplaintEmergencyContext(t *testing.T) {
	cases := []struct {
		name    string
		answers map[string]string
		want    bool
	}{
		{
			"внезапное начало",
			map[string]string{stepComplaintCategory: "Боль", stepComplaintLocation: "ключица левая", stepComplaintDuration: durationSudden},
			true,
		},
		{
			"высокая интенсивность",
			map[string]string{stepComplaintCategory: "Боль", stepComplaintLocation: "левая рука", stepComplaintSeverity: severityStrong},
			true,
		},
		{
			"нестерпимая боль под левой лопаткой",
			map[string]string{stepComplaintCategory: "Боль", stepComplaintLocation: "под левой лопаткой", stepComplaintSeverity: severityUnbearable},
			true,
		},
		{
			"нижняя челюсть + дыхание",
			map[string]string{stepComplaintCategory: "Боль", stepComplaintLocation: "нижняя челюсть", stepAdditionalSymptoms: "Дыхание"},
			true,
		},
		{
			"эпигастрий + тошнота",
			map[string]string{stepComplaintCategory: "Боль", stepComplaintLocation: "верх живота", stepAdditionalSymptoms: "тошнота"},
			true,
		},
		{
			"слабая боль без усиливающих факторов",
			map[string]string{
				stepComplaintCategory:  "Боль",
				stepComplaintLocation:  "ключица левая",
				stepComplaintDuration:  "1–3 дня",
				stepComplaintSeverity:  "Слабо, почти не мешает",
				stepAdditionalSymptoms: additionalNone,
			},
			false,
		},
		{
			"боль без иррадиации",
			map[string]string{
				stepComplaintCategory: "Боль",
				stepComplaintLocation: "Голова",
				stepComplaintDuration: durationSudden,
				stepComplaintSeverity: severityStrong,
			},
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, found := detectComplaintEmergency(&chatSession{Answers: tc.answers})
			if found != tc.want {
				t.Fatalf("detectComplaintEmergency = %v, ожидалось %v (answers=%v)", found, tc.want, tc.answers)
			}
		})
	}
}

func TestRadiatingLocationsRecognized(t *testing.T) {
	locations := []string{
		"левая ключица",
		"левое плечо",
		"левая рука",
		"под левой лопаткой",
		"нижняя челюсть",
		"верх живота",
		"эпигастрий",
	}

	for _, location := range locations {
		sess := &chatSession{Answers: map[string]string{
			stepComplaintCategory: "Боль",
			stepComplaintLocation: location,
			stepComplaintSeverity: severityStrong,
		}}
		if _, found := detectComplaintEmergency(sess); !found {
			t.Fatalf("локализация %q должна считаться иррадиирующей", location)
		}
	}
}

// В детекторе ровно 10 категорий из ТЗ плюс контекстная иррадиация ОКС.
func TestEmergencyRulesCount(t *testing.T) {
	want := []string{
		"chest", "dyspnea", "syncope", "seizure", "bleeding",
		"stroke", "anaphylaxis", "abdomen", "child_fever", "head_trauma",
	}
	if len(emergencyRules) != len(want) {
		t.Fatalf("категорий %d, ожидалось %d", len(emergencyRules), len(want))
	}
	for i, id := range want {
		if emergencyRules[i].ID != id {
			t.Fatalf("категория %d: %q, ожидалась %q", i, emergencyRules[i].ID, id)
		}
	}
}

func TestFuzzyRemapOfTypos(t *testing.T) {
	cases := map[string]string{
		"ключиуца": "ключица",
		"кобчмцм":  "ключица",
		"одышпка":  "одышка",
		"грудб":    "грудь",
	}

	for source, want := range cases {
		if got := correctEmergencyTypos(source); got != want {
			t.Fatalf("correctEmergencyTypos(%q) = %q, ожидалось %q", source, got, want)
		}
	}
}

func TestFuzzyDistanceWeights(t *testing.T) {
	// Одна замена на соседнюю клавишу (б ↔ ь) — 0.25.
	if distance := weightedLevenshtein([]rune("грудб"), []rune("грудь")); distance > 0.25+1e-9 {
		t.Fatalf("замена соседней клавиши: %.2f, ожидалось 0.25", distance)
	}
	// Одна вставка — 0.5.
	if distance := weightedLevenshtein([]rune("одышпка"), []rune("одышка")); distance > 0.5+1e-9 {
		t.Fatalf("вставка: %.2f, ожидалось 0.5", distance)
	}
	// Далёкая замена — 1.0, поэтому «бровь» не должна становиться «кровью».
	if distance := weightedLevenshtein([]rune("бровь"), []rune("кровь")); distance < 0.999 {
		t.Fatalf("далёкая замена: %.2f, ожидалось 1.0", distance)
	}
}

func TestFuzzyTyposTriggerEmergency(t *testing.T) {
	cases := []struct {
		name string
		text string
		age  int
	}{
		{"одышпка", "одышпка", 30},
		{"грудб с болью", "сильно болит грудб", 40},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, found := detectEmergency(tc.text, tc.age); !found {
				t.Fatalf("detectEmergency(%q) = false, ожидался красный флаг", tc.text)
			}
		})
	}
}

func TestFuzzyTyposInACSContext(t *testing.T) {
	cases := []string{
		"левая ключиуца",
		"левая кобчмцм",
	}

	for _, location := range cases {
		sess := &chatSession{Answers: map[string]string{
			stepComplaintCategory: "Боль",
			stepComplaintLocation: location,
			stepComplaintSeverity: severityStrong,
		}}
		if _, found := detectComplaintEmergency(sess); !found {
			t.Fatalf("локализация %q должна давать риск ОКС", location)
		}
	}
}

func TestFuzzyDoesNotRewriteOrdinaryWords(t *testing.T) {
	for _, word := range []string{"бровь", "дыхание", "левое"} {
		if got := correctEmergencyTypos(word); got != word {
			t.Fatalf("обычное слово %q переписано в %q", word, got)
		}
	}

	if _, found := detectEmergency("болит бровь", 30); found {
		t.Fatal("обычная боль в брови не должна быть красным флагом")
	}
}

func TestChildFeverThreshold(t *testing.T) {
	cases := []struct {
		name string
		text string
		age  int
		want bool
	}{
		{"ребёнок 7 лет, 39,8", "температура 39,8", 7, true},
		{"ребёнок 17 лет, 39,9", "температура 39,9", 17, true},
		{"ровно 39,5 — не выше порога", "температура 39,5", 7, false},
		{"39,6 без слова температура", "39,6", 7, true},
		{"взрослый 30 лет, 40", "температура 40", 30, false},
		{"взрослый, но упомянут ребёнок", "у ребёнка температура 40", 30, true},
		{"18 лет — уже не ребёнок", "температура 39,9", 18, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, found := detectEmergency(tc.text, tc.age)
			if found != tc.want {
				t.Fatalf("detectEmergency(%q, age=%d) = %v, ожидалось %v", tc.text, tc.age, found, tc.want)
			}
		})
	}
}

func TestExpandedSynonyms(t *testing.T) {
	cases := []struct {
		name string
		text string
		age  int
	}{
		{"кофейная гуща", "рвота кофейной гущей", 50},
		{"кровь в моче", "кровь в моче", 50},
		{"кровохарканье", "кровохарканье", 50},
		{"отек квинке", "отек квинке, трудно глотать", 40},
		{"пропало зрение", "внезапно пропало зрение на один глаз", 60},
		{"падение с высоты", "упал с высоты", 30},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, found := detectEmergency(tc.text, tc.age); !found {
				t.Fatalf("detectEmergency(%q) = false, ожидался красный флаг", tc.text)
			}
		})
	}
}

// Эталонные словари детектора нормализуются так же, как ввод пользователя:
// «жжение» в паттерне и в тексте одинаково сжимается до «жение».
func TestDictionaryNormalizationForDoubledLetters(t *testing.T) {
	cases := []struct {
		word       string
		normalized string
	}{
		{"жжение", "жение"},
		{"аллергический", "алергический"},
		{"асимметрия", "асиметрия"},
		{"доскообразный", "доскобразный"},
		{"конвульсии", "конвульси"},
	}

	for _, tc := range cases {
		if got := normalizeText(tc.word); got != tc.normalized {
			t.Fatalf("normalizeText(%q) = %q, ожидалось %q", tc.word, got, tc.normalized)
		}
		if got := normalizeReference(tc.word); got != tc.normalized {
			t.Fatalf("normalizeReference(%q) = %q, ожидалось %q", tc.word, got, tc.normalized)
		}
	}

	// Паттерны сжаты вместе со словами, поэтому удвоенные буквы в тексте не ломают детекцию.
	for _, text := range []string{
		"жжение в груди",
		"аллергический шок",
		"доскообразный живот",
		"конвульсии",
	} {
		if _, found := detectEmergency(text, 30); !found {
			t.Fatalf("detectEmergency(%q) = false, ожидался красный флаг", text)
		}
	}
}

// Детский профиль включает правило температуры даже на взрослом аккаунте родителя.
func TestChildModeFeverFlag(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		age   int
		child bool
		want  bool
	}{
		{"ребёнок, 40", "температура 40", 40, true, true},
		{"ребёнок, 39,6", "температура 39,6", 40, true, true},
		{"ребёнок, ровно 39,5", "температура 39,5", 40, true, false},
		{"без детского профиля, 40", "температура 40", 40, false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, found := detectEmergency(tc.text, tc.age, tc.child)
			if found != tc.want {
				t.Fatalf("detectEmergency(%q, age=%d, child=%v) = %v, ожидалось %v",
					tc.text, tc.age, tc.child, found, tc.want)
			}
		})
	}
}

func TestExertionalDyspneaIsNotEmergency(t *testing.T) {
	if hit, found := detectEmergency("Одышка при нагрузке", 40); found {
		t.Fatalf("одышка при нагрузке не должна быть экстренным флагом: %s", hit.Title)
	}
	if _, found := detectEmergency("Одышка в покое", 40); !found {
		t.Fatal("одышка в покое должна оставаться красным флагом")
	}
}

func TestDyspneaWithNormalRestIsNotEmergency(t *testing.T) {
	if hit, found := detectEmergency("Дышу нормально в покое, одышка только при нагрузке", 40); found {
		t.Fatalf("нормальное дыхание в покое не должно быть экстренным флагом: %s", hit.Title)
	}
	if _, found := detectEmergency("Нет одышки в покое, одышка только при нагрузке", 40); found {
		t.Fatal("отрицание одышки в покое не должно давать красный флаг")
	}
	if _, found := detectEmergency("Одышка в покое", 40); !found {
		t.Fatal("одышка в покое должна оставаться красным флагом")
	}
}

// Убедимся, что обычные фразы не дают ложных срабатываний.
func TestNoFalsePositiveOnPlainText(t *testing.T) {
	plain := []string{
		"здравствуйте, хочу записаться к врачу",
		"температура 36,6, чувствую себя нормально",
		"болит бровь после удара дверью",
		"движение в груди без боли",
	}
	for _, text := range plain {
		if hit, found := detectEmergency(text, 30); found {
			t.Fatalf("ложный красный флаг на %q: %s", text, hit.Title)
		}
	}
	if strings.TrimSpace(correctEmergencyTypos("")) != "" {
		t.Fatal("пустая строка должна оставаться пустой")
	}
}
