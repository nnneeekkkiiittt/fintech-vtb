package notifications

import "sort"

// Suggestion — рекомендованный конверт. Priority: 1 — самый важный, 5 — наименее важный.
// Percent — доля пополнения, рассчитанная по приоритетам (сумма по всем рекомендациям = 100).
type Suggestion struct {
	Name     string   `json:"name"`
	Category string   `json:"category"`
	Priority int      `json:"priority"`
	Percent  int      `json:"percent"`
	Reasons  []string `json:"reasons"`
}

// Категории конвертов в бэкенде ограничены 40 символами, поэтому используем короткие названия групп.
var envelopeCategories = map[string]string{
	"Продукты и товары для дома":    "Повседневные",
	"Жильё и коммунальные услуги":   "Обязательные",
	"Телефон, интернет и подписки":  "Обязательные",
	"Кредиты и долги":               "Обязательные",
	"Семья и близкие":               "Семья",
	"Кафе, рестораны и развлечения": "Образ жизни",
	"Транспорт":                     "Транспорт",
	"Одежда и покупки":              "Покупки",
	"Красота и личный уход":         "Образ жизни",
	"Здоровье":                      "Здоровье",
	"Путешествия и отпуск":          "Отдых",
	"Обучение, развитие и хобби":    "Развитие",
	"Резервный фонд":                "Накопления",
	"Накопления на цели":            "Накопления",
	"Свободные траты":               "Свободные",
}

// has проверяет, выбран ли конкретный вариант ответа.
func has(a Answers, question, option int) bool {
	for _, selected := range a[question] {
		if selected == option {
			return true
		}
	}
	return false
}

func addSuggestion(list *[]*Suggestion, name string, priority int, reason string) {
	for _, s := range *list {
		if s.Name == name {
			if priority > s.Priority {
				s.Priority = priority
			}
			s.Reasons = append(s.Reasons, reason)
			return
		}
	}
	category := envelopeCategories[name]
	if category == "" {
		category = "Другое"
	}
	*list = append(*list, &Suggestion{
		Name:     name,
		Category: category,
		Priority: priority,
		Reasons:  []string{reason},
	})
}

