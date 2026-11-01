import type { Answers, RecommendedEnvelope } from './types';

// Копия алгоритма подбора конвертов из envelope-backend/internal/domain/recommendation.go.
// Нужна только демо-режиму (без бэкенда). Если меняешь правила подбора, правь оба файла.

const CATALOG: Record<string, { category: string; description: string }> = {
  'Продукты и товары для дома': { category: 'Повседневные траты', description: 'Для повседневных покупок продуктов, бытовых и хозяйственных товаров.' },
  'Жильё и коммунальные услуги': { category: 'Обязательные платежи', description: 'Для аренды, ипотеки и коммунальных платежей.' },
  'Транспорт': { category: 'Поездки', description: 'Для общественного транспорта, такси, топлива, парковки и других поездок.' },
  'Кафе, рестораны и развлечения': { category: 'Досуг', description: 'Для кафе, ресторанов, доставки еды, кино и досуга.' },
  'Одежда и покупки': { category: 'Покупки', description: 'Для одежды, обуви, маркетплейсов и личных покупок.' },
  'Здоровье': { category: 'Здоровье', description: 'Для лекарств, медицинских услуг и других расходов на здоровье.' },
  'Красота и личный уход': { category: 'Уход за собой', description: 'Для косметики, парикмахерских, косметических услуг и средств ухода.' },
  'Телефон, интернет и подписки': { category: 'Связь и сервисы', description: 'Для мобильной связи, интернета и регулярных цифровых сервисов.' },
  'Обучение, развитие и хобби': { category: 'Развитие', description: 'Для курсов, книг, спорта, творчества и увлечений.' },
  'Путешествия и отпуск': { category: 'Отдых', description: 'Для поездок, билетов, проживания и отдыха.' },
  'Резервный фонд': { category: 'Накопления', description: 'Для непредвиденных расходов и финансовой подушки.' },
  'Накопления на цели': { category: 'Накопления', description: 'Для заранее выбранных крупных покупок и жизненных целей.' },
  'Кредиты и долги': { category: 'Обязательные платежи', description: 'Для обязательных платежей по кредитам, займам и рассрочкам.' },
  'Семья и близкие': { category: 'Семья', description: 'Для помощи родственникам, расходов на детей и семейных обязательств.' },
  'Свободные траты': { category: 'Личные траты', description: 'Для личных расходов, которые пользователь хочет оставить без жёсткой привязки к категории.' },
};

const NAME = {
  groceries: 'Продукты и товары для дома',
  housing: 'Жильё и коммунальные услуги',
  transport: 'Транспорт',
  leisure: 'Кафе, рестораны и развлечения',
  shopping: 'Одежда и покупки',
  health: 'Здоровье',
  beauty: 'Красота и личный уход',
  telecom: 'Телефон, интернет и подписки',
  learning: 'Обучение, развитие и хобби',
  travel: 'Путешествия и отпуск',
  reserve: 'Резервный фонд',
  goals: 'Накопления на цели',
  debts: 'Кредиты и долги',
  family: 'Семья и близкие',
  free: 'Свободные траты',
} as const;

// Вес приоритета при распределении: 1 = 5, 2 = 4, 3 = 3, 4 = 2, 5 = 1.
const WEIGHTS: Record<number, number> = { 1: 5, 2: 4, 3: 3, 4: 2, 5: 1 };
const weightOf = (priority: number) => WEIGHTS[priority] || 1;

// Вариант ответа на вопрос 9 (перерасход) -> конверт, приоритет которого он повышает.
const OVERSPEND_RULES: Array<[number, string]> = [
  [1, NAME.groceries],
  [2, NAME.leisure],
  [3, NAME.transport],
  [4, NAME.leisure],
  [5, NAME.shopping],
  [6, NAME.shopping],
  [7, NAME.beauty],
  [8, NAME.travel],
  [9, NAME.telecom],
];

