import { useState, type FormEvent } from 'react';
import { ArrowLeft, ArrowRight, Check, ClipboardList, X } from 'lucide-react';
import { applyRecommendation, createIncome, getRecommendation } from './lib/api';
import { QUESTIONS } from './lib/questionnaire';
import type { Answers, RecommendedEnvelope } from './lib/types';

type QuestionnaireFlowProps = {
  useMock: boolean;
  onClose: () => void;
  /** Вызывается после успешного применения: родитель закрывает окно и обновляет данные. */
  onApplied: (message: string) => void | Promise<void>;
};

function FlowHeader({ title, description, onClose }: { title: string; description: string; onClose: () => void }) {
  return (
    <div className="modal-heading">
      <span className="modal-heading-icon"><ClipboardList size={21} /></span>
      <div><h2>{title}</h2><p>{description}</p></div>
      <button className="icon-button modal-close" onClick={onClose} aria-label="Закрыть"><X size={19} /></button>
    </div>
  );
}

export function QuestionnaireFlow({ useMock, onClose, onApplied }: QuestionnaireFlowProps) {
  const [index, setIndex] = useState(0);
  const [answers, setAnswers] = useState<Answers>({});
  const [result, setResult] = useState<RecommendedEnvelope[] | null>(null);
  const [amount, setAmount] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const question = QUESTIONS[index];
  const selected = answers[question.id] ?? [];
  const isLast = index === QUESTIONS.length - 1;

  const toggle = (option: number) => {
    setAnswers((current) => {
      if (!question.multi) return { ...current, [question.id]: [option] };
      const previous = current[question.id] ?? [];
      const next = previous.includes(option)
        ? previous.filter((item) => item !== option)
        : [...previous, option].sort((a, b) => a - b);
      return { ...current, [question.id]: next };
    });
  };

  async function goNext() {
    setError('');
    if (!isLast) {
      setIndex((value) => value + 1);
      return;
    }
    setBusy(true);
    try {
      setResult(await getRecommendation(answers, useMock));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Не удалось подобрать конверты');
    } finally {
      setBusy(false);
    }
  }

  async function apply(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const rubles = amount.trim() === '' ? 0 : Number(amount.replace(',', '.'));
    if (!Number.isFinite(rubles) || rubles < 0) {
      setError('Введи корректную сумму или оставь поле пустым');
      return;
    }
    setBusy(true);
    setError('');
    try {
      await applyRecommendation(answers, useMock);
      if (rubles > 0) {
        try {
          await createIncome(rubles, 'Первое распределение по анкете', useMock);
        } catch {
          await onApplied('Конверты настроены, но пополнить счёт не удалось. Пополни его вручную.');
          return;
        }
        await onApplied('Конверты настроены, деньги распределены');
        return;
      }
      await onApplied('Конверты настроены по результатам анкеты');
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Не удалось применить рекомендацию');
    } finally {
      setBusy(false);
    }
  }

  if (result) {
    return (
      <>
        <FlowHeader title="Ваши конверты" description="Подобраны по ответам анкеты. Доли можно будет поменять позже." onClose={onClose} />
        <div className="recommendation-list">
          {result.map((item) => (
            <div className="recommendation-item" key={item.name}>
              <div className="recommendation-top">
                <strong>{item.name}</strong>
                <span className="allocation-pill">{item.allocation_percent}% <span>доля</span></span>
              </div>
              <p className="recommendation-description">{item.description}</p>
              <ul className="recommendation-reasons">
                {Array.from(new Set(item.reasons)).map((reason) => <li key={reason}>{reason}</li>)}
              </ul>
            </div>
          ))}
        </div>
        <form className="modal-form" onSubmit={apply}>
          <div className="form-info-note">
            <span>Пустые конверты без операций (например, стартовые) будут заменены. Конверты с остатком или историей останутся, но получат долю 0%.</span>
          </div>
          <label className="form-label">Сразу распределить сумму <span className="optional-label">необязательно</span>
            <span className="amount-input-wrap">
              <input type="number" min="0" step="0.01" value={amount} onChange={(event) => setAmount(event.target.value)} placeholder="0,00" />
              <span>₽</span>
            </span>
          </label>
          {error && <p className="form-warning">{error}</p>}
          <div className="modal-actions">
            <button type="button" className="button button-secondary" onClick={() => { setResult(null); setError(''); }} disabled={busy}><ArrowLeft size={16} /> К анкете</button>
            <button type="submit" className="button button-primary" disabled={busy}>{busy ? 'Применяем…' : 'Создать конверты'} <ArrowRight size={16} /></button>
          </div>
        </form>
      </>
    );
  }

  return (
    <>
      <FlowHeader title="Подбор конвертов" description="Ответь на вопросы, и мы предложим категории и доли распределения." onClose={onClose} />
      <div className="questionnaire-progress" role="progressbar" aria-valuemin={1} aria-valuemax={QUESTIONS.length} aria-valuenow={index + 1}>
        <span style={{ width: `${((index + 1) / QUESTIONS.length) * 100}%` }} />
      </div>
      <div className="questionnaire-step">Вопрос {index + 1} из {QUESTIONS.length}</div>
      <h3 className="questionnaire-question">{question.text}</h3>
      {question.multi && <span className="field-hint">Можно выбрать несколько вариантов</span>}
      <div className="option-list" role={question.multi ? 'group' : 'radiogroup'}>
        {question.options.map((label, position) => {
          const option = position + 1;
          const active = selected.includes(option);
          return (
            <button
              type="button"
              key={option}
              className={`option-item${active ? ' selected' : ''}`}
              role={question.multi ? 'checkbox' : 'radio'}
              aria-checked={active}
              onClick={() => toggle(option)}
            >
              <span className={`option-mark ${question.multi ? 'square' : 'round'}`}>{active && <Check size={12} />}</span>
              <span>{label}</span>
            </button>
          );
        })}
      </div>
      {error && <p className="form-warning">{error}</p>}
      <div className="modal-actions">
        {index === 0
          ? <button type="button" className="button button-secondary" onClick={onClose}>Отмена</button>
          : <button type="button" className="button button-secondary" onClick={() => setIndex((value) => value - 1)} disabled={busy}><ArrowLeft size={16} /> Назад</button>}
        <button type="button" className="button button-primary" onClick={() => void goNext()} disabled={busy || selected.length === 0}>
          {isLast ? (busy ? 'Подбираем…' : 'Подобрать конверты') : 'Далее'} <ArrowRight size={16} />
        </button>
      </div>
    </>
  );
}
