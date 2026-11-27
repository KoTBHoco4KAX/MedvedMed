package medved

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// emergencyInstruction — обязательная формулировка, которую видит пациент при красном флаге.
const emergencyInstruction = "Вызовите скорую помощь (103 или 112) прямо сейчас и не ждите окончания опроса."

const (
	actionChest       = "сядьте полусидя с опорой под спину, расстегните тесную одежду, откройте окно для притока воздуха, не ходите и не нагружайте себя, не ешьте и не пейте."
	actionDyspnea     = "сядьте, слегка наклонившись вперёд с опорой на руки, расстегните воротник и пояс, откройте окно, старайтесь дышать медленно и ровно, не ложитесь горизонтально."
	actionSyncope     = "если человек без сознания, но дышит — уложите его на бок (устойчивое боковое положение), расстегните тесную одежду, обеспечьте приток воздуха, не пытайтесь поить водой или поднимать."
	actionSeizure     = "уберите рядом острые и твёрдые предметы, подложите под голову что-то мягкое, поверните голову набок; не разжимайте челюсти, не вставляйте предметы в рот и не удерживайте человека силой."
	actionBleeding    = "при наружном кровотечении плотно прижмите рану чистой тканью или бинтом и по возможности приподнимите повреждённую конечность; при рвоте с кровью или чёрном стуле уложите человека, не давайте еду и питьё."
	actionStroke      = "уложите человека с приподнятой на 30° головой и плечами (при тошноте или сонливости — на бок), расстегните тесную одежду, не давайте воду, еду и лекарства, запомните точное время начала симптомов."
	actionAnaphylaxis = "немедленно прекратите контакт с возможным аллергеном, уложите человека на спину и приподнимите ноги (при одышке — усадите полусидя), расстегните тесную одежду, откройте окно."
	actionAbdomen     = "лягте в удобное положение (обычно на бок с подтянутыми к животу коленями), не прикладывайте грелку к животу, не ешьте, не пейте и не принимайте обезболивающие до осмотра врача (они могут смазать картину)."
	actionChildFever  = "снимите с ребёнка лишнюю одежду и не укутывайте, проветрите комнату, предлагайте прохладную воду маленькими глотками, не обтирайте спиртом или уксусом и не давайте препараты без назначения врача."
	actionHeadTrauma  = "обеспечьте полный покой, уложите человека (при тошноте/рвоте — строго на бок), не оставляйте одного, не давайте еду, воду и седативные средства."
)

// emergencyReason собирает спокойное объяснение и конкретные действия до приезда скорой.
func emergencyReason(risk, action string) string {
	return risk + "\n\nЧто делать до приезда скорой: " + action
}

// emergencyHit — сработавший красный флаг.
type emergencyHit struct {
	ID     string
	Title  string
	Reason string
}

type emergencyRule struct {
	ID      string
	Title   string
	Reason  string
	Pattern *regexp.Regexp
	Custom  func(text string, age int, isChild bool) bool
}

func (r *emergencyRule) matches(text string, age int, isChild bool) bool {
	if r.Custom != nil {
		return r.Custom(text, age, isChild)
	}
	return r.Pattern != nil && r.Pattern.MatchString(text)
}

// mustNormalizedRegexp прогоняет эталонный паттерн через ту же нормализацию,
// что и ввод пользователя: нижний регистр, ё→е и сжатие повторяющихся букв.
// Слова, которые схлопнулись (жжение → жение), дополнительно получают границу
// начала слова, чтобы не ловить «жение» внутри «движения».
func mustNormalizedRegexp(pattern string) *regexp.Regexp {
	return regexp.MustCompile(normalizePattern(pattern))
}

