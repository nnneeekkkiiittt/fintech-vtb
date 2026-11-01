import { useEffect, useState, type ReactNode } from 'react';
import { ArrowLeft, ArrowRight, Check, ClipboardList, Sparkles, X } from 'lucide-react';
import { getSurveyQuestions, runNotificationAction, submitSurvey } from './api';
import type { AppNotification, SurveyAnswers, SurveyQuestion } from './types';

type Props = {
  // Если передано готовое предложение, анкета пропускается и сразу показывается результат.
  initialProposal?: AppNotification;
  onClose: () => void;
  onChanged: () => void;
  onEnvelopesChanged: () => void | Promise<void>;
  onToast: (message: string) => void;
};

export function SurveyDialog({ initialProposal, onClose, onChanged, onEnvelopesChanged, onToast }: Props) {
  const [questions, setQuestions] = useState<SurveyQuestion[]>([]);
  const [loading, setLoading] = useState(!initialProposal);
  const [error, setError] = useState('');
  const [answers, setAnswers] = useState<SurveyAnswers>({});
  const [step, setStep] = useState(0);
  const [busy, setBusy] = useState(false);
  const [proposal, setProposal] = useState<AppNotification | null>(initialProposal ?? null);

  useEffect(() => {
    if (initialProposal) return;
    let cancelled = false;
    getSurveyQuestions()
      .then((list) => {
        if (cancelled) return;
        setQuestions(list);
        setLoading(false);
      })
      .catch((cause: unknown) => {
        if (cancelled) return;
        setError(cause instanceof Error ? cause.message : 'Не удалось загрузить анкету');
        setLoading(false);
      });
    return () => { cancelled = true; };
  }, [initialProposal]);

  useEffect(() => {
    const closeOnEscape = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose(); };
    window.addEventListener('keydown', closeOnEscape);
    return () => window.removeEventListener('keydown', closeOnEscape);
  }, [onClose]);

  const question = questions[step];
  const selected = question ? answers[question.id] ?? [] : [];
  const isLast = step === questions.length - 1;

  function toggle(target: SurveyQuestion, option: number) {
    setAnswers((current) => {
      if (!target.multi) return { ...current, [target.id]: [option] };
      const chosen = current[target.id] ?? [];
      const next = chosen.includes(option) ? chosen.filter((item) => item !== option) : [...chosen, option];
      return { ...current, [target.id]: next };
    });
  }

  async function handleSubmit() {
    setBusy(true);
    setError('');
    try {
      const created = await submitSurvey(answers);
      setProposal(created);
      onChanged();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Не удалось отправить ответы');
    } finally {
      setBusy(false);
    }
  }

  async function handleApply() {
    if (!proposal) return;
    setBusy(true);
    setError('');
    try {
      await runNotificationAction(proposal.id, 'apply');
      await onEnvelopesChanged();
      onChanged();
      onToast('Конверты и доли распределения обновлены');
      onClose();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Не удалось применить изменения');
    } finally {
      setBusy(false);
    }
  }

  async function handleDecline() {
    if (!proposal) return;
    setBusy(true);
    setError('');
    try {
      await runNotificationAction(proposal.id, 'dismiss');
      onChanged();
      onClose();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Не удалось закрыть предложение');
    } finally {
      setBusy(false);
    }
  }

  let content: ReactNode;
  if (proposal) {
    const items = proposal.proposal ?? [];
    content = (
      <>
        <DialogHeader icon={<Sparkles size={21} />} title="Предложенные конверты" description="Проверьте набор конвертов и доли, на которые будет делиться каждое пополнение." onClose={onClose} />
        <div className="survey-body">
          <ul className="survey-proposal-list">
            {items.map((item) => (
              <li key={item.name}>
                <div><strong>{item.name}</strong><span>{item.category}</span></div>
                <span className="survey-percent">{item.percent}%</span>
              </li>
            ))}
          </ul>
          <p className="survey-note">Недостающие конверты будут созданы. Существующие конверты, которых нет в списке, сохранят свои деньги, но перестанут получать новые пополнения (доля 0%).</p>
          {error && <p className="form-warning" role="alert">{error}</p>}
          <div className="modal-actions">
            <button type="button" className="button button-secondary" onClick={() => void handleDecline()} disabled={busy}>Отклонить</button>
            <button type="button" className="button button-primary" onClick={() => void handleApply()} disabled={busy || items.length === 0}>Применить <Check size={16} /></button>
          </div>
        </div>
      </>
    );
  } else if (loading) {
    content = (
      <>
        <DialogHeader icon={<ClipboardList size={21} />} title="Анкета" description="Загружаем вопросы…" onClose={onClose} />
        <div className="survey-body"><div className="survey-loading"><span className="loading-spinner" /></div></div>
      </>
    );
  } else if (!question) {
    content = (
      <>
        <DialogHeader icon={<ClipboardList size={21} />} title="Анкета" description="Не удалось получить вопросы." onClose={onClose} />
        <div className="survey-body">
          {error && <p className="form-warning" role="alert">{error}</p>}
          <div className="modal-actions"><button type="button" className="button button-secondary" onClick={onClose}>Закрыть</button></div>
        </div>
      </>
    );
  } else {
    content = (
      <>
        <DialogHeader icon={<ClipboardList size={21} />} title="Настроим конверты под вас" description="Ответы нужны только для подбора категорий и долей распределения." onClose={onClose} />
        <div className="survey-body">
          <div className="survey-progress" role="progressbar" aria-valuemin={1} aria-valuemax={questions.length} aria-valuenow={step + 1}>
            <span style={{ width: `${((step + 1) / questions.length) * 100}%` }} />
          </div>
          <p className="survey-step">Вопрос {step + 1} из {questions.length}{question.multi ? ' · можно выбрать несколько' : ''}</p>
          <h3 className="survey-question">{question.text}</h3>
          <div className="survey-options" role={question.multi ? 'group' : 'radiogroup'} aria-label={question.text}>
            {question.options.map((option, index) => {
              const number = index + 1;
              const checked = selected.includes(number);
              return (
                <button
                  key={number}
                  type="button"
                  className={`survey-option ${checked ? 'selected' : ''} ${question.multi ? 'multi' : ''}`}
                  role={question.multi ? 'checkbox' : 'radio'}
                  aria-checked={checked}
                  onClick={() => toggle(question, number)}
                >
                  <span className="survey-option-mark">{checked && <Check size={12} strokeWidth={3} />}</span>
                  <span>{option}</span>
                </button>
              );
            })}
          </div>
          {error && <p className="form-warning" role="alert">{error}</p>}
          <div className="modal-actions">
            <button type="button" className="button button-secondary" onClick={() => setStep((value) => value - 1)} disabled={step === 0 || busy}><ArrowLeft size={16} /> Назад</button>
            {isLast ? (
              <button type="button" className="button button-primary" onClick={() => void handleSubmit()} disabled={selected.length === 0 || busy}>Подобрать конверты <ArrowRight size={16} /></button>
            ) : (
              <button type="button" className="button button-primary" onClick={() => setStep((value) => value + 1)} disabled={selected.length === 0}>Далее <ArrowRight size={16} /></button>
            )}
          </div>
        </div>
      </>
    );
  }

  return (
    <div className="modal-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}>
      <section className="modal-card survey-card" role="dialog" aria-modal="true" aria-label="Анкета для настройки конвертов">
        {content}
      </section>
    </div>
  );
}

function DialogHeader({ icon, title, description, onClose }: { icon: ReactNode; title: string; description: string; onClose: () => void }) {
  return (
    <div className="modal-heading">
      <span className="modal-heading-icon">{icon}</span>
      <div><h2>{title}</h2><p>{description}</p></div>
      <button className="icon-button modal-close" onClick={onClose} aria-label="Закрыть"><X size={19} /></button>
    </div>
  );
}
