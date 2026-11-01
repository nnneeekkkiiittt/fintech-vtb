package domain

import "sort"

// Answers хранит выбранные варианты ответа: номер вопроса (с 1) → номера вариантов (с 1).
// В JSON это объект вида {"1": [2], "4": [1, 3]}.
type Answers map[int][]int

// Has сообщает, выбран ли в вопросе question вариант option.
func (a Answers) Has(question, option int) bool {
	for _, selected := range a[question] {
		if selected == option {
			return true
		}
	}
	return false
}

// RecommendedEnvelope — конверт, который алгоритм подбора предлагает пользователю.
// Priority: 1 — самый высокий приоритет, 5 — самый низкий.
// AllocationPercent — доля пополнения, рассчитанная по весам приоритетов; доли всех
// рекомендованных конвертов в сумме дают ровно 100.
type RecommendedEnvelope struct {
	Name              string   `json:"name"`
	Category          string   `json:"category"`
	Description       string   `json:"description"`
	Priority          int      `json:"priority"`
	AllocationPercent int      `json:"allocation_percent"`
	Reasons           []string `json:"reasons"`
}

// Названия конвертов общего пула (см. questions_and_envelopes.txt).
const (
	envGroceries = "Продукты и товары для дома"
	envHousing   = "Жильё и коммунальные услуги"
	envTransport = "Транспорт"
	envLeisure   = "Кафе, рестораны и развлечения"
	envShopping  = "Одежда и покупки"
	envHealth    = "Здоровье"
	envBeauty    = "Красота и личный уход"
	envTelecom   = "Телефон, интернет и подписки"
	envLearning  = "Обучение, развитие и хобби"
	envTravel    = "Путешествия и отпуск"
	envReserve   = "Резервный фонд"
	envGoals     = "Накопления на цели"
	envDebts     = "Кредиты и долги"
	envFamily    = "Семья и близкие"
	envFree      = "Свободные траты"
)

type envelopeTemplate struct {
	// Category — короткая подпись под названием конверта (в БД до 40 символов).
	Category    string
	Description string
}

var envelopeCatalog = map[string]envelopeTemplate{
	envGroceries: {Category: "Повседневные траты", Description: "Для повседневных покупок продуктов, бытовых и хозяйственных товаров."},
	envHousing:   {Category: "Обязательные платежи", Description: "Для аренды, ипотеки и коммунальных платежей."},
	envTransport: {Category: "Поездки", Description: "Для общественного транспорта, такси, топлива, парковки и других поездок."},
	envLeisure:   {Category: "Досуг", Description: "Для кафе, ресторанов, доставки еды, кино и досуга."},
	envShopping:  {Category: "Покупки", Description: "Для одежды, обуви, маркетплейсов и личных покупок."},
	envHealth:    {Category: "Здоровье", Description: "Для лекарств, медицинских услуг и других расходов на здоровье."},
	envBeauty:    {Category: "Уход за собой", Description: "Для косметики, парикмахерских, косметических услуг и средств ухода."},
	envTelecom:   {Category: "Связь и сервисы", Description: "Для мобильной связи, интернета и регулярных цифровых сервисов."},
	envLearning:  {Category: "Развитие", Description: "Для курсов, книг, спорта, творчества и увлечений."},
	envTravel:    {Category: "Отдых", Description: "Для поездок, билетов, проживания и отдыха."},
	envReserve:   {Category: "Накопления", Description: "Для непредвиденных расходов и финансовой подушки."},
	envGoals:     {Category: "Накопления", Description: "Для заранее выбранных крупных покупок и жизненных целей."},
	envDebts:     {Category: "Обязательные платежи", Description: "Для обязательных платежей по кредитам, займам и рассрочкам."},
	envFamily:    {Category: "Семья", Description: "Для помощи родственникам, расходов на детей и семейных обязательств."},
	envFree:      {Category: "Личные траты", Description: "Для личных расходов, которые пользователь хочет оставить без жёсткой привязки к категории."},
}

// priorityWeights — веса приоритетов при распределении: 1 = 5, 2 = 4, 3 = 3, 4 = 2, 5 = 1.
var priorityWeights = map[int]int64{1: 5, 2: 4, 3: 3, 4: 2, 5: 1}

func weightOf(priority int) int64 {
	if weight := priorityWeights[priority]; weight != 0 {
		return weight
	}
	return 1
}

// overspendRule связывает вариант ответа на вопрос 9 (перерасход) с конвертом,
// приоритет которого он повышает.
type overspendRule struct {
	Option   int
	Envelope string
}

var overspendRules = []overspendRule{
	{1, envGroceries},
	{2, envLeisure},
	{3, envTransport},
	{4, envLeisure},
	{5, envShopping},
	{6, envShopping},
	{7, envBeauty},
	{8, envTravel},
	{9, envTelecom},
}