// emergencyRules — ровно 10 критических сценариев из ТЗ.
// Проверяются до LLM, чтобы неотложная помощь не ждала окончания опроса.
var emergencyRules = []emergencyRule{
	{
		ID:      "chest",
		Title:   "боль или сдавливание в груди",
		Reason:  emergencyReason("Боль, сдавливание или жжение в груди важно как можно быстрее проверить врачу.", actionChest),
		Pattern: mustNormalizedRegexp(`((боль|болит|боли|больно|болев[а-я]*|давит|давило|сдавлива[а-я]*|сжима[а-я]*|жмет|жжет|жжение|печет|щемит|горит|кольнул[а-я]*|прихватил[а-я]*|ноет|тяжесть|ком|дискомфорт)[а-я]* .{0,40}(груд|загрудин|за грудин|сердц))|((груд|загрудин|за грудин|сердц)[а-я]* .{0,40}(боль|болит|давит|сдавлива|сжима|жмет|жжет|жжение|печет|щемит|горит|кольнул|прихватил|ноет|тяжесть))`),
	},
	{
		ID:      "dyspnea",
		Title:   "одышка, удушье или нехватка воздуха",
		Reason:  emergencyReason("Одышку в покое, удушье или нехватку воздуха важно как можно быстрее проверить врачу.", actionDyspnea),
		Pattern: mustNormalizedRegexp(`(одышк[а-я]*|удушь[а-я]*|задыха[а-я]*|задохнул[а-я]*|не хватает воздух[а-я]*|воздуха не хватает|нехватк[а-я]* воздух[а-я]*|не надыш[а-я]*|не могу дышать|не может дышать|нечем дышать|не могу вдохнуть|не могу выдохнуть|не может вдохнуть|тяжело дышать|трудно дышать|перехватило дыхан[а-я]*|дыхание перехватил[а-я]*|приступ удушья|асфикси[а-я]*)`),
	},
	{
		ID:      "syncope",
		Title:   "потеря сознания или обморок",
		Reason:  emergencyReason("Потерю сознания или обморок важно как можно быстрее проверить врачу.", actionSyncope),
		Pattern: mustNormalizedRegexp(`(потеря[а-я]* сознани[а-я]*|сознани[а-я]* потерял[а-я]*|потерял[а-я]* сознани[а-я]*|обморок[а-я]*|обморочн[а-я]*|упал[а-я]* в обморок|упал[а-я]* без сознани[а-я]*|без сознани[а-я]*|отключил[а-я]*ся|отключк[а-я]*|терял[а-я]* сознани[а-я]*)`),
	},
	{
		ID:      "seizure",
		Title:   "судороги или приступ",
		Reason:  emergencyReason("Судороги или приступ важно как можно быстрее оценить врачу.", actionSeizure),
		Pattern: mustNormalizedRegexp(`(судорог[а-я]*|припадок|припадк[а-я]*|приступ эпилепс[а-я]*|эпилептическ[а-я]* приступ|эпиприступ|конвульси[а-я]*|бьется в судорогах|бьет в судорогах|сводит судорог[а-я]*)`),
	},
	{
		ID:      "bleeding",
		Title:   "сильное или не останавливающееся кровотечение",
		Reason:  emergencyReason("Сильное кровотечение, кровь в рвоте или стуле важно как можно быстрее проверить врачу.", actionBleeding),
		Pattern: mustNormalizedRegexp(`(кровотечени[а-я]*|кровоточи[а-я]*|кровь не останавлива[а-я]*|не останавливается кров[а-я]*|хлещет кров[а-я]*|рвота с кровью|рвот[а-я]* с кровью|рвет кровью|кровав[а-я]* рвот[а-я]*|рвота кровью|кофейн[а-я]* гущ[а-я]*|черный стул|стул черный|мелена|кровавый понос|кровь в моче|кровь в кале|кровь в стуле|кровохаркан[а-я]*|кровь в мокроте|кровь из носа не останавлива[а-я]*)`),
	},
	{
		ID:      "stroke",
		Title:   "признаки инсульта",
		Reason:  emergencyReason("Перекос лица, слабость или онемение руки или половины тела, внезапное нарушение речи или зрения важно как можно быстрее проверить врачу.", actionStroke),
		Pattern: mustNormalizedRegexp(`(перекос[а-я]* лиц[а-я]*|лицо перекосил[а-я]*|перекосило (лицо|рот)|асимметри[а-я]* лиц[а-я]*|слабость в (рук|ног|половин|тел)[а-я]*|слабость (рук|ног|половин)[а-я]*|онемел[а-я]* (рук|ног|лиц|половин|язык|тел)[а-я]*|онемени[а-я]* (рук|ног|лиц|половин|тел)[а-я]*|не чувствую (рук|ног|лиц|половин|тел)[а-я]*|нарушени[а-я]* реч[а-я]*|речь нарушил[а-я]*|не понимает реч[а-я]*|заплета[а-я]* язык|не может говорить|не может улыбнуться|улыбка перекосила[а-я]*|опущен уголок рта|внезапн[а-я]* (пропал[а-я]*|потерял[а-я]*|исчез[а-я]*) зрени[а-я]*|пропало зрени[а-я]*|потерял[а-я]* зрени[а-я]*|не вижу на один глаз|слепота на один глаз)`),
	},
	{
		ID:      "anaphylaxis",
		Title:   "признаки анафилаксии или отёка Квинке",
		Reason:  emergencyReason("Отёк лица, губ, языка или горла, затруднение глотания или дыхания после лекарства, еды или укуса важно как можно быстрее проверить врачу.", actionAnaphylaxis),
		Pattern: mustNormalizedRegexp(`(отек[а-я]* (лиц|губ|язык|горл|ше)[а-я]*|отека[а-я]* (лиц|губ|язык|горл)[а-я]*|распух[а-я]* (лиц|губ|язык|горл)[а-я]*|опух[а-я]* (лиц|губ|язык|горл)[а-я]*|анафилакс[а-я]*|отек квинке|квинке|трудно глотать|не могу глотать|аллергическ[а-я]* шок|после (укус[а-я]*|лекарств[а-я]*|еды|пищи) .{0,40}(задыха[а-я]*|отек|затруднен[а-я]* дыхан[а-я]*|глотать)|затруднен[а-я]* дыхан[а-я]* после (укус[а-я]*|лекарств[а-я]*|еды|пищи))`),
	},
	{
		ID:      "abdomen",
		Title:   "острая боль в животе",
		Reason:  emergencyReason("Внезапную сильную или нестерпимую боль в животе важно как можно быстрее проверить врачу.", actionAbdomen),
		Pattern: mustNormalizedRegexp(`(кинжальн[а-я]* .{0,40}живот[а-я]*|живот[а-я]* .{0,40}кинжальн[а-я]*|нестерпим[а-я]* .{0,40}живот[а-я]*|живот[а-я]* .{0,40}нестерпим[а-я]*|невыносим[а-я]* .{0,40}живот[а-я]*|живот[а-я]* .{0,40}невыносим[а-я]*|острая боль в живот[а-я]*|резкая боль в живот[а-я]*|сильная боль в живот[а-я]*|боль в живот[а-я]* .{0,30}(сильн|резк|остр|нестерпим|невыносим)[а-я]*|скрутило живот[а-я]*|режет живот[а-я]*|доскообразн[а-я]* живот|живот[а-я]* .{0,20}(тверд|доскообразн|каменн)[а-я]*)`),
	},
	{
		ID:     "child_fever",
		Title:  "высокая температура у ребёнка",
		Reason: emergencyReason("Температуру выше 39,5 °C у ребёнка важно как можно быстрее проверить врачу.", actionChildFever),
		Custom: feverInChild,
	},
	{
		ID:     "head_trauma",
		Title:  "травма головы с потерей сознания, рвотой или падение с высоты",
		Reason: emergencyReason("Травму головы с потерей сознания, рвотой или спутанностью, а также падение с высоты важно как можно быстрее проверить врачу.", actionHeadTrauma),
		Custom: headTraumaWithSequelae,
	},
}