// Recommend подбирает конверты по ответам анкеты и сразу считает доли пополнения.
// Логика подбора и приоритеты перенесены из прототипа без изменений.
func Recommend(a Answers) []Suggestion {
	var result []*Suggestion

	// Базовые категории, которые обычно нужны для планирования бюджета.
	addSuggestion(&result, "Продукты и товары для дома", 1, "Базовая категория повседневных расходов")
	addSuggestion(&result, "Свободные траты", 4, "Деньги на личные покупки и спонтанные расходы")

	// Обязательные платежи из вопроса 4.
	if has(a, 4, 1) || has(a, 4, 2) || has(a, 4, 3) {
		addSuggestion(&result, "Жильё и коммунальные услуги", 1, "Есть обязательные расходы на жильё")
	}
	if has(a, 4, 5) || has(a, 4, 7) {
		addSuggestion(&result, "Телефон, интернет и подписки", 2, "Есть регулярные платежи за связь или сервисы")
	}
	if has(a, 4, 6) || has(a, 13, 2) || has(a, 13, 3) || has(a, 13, 4) || has(a, 13, 5) {
		addSuggestion(&result, "Кредиты и долги", 1, "Есть кредитные или долговые обязательства")
	}
	if has(a, 4, 8) || has(a, 4, 9) || has(a, 6, 1) || has(a, 6, 2) {
		addSuggestion(&result, "Семья и близкие", 2, "Есть семейные расходы или помощь близким")
	}

	// Категории по регулярным тратам из вопроса 7.
	if has(a, 7, 2) {
		addSuggestion(&result, "Кафе, рестораны и развлечения", 3, "Есть расходы на кафе, доставку или досуг")
	}
	if has(a, 7, 3) {
		addSuggestion(&result, "Транспорт", 2, "Есть расходы на транспорт")
	}
	if has(a, 7, 5) || has(a, 7, 8) {
		addSuggestion(&result, "Одежда и покупки", 3, "Есть расходы на одежду или онлайн-покупки")
	}
	if has(a, 7, 6) {
		addSuggestion(&result, "Красота и личный уход", 4, "Есть расходы на красоту и уход")
	}
	if has(a, 7, 7) {
		addSuggestion(&result, "Здоровье", 2, "Есть расходы на здоровье")
	}
	if has(a, 7, 9) {
		addSuggestion(&result, "Путешествия и отпуск", 4, "Есть расходы или интерес к поездкам")
	}
	if has(a, 7, 10) || has(a, 7, 11) {
		addSuggestion(&result, "Обучение, развитие и хобби", 4, "Есть расходы на обучение, спорт или хобби")
	}

	// Цели накоплений из вопросов 12 и 15.
	if has(a, 12, 1) || has(a, 15, 1) || has(a, 15, 2) || has(a, 15, 3) {
		addSuggestion(&result, "Резервный фонд", 2, "Пользователь хочет или готов формировать резерв")
	}
	if len(a[12]) > 0 && !has(a, 12, 11) {
		addSuggestion(&result, "Накопления на цели", 3, "Выбрана одна или несколько финансовых целей")
	}

	// Ответы о перерасходе (вопрос 9) повышают приоритет уже существующих категорий.
	// Сам по себе перерасход не создаёт категорию, если она не была выбрана ранее.
	overspend := []struct {
		option int
		name   string
	}{
		{1, "Продукты и товары для дома"},
		{2, "Кафе, рестораны и развлечения"},
		{3, "Транспорт"},
		{4, "Кафе, рестораны и развлечения"},
		{5, "Одежда и покупки"},
		{6, "Одежда и покупки"},
		{7, "Красота и личный уход"},
		{8, "Путешествия и отпуск"},
		{9, "Телефон, интернет и подписки"},
	}
	for _, item := range overspend {
		if !has(a, 9, item.option) {
			continue
		}
		for _, s := range result {
			if s.Name == item.name && s.Priority > 1 {
				s.Priority--
				s.Reasons = append(s.Reasons, "В этой категории пользователь отмечает перерасход")
			}
		}
	}

	// Если обязательные расходы занимают большую часть дохода, повышаем важность
	// обязательных категорий, которые уже присутствуют в списке.
	if has(a, 5, 4) || has(a, 5, 5) {
		for _, s := range result {
			if s.Name == "Жильё и коммунальные услуги" || s.Name == "Кредиты и долги" ||
				s.Name == "Продукты и товары для дома" {
				if s.Priority > 1 {
					s.Priority--
				}
			}
		}
	}

	// Стабильная сортировка: сначала более важные категории.
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Priority < result[j].Priority
	})

	items := make([]Suggestion, len(result))
	for i, s := range result {
		items[i] = *s
	}
	assignPercents(items)
	return items
}

// assignPercents превращает приоритеты в проценты пополнения. Веса: приоритет 1 = 5,
// 2 = 4, 3 = 3, 4 = 2, 5 = 1 (как в прототипе). Бэкенд требует целые проценты с суммой
// ровно 100, поэтому остаток раздаётся по методу наибольших остатков.
func assignPercents(items []Suggestion) {
	weights := map[int]int{1: 5, 2: 4, 3: 3, 4: 2, 5: 1}
	weightOf := func(priority int) int {
		if w, ok := weights[priority]; ok {
			return w
		}
		return 1
	}

	total := 0
	for _, item := range items {
		total += weightOf(item.Priority)
	}
	if total == 0 {
		return
	}

	type remainder struct {
		index int
		value int
	}
	remainders := make([]remainder, len(items))
	sum := 0
	for i := range items {
		numerator := 100 * weightOf(items[i].Priority)
		items[i].Percent = numerator / total
		remainders[i] = remainder{index: i, value: numerator % total}
		sum += items[i].Percent
	}
	sort.SliceStable(remainders, func(a, b int) bool {
		return remainders[a].value > remainders[b].value
	})
	for i := 0; sum < 100; i++ {
		items[remainders[i%len(remainders)].index].Percent++
		sum++
	}
}