// BuildRecommendation подбирает конверты по ответам анкеты, назначает приоритеты
// и считает доли распределения. Ответы должны быть заранее проверены ValidateAnswers.
// Результат отсортирован от более важных конвертов к менее важным.
func BuildRecommendation(a Answers) []RecommendedEnvelope {
	var list []*RecommendedEnvelope

	// add добавляет конверт, если его ещё нет; иначе дописывает причину.
	add := func(name string, priority int, reason string) {
		for _, item := range list {
			if item.Name == name {
				if priority > item.Priority {
					item.Priority = priority
				}
				item.Reasons = append(item.Reasons, reason)
				return
			}
		}
		template := envelopeCatalog[name]
		list = append(list, &RecommendedEnvelope{
			Name:        name,
			Category:    template.Category,
			Description: template.Description,
			Priority:    priority,
			Reasons:     []string{reason},
		})
	}

	// Базовые категории, которые обычно нужны для планирования бюджета.
	add(envGroceries, 1, "Базовая категория повседневных расходов")
	add(envFree, 4, "Деньги на личные покупки и спонтанные расходы")

	// Обязательные платежи из вопроса 4.
	if a.Has(4, 1) || a.Has(4, 2) || a.Has(4, 3) {
		add(envHousing, 1, "Есть обязательные расходы на жильё")
	}
	if a.Has(4, 5) || a.Has(4, 7) {
		add(envTelecom, 2, "Есть регулярные платежи за связь или сервисы")
	}
	if a.Has(4, 6) || a.Has(13, 2) || a.Has(13, 3) || a.Has(13, 4) || a.Has(13, 5) {
		add(envDebts, 1, "Есть кредитные или долговые обязательства")
	}
	if a.Has(4, 8) || a.Has(4, 9) || a.Has(6, 1) || a.Has(6, 2) {
		add(envFamily, 2, "Есть семейные расходы или помощь близким")
	}

	// Категории по регулярным тратам из вопроса 7.
	if a.Has(7, 2) {
		add(envLeisure, 3, "Есть расходы на кафе, доставку или досуг")
	}
	if a.Has(7, 3) {
		add(envTransport, 2, "Есть расходы на транспорт")
	}
	if a.Has(7, 5) || a.Has(7, 8) {
		add(envShopping, 3, "Есть расходы на одежду или онлайн-покупки")
	}
	if a.Has(7, 6) {
		add(envBeauty, 4, "Есть расходы на красоту и уход")
	}
	if a.Has(7, 7) {
		add(envHealth, 2, "Есть расходы на здоровье")
	}
	if a.Has(7, 9) {
		add(envTravel, 4, "Есть расходы или интерес к поездкам")
	}
	if a.Has(7, 10) || a.Has(7, 11) {
		add(envLearning, 4, "Есть расходы на обучение, спорт или хобби")
	}

	// Цели накоплений из вопросов 12 и 15.
	if a.Has(12, 1) || a.Has(15, 1) || a.Has(15, 2) || a.Has(15, 3) {
		add(envReserve, 2, "Пользователь хочет или готов формировать резерв")
	}
	if len(a[12]) > 0 && !a.Has(12, 11) {
		add(envGoals, 3, "Выбрана одна или несколько финансовых целей")
	}

	// Ответы о перерасходе повышают приоритет уже существующих категорий.
	// Сам по себе перерасход не создаёт категорию, если она не была выбрана ранее.
	for _, rule := range overspendRules {
		if !a.Has(9, rule.Option) {
			continue
		}
		for _, item := range list {
			if item.Name == rule.Envelope && item.Priority > 1 {
				item.Priority--
				item.Reasons = append(item.Reasons, "В этой категории пользователь отмечает перерасход")
			}
		}
	}

	// Если обязательные расходы занимают большую часть дохода, повышаем важность
	// обязательных категорий, которые уже присутствуют в списке.
	if a.Has(5, 4) || a.Has(5, 5) {
		for _, item := range list {
			if item.Name == envHousing || item.Name == envDebts || item.Name == envGroceries {
				if item.Priority > 1 {
					item.Priority--
				}
			}
		}
	}

	// Стабильная сортировка: сначала более важные категории.
	sort.SliceStable(list, func(i, j int) bool {
		return list[i].Priority < list[j].Priority
	})

	result := make([]RecommendedEnvelope, len(list))
	for i, item := range list {
		result[i] = *item
	}
	assignPercents(result)
	return result
}

// assignPercents переводит веса приоритетов в целые проценты, которые в сумме дают 100.
// Остаток от округления получают конверты с наибольшим дробным остатком
// (при равенстве — более важные, так как список уже отсортирован по приоритету).
func assignPercents(items []RecommendedEnvelope) {
	var totalWeight int64
	for _, item := range items {
		totalWeight += weightOf(item.Priority)
	}
	if totalWeight == 0 {
		return
	}

	remainders := make([]int64, len(items))
	var assigned int64
	for i := range items {
		weight := weightOf(items[i].Priority)
		items[i].AllocationPercent = int(weight * 100 / totalWeight)
		remainders[i] = weight * 100 % totalWeight
		assigned += int64(items[i].AllocationPercent)
	}

	order := make([]int, len(items))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		return remainders[order[i]] > remainders[order[j]]
	})
	for k := 0; int64(k) < 100-assigned && k < len(order); k++ {
		items[order[k]].AllocationPercent++
	}
}