var (
	temperatureCueRe = mustNormalizedRegexp(`(температур|градус|жар|t=|температурит|(^| )c( |$))`)
	childMentionRe   = mustNormalizedRegexp(`(ребен[а-я]*|малыш[а-я]*|груднич[а-я]*|младен[а-я]*|сын[а-я]*|дочь|дочк[а-я]*|дочер[а-я]*|дети|детей|детям|детях|детск[а-я]*)`)
	numberRe         = mustNormalizedRegexp(`\d+(?:\.\d+)?`)

	headTraumaRe     = mustNormalizedRegexp(`(травм[а-я]* голов[а-я]*|травм[а-я]* череп[а-я]*|черепно мозгов[а-я]* травм[а-я]*|удар[а-я]* голов[а-я]*|ударил[а-я]*ся голов[а-я]*|ударил[а-я]* голов[а-я]*|приложил[а-я]*ся голов[а-я]*|упал[а-я]* .{0,20}голов[а-я]*|сотрясени[а-я]*|ушиб[а-я]* голов[а-я]*)`)
	headSequelaeRe   = mustNormalizedRegexp(`(потеря[а-я]* сознани[а-я]*|потерял[а-я]* сознани[а-я]*|обморок[а-я]*|тошнит|тошнот[а-я]*|рвот[а-я]*|рвет|вырвало|спутанн[а-я]*|не помнит|дезориентир[а-я]*|головокружени[а-я]*|сонливост[а-я]*)`)
	fallFromHeightRe = mustNormalizedRegexp(`(упал[а-я]*|падени[а-я]*|свалил[а-я]*ся|сорвал[а-я]*ся|прыгнул[а-я]*) .{0,20}с высот[а-я]*`)

	decimalCommaRe = mustNormalizedRegexp(`(\d),(\d)`)
)

// detectEmergency ищет красные флаги в тексте пользователя (ответ, свободный ввод, выбранный вариант).
// Перед проверкой опечатки в ключевых словах исправляются по раскладке ЙЦУКЕН.
// childMode=true означает, что анкета заполняется на ребёнка (аккаунт родителя).
func detectEmergency(text string, age int, childMode ...bool) (*emergencyHit, bool) {
	isChild := len(childMode) > 0 && childMode[0]

	normalized := correctEmergencyTypos(normalizeText(text))
	if normalized == "" {
		return nil, false
	}
	for i := range emergencyRules {
		if emergencyRules[i].matches(normalized, age, isChild) {
			// Одышка только при нагрузке — не экстренный признак, если в покое дыхание нормальное.
			if emergencyRules[i].ID == "dyspnea" && (dyspneaExcluded(normalized) || mildExertionalDyspnea(normalized)) {
				continue
			}
			return &emergencyHit{
				ID:     emergencyRules[i].ID,
				Title:  emergencyRules[i].Title,
				Reason: emergencyRules[i].Reason,
			}, true
		}
	}
	return nil, false
}