export function buildRecommendation(answers: Answers): RecommendedEnvelope[] {
  const has = (question: number, option: number) => (answers[question] ?? []).includes(option);
  const list: RecommendedEnvelope[] = [];

  const add = (name: string, priority: number, reason: string) => {
    const existing = list.find((item) => item.name === name);
    if (existing) {
      if (priority > existing.priority) existing.priority = priority;
      existing.reasons.push(reason);
      return;
    }
    const template = CATALOG[name];
    list.push({
      name,
      category: template.category,
      description: template.description,
      priority,
      allocation_percent: 0,
      reasons: [reason],
    });
  };

  // Базовые категории.
  add(NAME.groceries, 1, 'Базовая категория повседневных расходов');
  add(NAME.free, 4, 'Деньги на личные покупки и спонтанные расходы');

  // Обязательные платежи (вопрос 4).
  if (has(4, 1) || has(4, 2) || has(4, 3)) add(NAME.housing, 1, 'Есть обязательные расходы на жильё');
  if (has(4, 5) || has(4, 7)) add(NAME.telecom, 2, 'Есть регулярные платежи за связь или сервисы');
  if (has(4, 6) || has(13, 2) || has(13, 3) || has(13, 4) || has(13, 5)) add(NAME.debts, 1, 'Есть кредитные или долговые обязательства');
  if (has(4, 8) || has(4, 9) || has(6, 1) || has(6, 2)) add(NAME.family, 2, 'Есть семейные расходы или помощь близким');

  // Регулярные траты (вопрос 7).
  if (has(7, 2)) add(NAME.leisure, 3, 'Есть расходы на кафе, доставку или досуг');
  if (has(7, 3)) add(NAME.transport, 2, 'Есть расходы на транспорт');
  if (has(7, 5) || has(7, 8)) add(NAME.shopping, 3, 'Есть расходы на одежду или онлайн-покупки');
  if (has(7, 6)) add(NAME.beauty, 4, 'Есть расходы на красоту и уход');
  if (has(7, 7)) add(NAME.health, 2, 'Есть расходы на здоровье');
  if (has(7, 9)) add(NAME.travel, 4, 'Есть расходы или интерес к поездкам');
  if (has(7, 10) || has(7, 11)) add(NAME.learning, 4, 'Есть расходы на обучение, спорт или хобби');

  // Резерв и цели (вопросы 12 и 15).
  if (has(12, 1) || has(15, 1) || has(15, 2) || has(15, 3)) add(NAME.reserve, 2, 'Пользователь хочет или готов формировать резерв');
  if ((answers[12] ?? []).length > 0 && !has(12, 11)) add(NAME.goals, 3, 'Выбрана одна или несколько финансовых целей');

  // Перерасход повышает приоритет уже существующих категорий, но не создаёт новых.
  for (const [option, envelopeName] of OVERSPEND_RULES) {
    if (!has(9, option)) continue;
    for (const item of list) {
      if (item.name === envelopeName && item.priority > 1) {
        item.priority -= 1;
        item.reasons.push('В этой категории пользователь отмечает перерасход');
      }
    }
  }

  // Большая доля обязательных расходов повышает важность обязательных категорий.
  if (has(5, 4) || has(5, 5)) {
    for (const item of list) {
      if ((item.name === NAME.housing || item.name === NAME.debts || item.name === NAME.groceries) && item.priority > 1) {
        item.priority -= 1;
      }
    }
  }

  // Array.prototype.sort стабилен: сначала более важные категории.
  list.sort((a, b) => a.priority - b.priority);
  assignPercents(list);
  return list;
}

// Переводит веса приоритетов в целые проценты, которые в сумме дают 100
// (метод наибольших остатков; при равенстве выигрывают более важные конверты).
function assignPercents(items: RecommendedEnvelope[]) {
  const totalWeight = items.reduce((sum, item) => sum + weightOf(item.priority), 0);
  if (totalWeight === 0) return;

  const remainders = items.map((item) => (weightOf(item.priority) * 100) % totalWeight);
  items.forEach((item) => { item.allocation_percent = Math.floor((weightOf(item.priority) * 100) / totalWeight); });

  const assigned = items.reduce((sum, item) => sum + item.allocation_percent, 0);
  const order = items.map((_, index) => index).sort((a, b) => remainders[b] - remainders[a]);
  for (let k = 0; k < 100 - assigned && k < order.length; k += 1) {
    items[order[k]].allocation_percent += 1;
  }
}