// dyspneaExcluded — явные формулировки, что в покое дыхание нормальное.
func dyspneaExcluded(text string) bool {
	for _, phrase := range []string{
		"дышу нормально",
		"нормально в покое",
		"только при нагрузке",
		"нет одышки в покое",
	} {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}

// mildExertionalDyspnea отличает «одышку при нагрузке» от удушья/одышки в покое.
func mildExertionalDyspnea(text string) bool {
	if !strings.Contains(text, "одышк") {
		return false
	}
	for _, severe := range []string{
		"в покое", "удушь", "не хватает", "нехватк", "не могу дышать",
		"не может дышать", "тяжело дышать", "трудно дышать", "задыха",
	} {
		if strings.Contains(text, severe) {
			return false
		}
	}
	return strings.Contains(text, "при нагрузке") ||
		strings.Contains(text, "после нагрузки") ||
		strings.Contains(text, "при физической")
}

// Паттерны для контекстного риска боли с иррадиацией (ОКС).
var (
	// Иррадиация: левая ключица, плечо, рука, под левой лопаткой, нижняя челюсть, эпигастрий.
	radiatingLocationRe = mustNormalizedRegexp(`(лев[а-я]* .{0,25}(ключиц|плеч|рук|лопатк|челюст))|((ключиц|плеч|рук|лопатк|челюст)[а-я]* .{0,20}лев)|(под левой лопатк[а-я]*)|(нижн[а-я]* челюст[а-я]*)|(верх[а-я]* живот[а-я]*)|(эпигастр[а-я]*)|(подложеч[а-я]*)|(под ложечк[а-я]*)`)

	strongSeverityRe   = mustNormalizedRegexp(`(сильн|нестерпим|остр|резк)`)
	suddenOnsetRe      = mustNormalizedRegexp(`(только что|внезапн|сегодня|резко|как удар|за секунды|за минуты)`)
	companionSymptomRe = mustNormalizedRegexp(`(дыхан|одышк|не хватает воздух|нехватк[а-я]* воздух|невролог|головокруж|холодн[а-я]* пот|потливост|тошнот|предобморочн)`)
)

// optionRedFlags — явная привязка красного флага к конкретной кнопке.
// Контекст (локализация/категория) проверяется там, где короткая подпись
// вроде «В покое тоже» или «Тошнота / рвота» иначе была бы неоднозначной.
var optionRedFlags = []optionRedFlag{
	// Локализация
	{Step: stepComplaintLocation, Option: "Грудная клетка", HitID: "chest"},
	{Step: stepComplaintLocation, Option: "Сердце / область сердца", HitID: "chest"},
	{Step: stepComplaintLocation, Option: "Слабость в руке/ноге или нарушение речи", HitID: "stroke"},
	{Step: stepComplaintLocation, Option: "Выше 39,5 °C", HitID: "child_fever", ChildOnly: true},

	// Характер симптома
	{Step: stepSymptomCharacter, Option: "В покое тоже", Location: "Одышка при нагрузке", HitID: "dyspnea"},
	{Step: stepSymptomCharacter, Option: "Нехватка воздуха / удушье", HitID: "dyspnea"},
	{Step: stepSymptomCharacter, Option: "С болью в груди", HitID: "chest"},
	{Step: stepSymptomCharacter, Option: "Потеря сознания", HitID: "syncope"},
	{Step: stepSymptomCharacter, Option: "С нарушением речи", HitID: "stroke"},
	{Step: stepSymptomCharacter, Option: "Нарушение речи", HitID: "stroke"},
	{Step: stepSymptomCharacter, Option: "Слабость на одной стороне", HitID: "stroke"},
	{Step: stepSymptomCharacter, Option: "Слабость в одной руке", HitID: "stroke"},
	{Step: stepSymptomCharacter, Option: "Слабость в одной ноге", HitID: "stroke"},
	{Step: stepSymptomCharacter, Option: "Онемение лица", HitID: "stroke"},
	{Step: stepSymptomCharacter, Option: "Кровь в стуле", HitID: "bleeding"},
	{Step: stepSymptomCharacter, Option: "С кровью в стуле", HitID: "bleeding"},
	{Step: stepSymptomCharacter, Option: "Рвота с кровью", HitID: "bleeding"},
	{Step: stepSymptomCharacter, Option: "Кровотечение не останавливается", HitID: "bleeding"},
	{Step: stepSymptomCharacter, Option: "Тошнота / рвота", Location: "Удар по голове", HitID: "head_trauma_context"},
	{Step: stepSymptomCharacter, Option: "Спутанность / не помнит", Location: "Удар по голове", HitID: "head_trauma_context"},
	{Step: stepSymptomCharacter, Option: "Кружится голова", Location: "Удар по голове", HitID: "head_trauma_context"},
	{Step: stepSymptomCharacter, Option: "Резкая, острая / схваткообразная", Location: "Боль / тяжесть в животе", HitID: "acute_abdomen"},
	{Step: stepSymptomCharacter, Option: "Резкая, острая", Location: "Живот", HitID: "acute_abdomen"},

	// Выраженность при боли в животе
	{Step: stepComplaintSeverity, Option: "Нестерпимо / острая", Location: "Живот", HitID: "acute_abdomen"},
	{Step: stepComplaintSeverity, Option: "Нестерпимо / острая", Location: "Боль / тяжесть в животе", HitID: "acute_abdomen"},
}

type optionRedFlag struct {
	Step      string
	Option    string
	Category  string
	Location  string
	ChildOnly bool
	HitID     string
}

// optionRedFlagHit ищет явный красный флаг, привязанный к выбранной кнопке.
// Это не заменяет текстовый детектор, а страхует короткие подписи вроде
// «В покое тоже», «С кровью» или «Потеря сознания».
func optionRedFlagHit(sess *chatSession, step, option string) *emergencyHit {
	if sess == nil {
		return nil
	}
	option = strings.TrimSpace(option)
	for _, rule := range optionRedFlags {
		if rule.Step != step || !strings.EqualFold(strings.TrimSpace(rule.Option), option) {
			continue
		}
		if rule.Category != "" && !strings.EqualFold(strings.TrimSpace(sess.Answers[stepComplaintCategory]), rule.Category) {
			continue
		}
		if rule.Location != "" && !strings.EqualFold(strings.TrimSpace(sess.Answers[stepComplaintLocation]), rule.Location) {
			continue
		}
		if rule.ChildOnly && !sess.isChild() && !(sess.Age > 0 && sess.Age < 18) {
			continue
		}

		switch rule.HitID {
		case "acute_abdomen":
			return acuteAbdomenHit()
		case "head_trauma_context":
			return headTraumaContextHit()
		default:
			if hit := emergencyHitByID(rule.HitID); hit != nil {
				return hit
			}
		}
	}
	return nil
}

func acuteAbdomenHit() *emergencyHit {
	return &emergencyHit{
		ID:     "acute_abdomen",
		Title:  "острая боль в животе",
		Reason: emergencyReason("Внезапную резкую или нестерпимую боль в животе важно как можно быстрее проверить врачу.", actionAbdomen),
	}
}

func headTraumaContextHit() *emergencyHit {
	return &emergencyHit{
		ID:     "head_trauma_context",
		Title:  "травма головы с последствиями",
		Reason: emergencyReason("После удара или травмы головы появились тошнота, рвота, потеря сознания или спутанность — это важно как можно быстрее проверить врачу.", actionHeadTrauma),
	}
}

// detectComplaintEmergency ищет риск ОКС и другие контекстные красные флаги
// по совокупности ответов блока жалоб.
func detectComplaintEmergency(sess *chatSession) (*emergencyHit, bool) {
	if sess == nil {
		return nil, false
	}

	location := correctEmergencyTypos(normalizeText(sess.Answers[stepComplaintLocation]))
	character := correctEmergencyTypos(normalizeText(sess.Answers[stepSymptomCharacter]))
	severity := correctEmergencyTypos(normalizeText(sess.Answers[stepComplaintSeverity]))
	additional := correctEmergencyTypos(normalizeText(sess.Answers[stepAdditionalSymptoms]))
	clarificationRaw := sess.Answers[stepAIClarification]
	if isAffirmativeClarificationAnswer(clarificationRaw) {
		if clarification := parseStoredClarification(sess.Answers); clarification != nil {
			clarificationRaw = clarification.Question + " " + clarificationRaw
		}
	}
	clarificationAnswer := correctEmergencyTypos(normalizeText(clarificationRaw))

	// Новые кнопки характера часто сами по себе не содержат названия симптома
	// («В покое тоже», «С кровью», «Потеря сознания»). Поэтому проверяем
	// локализацию вместе с характером, чтобы не потерять красный флаг.
	if hit, found := detectEmergency(joinNonEmpty(sess.Answers[stepComplaintLocation], sess.Answers[stepSymptomCharacter]), sess.Age, sess.isChild()); found {
		return hit, true
	}

	// Грудная клетка или область сердца — красный флаг №1 из ТЗ сразу по локализации.
	if strings.Contains(location, "груд") || strings.Contains(location, "сердц") {
		if hit := emergencyHitByID("chest"); hit != nil {
			return hit, true
		}
	}

	// Контекст острого живота: локализация + резкий характер или нестерпимая боль.
	if strings.Contains(location, "живот") {
		sharpCharacter := strings.Contains(character, "резкая") && strings.Contains(character, "острая")
		unbearable := strings.Contains(severity, "нестерпим")
		if sharpCharacter || unbearable {
			return acuteAbdomenHit(), true
		}
	}

	// Контекст травмы головы: удар/травма головы + рвота, тошнота, потеря сознания, спутанность.
	headInjury := (strings.Contains(location, "голов") || strings.Contains(character, "голов") ||
		strings.Contains(location, "череп") || strings.Contains(character, "череп")) &&
		(strings.Contains(location, "удар") || strings.Contains(character, "удар") ||
			strings.Contains(location, "травм") || strings.Contains(character, "травм") ||
			strings.Contains(location, "падени") || strings.Contains(character, "падени"))
	sequelaeText := character + " " + additional + " " + clarificationAnswer
	headSequelae := false
	for _, marker := range []string{
		"рвот", "тошн", "сознани", "обморок", "спутан", "не помнит", "дезориент", "вырвало", "рвет", "круж",
	} {
		if strings.Contains(sequelaeText, marker) {
			headSequelae = true
			break
		}
	}
	if headInjury && headSequelae {
		return headTraumaContextHit(), true
	}

	complaint := correctEmergencyTypos(normalizeText(joinNonEmpty(
		sess.Answers[stepComplaintCategory],
		sess.Answers[stepComplaintLocation],
		sess.Answers[stepSymptomCharacter],
		sess.Answers[stepAdditionalSymptoms],
	)))
	if !radiatingLocationRe.MatchString(complaint) {
		return nil, false
	}

	duration := correctEmergencyTypos(normalizeText(sess.Answers[stepComplaintDuration]))

	intensified := strongSeverityRe.MatchString(severity) ||
		suddenOnsetRe.MatchString(duration) ||
		companionSymptomRe.MatchString(additional)
	if !intensified {
		return nil, false
	}

	return &emergencyHit{
		ID:     "acs_radiating",
		Title:  "боль с иррадиацией, похожая на сердечный приступ",
		Reason: emergencyReason("Боль с отдачей в левую ключицу, левое плечо или руку, под левую лопатку, в нижнюю челюсть или верхнюю часть живота, особенно сильную или внезапную, важно как можно быстрее проверить врачу.", actionChest),
	}, true
}

func emergencyHitByID(id string) *emergencyHit {
	for i := range emergencyRules {
		if emergencyRules[i].ID == id {
			return &emergencyHit{
				ID:     emergencyRules[i].ID,
				Title:  emergencyRules[i].Title,
				Reason: emergencyRules[i].Reason,
			}
		}
	}
	return nil
}

func emergencyMessage(hit *emergencyHit) string {
	tail := "Пока едет бригада: не оставайтесь один, откройте дверь, не ешьте и не пейте, не принимайте лекарства самостоятельно. Если рядом есть близкие — позовите их."
	if strings.Contains(hit.Reason, "Что делать до приезда скорой:") {
		// Индивидуальная инструкция уже содержит запреты по еде/питью и лекарствам.
		tail = "Пока едет бригада: не оставайтесь один, откройте дверь и позовите близких, если они рядом."
	}
	return fmt.Sprintf(
		"⚠️ Похоже, речь о неотложном состоянии: %s.\n\n%s\n\nПочему это важно: %s\n\n%s",
		hit.Title, emergencyInstruction, hit.Reason, tail,
	)
}

// feverInChild срабатывает на температуру > 39,5 °C, если речь о ребёнке:
// анкета заполняется за ребёнка, возраст пользователя из Core < 18 или ребёнок упомянут в тексте.
func feverInChild(text string, age int, isChild bool) bool {
	childContext := isChild || (age > 0 && age < 18) || childMentionRe.MatchString(text)
	if !childContext {
		return false
	}

	// Кнопка «Выше 39,5 °C» не содержит числа больше порога, но означает
	// температуру выше 39,5 — для ребёнка это красный флаг.
	if strings.Contains(normalizeText(text), "выше 39.5") {
		return true
	}

	maxTemp := 0.0
	for _, token := range numberRe.FindAllString(text, -1) {
		value, err := strconv.ParseFloat(token, 64)
		if err != nil {
			continue
		}
		// Отсекаем давление, вес и другие числа: температура физиологична в диапазоне 35–43.
		if value >= 35 && value <= 43 && value > maxTemp {
			maxTemp = value
		}
	}
	if maxTemp <= 39.5 {
		return false
	}

	// Если слов про температуру нет, принимаем только короткий ответ вида «39,5».
	if temperatureCueRe.MatchString(text) {
		return true
	}
	return utf8.RuneCountInString(text) <= 12
}

// headTraumaWithSequelae: травма головы с последствиями либо падение с высоты
// с потерей сознания/рвотой/спутанностью. Само падение с высоты без последствий
// красным флагом не является.
func headTraumaWithSequelae(text string, _ int, _ bool) bool {
	return headSequelaeRe.MatchString(text) &&
		(headTraumaRe.MatchString(text) || fallFromHeightRe.MatchString(text))
}

// normalizeText приводит текст к виду, удобному для регулярных выражений:
// нижний регистр, ё→е, десятичная запятая → точка, пунктуация → пробелы,
// повторяющиеся буквы схлопываются (панические «нееет», «болииит»).
func normalizeText(text string) string {
	text = strings.ToLower(text)
	text = strings.ReplaceAll(text, "ё", "е")
	text = decimalCommaRe.ReplaceAllString(text, "$1.$2")

	var builder strings.Builder
	prevSpace := true
	for _, r := range text {
		isWord := (r >= 'а' && r <= 'я') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.'
		if isWord {
			builder.WriteRune(r)
			prevSpace = false
			continue
		}
		if !prevSpace {
			builder.WriteByte(' ')
			prevSpace = true
		}
	}
	return collapseRepeatedLetters(strings.TrimSpace(builder.String()))
}

// collapseRepeatedLetters убирает подряд идущие дубли букв, не трогая цифры и точку.
func collapseRepeatedLetters(text string) string {
	var builder strings.Builder
	var previous rune
	for _, r := range text {
		isLetter := (r >= 'а' && r <= 'я') || (r >= 'a' && r <= 'z')
		if isLetter && r == previous {
			continue
		}
		builder.WriteRune(r)
		previous = r
	}
	return builder.String()
}

// normalizeReference прогоняет эталонные слова, корни и паттерны детектора
// через ту же нормализацию, что и ввод пользователя: нижний регистр, ё→е,
// сжатие повторяющихся букв. Поэтому «жжение» в словаре превращается в «жение»
// и совпадает с нормализованным «жжением» из текста пациента.
func normalizeReference(reference string) string {
	reference = strings.ToLower(reference)
	reference = strings.ReplaceAll(reference, "ё", "е")
	return collapseRepeatedLetters(reference)
}

// normalizePattern нормализует литеральные слова внутри регулярного паттерна.
// Слова с удвоенными буквами после сжатия получают границу начала слова:
// альтернатива «жение» не должна срабатывать на «движение».
func normalizePattern(pattern string) string {
	pattern = strings.ToLower(pattern)
	pattern = strings.ReplaceAll(pattern, "ё", "е")

	var builder strings.Builder
	var word strings.Builder
	flush := func() {
		literal := word.String()
		word.Reset()
		if literal == "" {
			return
		}
		collapsed := collapseRepeatedLetters(literal)
		if collapsed != literal {
			builder.WriteString("(^| )")
		}
		builder.WriteString(collapsed)
	}

	for _, r := range pattern {
		if (r >= 'а' && r <= 'я') || (r >= 'a' && r <= 'z') {
			word.WriteRune(r)
			continue
		}
		flush()
		builder.WriteRune(r)
	}
	flush()

	return builder.String()
}

// normalizeVocabulary нормализует список эталонных слов детектора.
func normalizeVocabulary(words []string) []string {
	normalized := make([]string, len(words))
	for i, word := range words {
		normalized[i] = normalizeReference(word)
	}
	return normalized
}

// ruKeyCoords — координаты клавиш ЙЦУКЕН для оценки соседства при опечатках.
var ruKeyCoords = map[rune][2]float64{
	// верхний ряд
	'й': {0, 0}, 'ц': {1, 0}, 'у': {2, 0}, 'к': {3, 0}, 'е': {4, 0}, 'н': {5, 0},
	'г': {6, 0}, 'ш': {7, 0}, 'щ': {8, 0}, 'з': {9, 0}, 'х': {10, 0}, 'ъ': {11, 0},
	// средний ряд
	'ф': {0.25, 1}, 'ы': {1.25, 1}, 'в': {2.25, 1}, 'а': {3.25, 1}, 'п': {4.25, 1},
	'р': {5.25, 1}, 'о': {6.25, 1}, 'л': {7.25, 1}, 'д': {8.25, 1}, 'ж': {9.25, 1},
	'э': {10.25, 1},
	// нижний ряд
	'я': {0.75, 2}, 'ч': {1.75, 2}, 'с': {2.75, 2}, 'м': {3.75, 2}, 'и': {4.75, 2},
	'т': {5.75, 2}, 'ь': {6.75, 2}, 'б': {7.75, 2}, 'ю': {8.75, 2},
}

// fuzzyVocabulary — канонические слова, которые ищутся с учётом опечаток.
// Только они участвуют в исправлении, поэтому обычные слова не переписываются.
var fuzzyVocabulary = []string{
	// грудь и сердце
	"грудь", "грудина", "груди", "грудине", "грудину", "грудной",
	"сердце", "сердца", "сердцу", "сердцем",
	"давит", "давило", "сдавливает", "жжение", "сжимает", "тяжесть", "щемит",
	// дыхание
	"одышка", "одышку", "одышкой", "удушье", "удушья", "удушьем",
	"задыхаюсь", "задыхается", "задыхаются", "задохнулся", "воздуха", "воздух", "дышать",
	// сознание
	"обморок", "обмороке", "сознание", "сознания", "сознании", "обморочный", "отключился",
	// судороги
	"судороги", "судорогах", "судорогами", "припадок", "припадке", "конвульсии", "конвульсий", "эпилепсия",
	// кровотечение
	"кровотечение", "кровотечения", "кровотечением", "кровь", "крови", "кровью",
	"кровавая", "кровохарканье", "мелена",
	// инсульт
	"перекос", "перекосило", "онемела", "онемение", "онемели", "слабость", "речью", "зрение", "зрения",
	// анафилаксия
	"отек", "отека", "отеком", "анафилаксия", "распух", "глотать", "квинке",
	"горло", "горле", "языка", "язык",
	// живот
	"живот", "животе", "живота", "животом", "кинжальная", "нестерпимая", "резкая", "острая",
	// температура
	"температура", "градус", "температурит",
	// голова
	"травма", "голова", "головы", "голову", "головой", "сотрясение", "высота", "высоты", "ударился",
	// локализации ОКС
	"ключица", "плечо", "лопатка", "челюсть", "эпигастрий",
}

// normalizedFuzzyVocabulary — те же слова после нормализации (жжение → жение):
// сравнение идёт с нормализованным вводом пользователя.
var normalizedFuzzyVocabulary = normalizeVocabulary(fuzzyVocabulary)

// emergencyTypoAliases — частые опечатки, которые не проходят порог
// для коротких слов, но должны приводить к каноническому термину.
var emergencyTypoAliases = map[string]string{
	"давыт": "давит",
}

// correctEmergencyTypos исправляет слова-опечатки на канонические термины.
func correctEmergencyTypos(text string) string {
	if text == "" {
		return ""
	}

	words := strings.Fields(text)
	changed := false
	for i, word := range words {
		candidate := strings.Trim(word, ".")
		if !isCyrillicWord(candidate) {
			continue
		}
		if alias, ok := emergencyTypoAliases[candidate]; ok {
			words[i] = alias
			changed = true
		} else if canonical, ok := fuzzyCanonical(candidate); ok {
			words[i] = canonical
			changed = true
		} else if candidate != word {
			words[i] = candidate
			changed = true
		}
	}
	if !changed {
		return text
	}
	return strings.Join(words, " ")
}

func isCyrillicWord(word string) bool {
	if word == "" {
		return false
	}
	for _, r := range word {
		if r < 'а' || r > 'я' {
			return false
		}
	}
	return true
}

// fuzzyCanonical ищет ближайшее каноническое слово в пределах порога.
func fuzzyCanonical(word string) (string, bool) {
	runes := []rune(word)
	if len(runes) < 5 {
		return "", false
	}

	// Порог не превышает 1.25, но для коротких слов он строже,
	// чтобы обычные слова не переписывались: «бровь» ≠ «кровь», «дыхание» ≠ «жжение».
	limit := 1.25
	switch len(runes) {
	case 5:
		limit = 0.75
	case 6, 7:
		limit = 1.0
	}

	best := ""
	bestDistance := math.Inf(1)
	for _, keyword := range normalizedFuzzyVocabulary {
		keywordRunes := []rune(keyword)
		if len(keywordRunes) < 5 {
			continue
		}
		if absInt(len(runes)-len(keywordRunes)) > 5 {
			continue
		}
		distance := weightedLevenshtein(runes, keywordRunes)
		if distance < bestDistance {
			bestDistance = distance
			best = keyword
		}
	}

	if best == "" || bestDistance > limit+1e-9 {
		return "", false
	}
	return best, true
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// weightedLevenshtein: замена на соседнюю клавишу — 0.25, вставка/пропуск — 0.5,
// замена на далёкую клавишу — 1.0.
func weightedLevenshtein(a, b []rune) float64 {
	rows, cols := len(a), len(b)
	previous := make([]float64, cols+1)
	current := make([]float64, cols+1)

	for j := 1; j <= cols; j++ {
		previous[j] = previous[j-1] + 0.5
	}

	for i := 1; i <= rows; i++ {
		current[0] = previous[0] + 0.5
		for j := 1; j <= cols; j++ {
			cost := 0.0
			if a[i-1] != b[j-1] {
				if adjacentKeys(a[i-1], b[j-1]) {
					cost = 0.25
				} else {
					cost = 1.0
				}
			}
			current[j] = math.Min(
				previous[j-1]+cost,
				math.Min(previous[j]+0.5, current[j-1]+0.5),
			)
		}
		previous, current = current, previous
	}
	return previous[cols]
}

// adjacentKeys считает клавиши соседними с учётом вертикальных и диагональных связей.
func adjacentKeys(a, b rune) bool {
	first, okA := ruKeyCoords[a]
	second, okB := ruKeyCoords[b]
	if !okA || !okB {
		return false
	}
	dx := first[0] - second[0]
	dy := first[1] - second[1]
	return math.Hypot(dx, dy) <= 1.3
}
