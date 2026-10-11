import { useCallback, useEffect, useMemo, useState, type FormEvent, type ReactNode } from 'react';
import {
  ArrowDownToLine,
  ArrowDownLeft,
  ArrowRight,
  ArrowUpFromLine,
  BadgeCheck,
  Car,
  Check,
  ChevronRight,
  CircleHelp,
  Clock3,
  Eye,
  EyeOff,
  FileText,
  Gift,
  History as HistoryIcon,
  House,
  LayoutDashboard,
  Pencil,
  Plus,
  ReceiptText,
  RefreshCw,
  ShoppingBag,
  SlidersHorizontal,
  Sparkles,
  Trash2,
  TrendingUp,
  Utensils,
  Wallet,
  X,
} from 'lucide-react';
import {
  createEnvelope,
  createExpense,
  createIncome,
  deleteEnvelope,
  getEnvelopes,
  getTransactions,
  setAllocations,
  START_IN_DEMO_MODE,
  updateEnvelope,
} from './lib/api';
import type { AllocationUpdate, Envelope, Transaction } from './lib/types';

type Page = 'home' | 'envelopes' | 'history';
type ModalState =
  | { kind: 'income' }
  | { kind: 'expense'; envelopeId?: number }
  | { kind: 'create-envelope' }
  | { kind: 'edit-envelope'; envelope: Envelope };
type TransactionFilter = 'all' | 'income' | 'expense';

const money = (amountRubles: number, digits = 2) => new Intl.NumberFormat('ru-RU', {
  style: 'currency', currency: 'RUB', minimumFractionDigits: 0, maximumFractionDigits: digits,
}).format(amountRubles);
const rubles = (kopecks: number) => kopecks / 100;
const formatDate = (isoDate: string) => new Intl.DateTimeFormat('ru-RU', {
  day: 'numeric', month: 'long', hour: '2-digit', minute: '2-digit',
}).format(new Date(isoDate));
const formatDay = (isoDate: string) => {
  const date = new Date(isoDate);
  const today = new Date();
  const yesterday = new Date();
  yesterday.setDate(today.getDate() - 1);
  if (date.toDateString() === today.toDateString()) return 'Сегодня';
  if (date.toDateString() === yesterday.toDateString()) return 'Вчера';
  return new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'long' }).format(date);
};

function envelopeIcon(envelope: Envelope, size = 22) {
  const name = `${envelope.name} ${envelope.category}`.toLocaleLowerCase('ru-RU');
  if (/ресторан|кафе|еда|продукт|кухн/.test(name)) return <Utensils size={size} strokeWidth={1.8} />;
  if (/такси|транспорт|машин|авто|бензин/.test(name)) return <Car size={size} strokeWidth={1.8} />;
  if (/дом|квартир|ремонт|жиль/.test(name)) return <House size={size} strokeWidth={1.8} />;
  if (/подар|празд/.test(name)) return <Gift size={size} strokeWidth={1.8} />;
  if (/путеше|отпуск|билет/.test(name)) return <Sparkles size={size} strokeWidth={1.8} />;
  if (/покуп|магаз/.test(name)) return <ShoppingBag size={size} strokeWidth={1.8} />;
  return <Wallet size={size} strokeWidth={1.8} />;
}

function App() {
  const [page, setPage] = useState<Page>('home');
  const [useMock, setUseMock] = useState(START_IN_DEMO_MODE);
  const [envelopes, setEnvelopes] = useState<Envelope[]>([]);
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [loading, setLoading] = useState(true);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [error, setError] = useState('');
  const [toast, setToast] = useState('');
  const [modal, setModal] = useState<ModalState | null>(null);
  const [balanceVisible, setBalanceVisible] = useState(true);
  const [allocationsOpen, setAllocationsOpen] = useState(false);
  const [allocationValues, setAllocationValues] = useState<Record<number, string>>({});
  const [transactionFilter, setTransactionFilter] = useState<TransactionFilter>('all');
  const [searchText, setSearchText] = useState('');
  const [historyHasMore, setHistoryHasMore] = useState(true);

  const loadData = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const [nextEnvelopes, transactionPage] = await Promise.all([
        getEnvelopes(useMock),
        getTransactions(20, 0, useMock),
      ]);
      setEnvelopes(nextEnvelopes);
      setTransactions(transactionPage.data);
      setHistoryHasMore(transactionPage.data.length === transactionPage.pagination.limit);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Не удалось загрузить данные');
    } finally {
      setLoading(false);
    }
  }, [useMock]);

  useEffect(() => { void loadData(); }, [loadData]);

  useEffect(() => {
    if (!toast) return;
    const timeout = window.setTimeout(() => setToast(''), 3200);
    return () => window.clearTimeout(timeout);
  }, [toast]);

  useEffect(() => {
    setAllocationValues(Object.fromEntries(envelopes.map((item) => [item.id, String(item.allocation_percent)])));
  }, [envelopes]);

  const totalKopecks = useMemo(() => envelopes.reduce((sum, item) => sum + item.balance_kopecks, 0), [envelopes]);
  const percentSum = useMemo(() => envelopes.reduce((sum, item) => sum + (Number(allocationValues[item.id]) || 0), 0), [envelopes, allocationValues]);
  const filteredTransactions = useMemo(() => {
    const query = searchText.trim().toLocaleLowerCase('ru-RU');
    return transactions.filter((tx) => {
      const typeMatches = transactionFilter === 'all' || tx.type === transactionFilter;
      const searchMatches = !query || `${tx.description} ${tx.allocations.map((item) => item.envelope_name).join(' ')}`.toLocaleLowerCase('ru-RU').includes(query);
      return typeMatches && searchMatches;
    });
  }, [transactions, transactionFilter, searchText]);

  const showToast = (message: string) => setToast(message);

  async function reloadAndNotify(message: string) {
    await loadData();
    showToast(message);
  }

  async function handleEnvelopeSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!modal || (modal.kind !== 'create-envelope' && modal.kind !== 'edit-envelope')) return;
    const form = new FormData(event.currentTarget);
    const name = String(form.get('name') ?? '').trim();
    const category = String(form.get('category') ?? '').trim();
    if (!name || !category) return;
    try {
      if (modal.kind === 'create-envelope') {
        await createEnvelope(name, category, useMock);
        setModal(null);
        await reloadAndNotify('Конверт создан');
      } else {
        await updateEnvelope(modal.envelope.id, name, category, useMock);
        setModal(null);
        await reloadAndNotify('Изменения сохранены');
      }
    } catch (cause) {
      showToast(cause instanceof Error ? cause.message : 'Не удалось сохранить конверт');
    }
  }

  async function handleTransactionSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!modal || (modal.kind !== 'income' && modal.kind !== 'expense')) return;
    const form = new FormData(event.currentTarget);
    const amountValue = String(form.get('amount') ?? '').replace(',', '.');
    const amount = Number(amountValue);
    const description = String(form.get('description') ?? '').trim();
    if (!Number.isFinite(amount) || amount <= 0) {
      showToast('Введи корректную сумму');
      return;
    }
    try {
      if (modal.kind === 'income') {
        await createIncome(amount, description, useMock);
        setModal(null);
        await reloadAndNotify('Счёт пополнен, деньги распределены по конвертам');
      } else {
        const envelopeId = Number(form.get('envelope_id'));
        await createExpense(envelopeId, amount, description, useMock);
        setModal(null);
        await reloadAndNotify('Расход записан');
      }
    } catch (cause) {
      showToast(cause instanceof Error ? cause.message : 'Не удалось провести операцию');
    }
  }

  async function handleDeleteEnvelope(envelope: Envelope) {
    const confirmed = window.confirm(`Удалить конверт «${envelope.name}»? Бэкенд разрешает удалить только пустой конверт без истории и с долей 0%.`);
    if (!confirmed) return;
    try {
      await deleteEnvelope(envelope.id, useMock);
      await reloadAndNotify('Конверт удалён');
    } catch (cause) {
      showToast(cause instanceof Error ? cause.message : 'Не удалось удалить конверт');
    }
  }

  async function handleAllocationSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (percentSum !== 100) {
      showToast('Сумма долей должна составлять ровно 100%');
      return;
    }
    const updates: AllocationUpdate[] = envelopes.map((item) => ({ envelope_id: item.id, percent: Number(allocationValues[item.id] || 0) }));
    try {
      await setAllocations(updates, useMock);
      await reloadAndNotify('Доли распределения сохранены');
      setAllocationsOpen(false);
    } catch (cause) {
      showToast(cause instanceof Error ? cause.message : 'Не удалось сохранить доли');
    }
  }

  async function loadMoreTransactions() {
    setHistoryLoading(true);
    try {
      const result = await getTransactions(20, transactions.length, useMock);
      setTransactions((current) => [...current, ...result.data]);
      setHistoryHasMore(result.data.length === result.pagination.limit);
    } catch (cause) {
      showToast(cause instanceof Error ? cause.message : 'Не удалось загрузить историю');
    } finally {
      setHistoryLoading(false);
    }
  }

  const pageTitle = page === 'home' ? 'Главная' : page === 'envelopes' ? 'Мои конверты' : 'История операций';
  const switchMode = () => {
    setUseMock((current) => !current);
    setError('');
    setTransactions([]);
    setEnvelopes([]);
    setPage('home');
    showToast(useMock ? 'Подключаем реальный API' : 'Открыт демо-режим');
  };

  return (
    <div className="app-shell">
      <header className="topbar">
        <div className="topbar-inner">
          <button className="brand" onClick={() => setPage('home')} aria-label="ВТБ — на главную">
            <span className="brand-symbol"><span /><span /><span /></span>
            <span className="brand-name">ВТБ</span>
          </button>
          <nav className="desktop-nav" aria-label="Основная навигация">
            <NavButton active={page === 'home'} onClick={() => setPage('home')} icon={<LayoutDashboard size={18} />} label="Главная" />
            <NavButton active={page === 'envelopes'} onClick={() => setPage('envelopes')} icon={<Wallet size={18} />} label="Конверты" />
            <NavButton active={page === 'history'} onClick={() => setPage('history')} icon={<HistoryIcon size={18} />} label="История" />
          </nav>
          <div className="topbar-tools">
            <button className={`mode-pill ${useMock ? 'mode-demo' : 'mode-live'}`} onClick={switchMode} title="Переключить источник данных">
              <span className="mode-dot" />
              {useMock ? 'Демо-режим' : 'Реальный API'}
              <RefreshCw size={13} />
            </button>
            <button className="profile-button" aria-label="Профиль"><span>Н</span></button>
          </div>
        </div>
      </header>

      <main className="main-content">
        <div className="page-heading">
          <div>
            <p className="eyebrow">ВТБ · ФИНАНСЫ</p>
            <h1>{pageTitle}</h1>
          </div>
          {page === 'envelopes' && <button className="button button-primary heading-action" onClick={() => setModal({ kind: 'create-envelope' })}><Plus size={18} /> Новый конверт</button>}
          {page === 'history' && <span className="history-count">{transactions.length} операций загружено</span>}
        </div>

        {error && (
          <div className="error-banner" role="alert">
            <div className="error-icon"><CircleHelp size={20} /></div>
            <div className="error-copy"><strong>Не удалось загрузить данные</strong><span>{error}</span></div>
            <button className="button button-secondary" onClick={() => void loadData()}><RefreshCw size={15} /> Повторить</button>
            {!useMock && <button className="button button-link" onClick={switchMode}>Открыть демо</button>}
          </div>
        )}

        {loading && !envelopes.length && !error ? <LoadingState /> : (
          <>
            {page === 'home' && (
              <HomePage
                envelopes={envelopes}
                transactions={transactions.slice(0, 4)}
                totalKopecks={totalKopecks}
                balanceVisible={balanceVisible}
                onToggleBalance={() => setBalanceVisible((value) => !value)}
                onIncome={() => setModal({ kind: 'income' })}
                onExpense={() => setModal({ kind: 'expense' })}
                onCreate={() => setModal({ kind: 'create-envelope' })}
                onAllocations={() => { setPage('envelopes'); setAllocationsOpen(true); }}
                onAllEnvelopes={() => setPage('envelopes')}
                onAllTransactions={() => setPage('history')}
                onSelectEnvelope={(envelope) => setModal({ kind: 'expense', envelopeId: envelope.id })}
              />
            )}
            {page === 'envelopes' && (
              <EnvelopesPage
                envelopes={envelopes}
                allocationValues={allocationValues}
                percentSum={percentSum}
                allocationsOpen={allocationsOpen}
                onToggleAllocations={() => setAllocationsOpen((open) => !open)}
                onAllocationChange={(id, value) => setAllocationValues((current) => ({ ...current, [id]: value }))}
                onAllocationSubmit={handleAllocationSubmit}
                onCreate={() => setModal({ kind: 'create-envelope' })}
                onEdit={(envelope) => setModal({ kind: 'edit-envelope', envelope })}
                onDelete={(envelope) => void handleDeleteEnvelope(envelope)}
                onSpend={(envelope) => setModal({ kind: 'expense', envelopeId: envelope.id })}
                onIncome={() => setModal({ kind: 'income' })}
              />
            )}
            {page === 'history' && (
              <HistoryPage
                transactions={filteredTransactions}
                filter={transactionFilter}
                onFilterChange={setTransactionFilter}
                searchText={searchText}
                onSearchChange={setSearchText}
                hasMore={historyHasMore && transactionFilter === 'all' && !searchText}
                loadingMore={historyLoading}
                onLoadMore={() => void loadMoreTransactions()}
              />
            )}
          </>
        )}
      </main>

      <BottomNavigation page={page} onNavigate={setPage} />
      <footer className="app-footer"><ShieldLine />Учебный прототип интерфейса. Не является официальным приложением ВТБ.</footer>

      {modal && (
        <Modal onClose={() => setModal(null)}>
          {modal.kind === 'income' && (
            <TransactionForm
              kind="income"
              envelopes={envelopes}
              onClose={() => setModal(null)}
              onSubmit={handleTransactionSubmit}
            />
          )}
          {modal.kind === 'expense' && (
            <TransactionForm
              kind="expense"
              envelopes={envelopes}
              initialEnvelopeId={modal.envelopeId}
              onClose={() => setModal(null)}
              onSubmit={handleTransactionSubmit}
            />
          )}
          {(modal.kind === 'create-envelope' || modal.kind === 'edit-envelope') && (
            <EnvelopeForm
              editing={modal.kind === 'edit-envelope' ? modal.envelope : undefined}
              onClose={() => setModal(null)}
              onSubmit={handleEnvelopeSubmit}
            />
          )}
        </Modal>
      )}

      {toast && <div className="toast-message" role="status"><span className="toast-check"><Check size={15} /></span>{toast}</div>}
    </div>
  );
}

function NavButton({ active, onClick, icon, label }: { active: boolean; onClick: () => void; icon: ReactNode; label: string }) {
  return <button className={`nav-button ${active ? 'active' : ''}`} onClick={onClick}>{icon}<span>{label}</span></button>;
}

function BottomNavigation({ page, onNavigate }: { page: Page; onNavigate: (page: Page) => void }) {
  return (
    <nav className="bottom-nav" aria-label="Нижняя навигация">
      <NavButton active={page === 'home'} onClick={() => onNavigate('home')} icon={<LayoutDashboard size={21} />} label="Главная" />
      <NavButton active={page === 'envelopes'} onClick={() => onNavigate('envelopes')} icon={<Wallet size={21} />} label="Конверты" />
      <NavButton active={page === 'history'} onClick={() => onNavigate('history')} icon={<HistoryIcon size={21} />} label="История" />
    </nav>
  );
}

function HomePage({
  envelopes, transactions, totalKopecks, balanceVisible, onToggleBalance, onIncome, onExpense, onCreate, onAllocations, onAllEnvelopes, onAllTransactions, onSelectEnvelope,
}: {
  envelopes: Envelope[];
  transactions: Transaction[];
  totalKopecks: number;
  balanceVisible: boolean;
  onToggleBalance: () => void;
  onIncome: () => void;
  onExpense: () => void;
  onCreate: () => void;
  onAllocations: () => void;
  onAllEnvelopes: () => void;
  onAllTransactions: () => void;
  onSelectEnvelope: (envelope: Envelope) => void;
}) {
  const percentSum = envelopes.reduce((sum, item) => sum + item.allocation_percent, 0);
  return (
    <div className="home-layout">
      <section className="balance-card">
        <div className="balance-card-orb balance-orb-one" /><div className="balance-card-orb balance-orb-two" />
        <div className="balance-card-top"><span className="balance-label"><span className="balance-label-dot" />ОБЩИЙ ОСТАТОК В КОНВЕРТАХ</span><button onClick={onToggleBalance} className="balance-visibility" aria-label={balanceVisible ? 'Скрыть баланс' : 'Показать баланс'}>{balanceVisible ? <Eye size={19} /> : <EyeOff size={19} />}</button></div>
        <div className="balance-amount">{balanceVisible ? money(rubles(totalKopecks)) : '•••••• ₽'}</div>
        <div className="balance-card-bottom"><div><span className="balance-bottom-label">Деньги распределены</span><span className="balance-bottom-value">{envelopes.length} {plural(envelopes.length, 'конверт', 'конверта', 'конвертов')}</span></div><div className="balance-card-mark"><span className="brand-symbol light"><span /><span /><span /></span>ВТБ</div></div>
      </section>

      <section className="quick-actions" aria-label="Быстрые действия">
        <QuickAction icon={<ArrowDownToLine size={22} />} label="Пополнить" tone="blue" onClick={onIncome} />
        <QuickAction icon={<ArrowUpFromLine size={22} />} label="Потратить" tone="purple" onClick={onExpense} />
        <QuickAction icon={<Plus size={23} />} label="Новый конверт" tone="mint" onClick={onCreate} />
        <QuickAction icon={<SlidersHorizontal size={22} />} label="Распределение" tone="orange" onClick={onAllocations} />
      </section>

      <section className="section-block">
        <div className="section-heading"><div><h2>Мои конверты</h2><p>Каждому рублю — своё назначение</p></div><button className="text-link" onClick={onAllEnvelopes}>Все конверты <ChevronRight size={16} /></button></div>
        {envelopes.length === 0 ? <EmptyState title="Пока нет конвертов" description="Создай первый конверт, чтобы распределять деньги по категориям." action="Создать конверт" onAction={onCreate} /> : (
          <div className="envelope-grid home-envelope-grid">
            {envelopes.slice(0, 3).map((envelope, index) => <EnvelopeCard key={envelope.id} envelope={envelope} index={index} onClick={() => onSelectEnvelope(envelope)} />)}
            {envelopes.length > 3 && <button className="more-envelope-card" onClick={onAllEnvelopes}><span>+{envelopes.length - 3}</span><strong>Другие конверты</strong><ChevronRight size={17} /></button>}
          </div>
        )}
      </section>

      <div className="home-bottom-grid">
        <section className="panel distribution-panel">
          <div className="panel-heading"><div className="panel-icon panel-icon-blue"><TrendingUp size={19} /></div><div><h3>Автораспределение</h3><p>Доли от каждого пополнения</p></div></div>
          <div className="distribution-bar" aria-label={`Распределение: ${percentSum}%`}>
            {envelopes.map((envelope, index) => <span key={envelope.id} className={`distribution-segment segment-${index % 4}`} style={{ width: `${Math.max(0, envelope.allocation_percent)}%` }} />)}
          </div>
          <div className="distribution-legend">{envelopes.slice(0, 3).map((envelope, index) => <div className="legend-item" key={envelope.id}><span className={`legend-dot segment-${index % 4}`} /><span>{envelope.name}</span><strong>{envelope.allocation_percent}%</strong></div>)}</div>
          {percentSum !== 100 && <div className="inline-warning">Сейчас распределение составляет {percentSum}%. Для пополнения сумма долей должна быть 100%.</div>}
          <button className="button button-light full-width" onClick={onAllocations}>Настроить доли <ArrowRight size={16} /></button>
        </section>

        <section className="panel transactions-panel">
          <div className="section-heading compact"><div><h2>Последние операции</h2><p>Движение по конвертам</p></div><button className="icon-button subtle" onClick={onAllTransactions} aria-label="Вся история"><ChevronRight size={19} /></button></div>
          {transactions.length === 0 ? <EmptyMini text="Операции появятся здесь после пополнения или расхода." /> : <div className="transaction-list compact-list">{transactions.slice(0, 3).map((transaction) => <TransactionRow key={transaction.id} transaction={transaction} />)}</div>}
          <button className="text-link history-link" onClick={onAllTransactions}>Вся история <ChevronRight size={16} /></button>
        </section>
      </div>

      <section className="security-note"><div className="security-icon"><BadgeCheck size={20} /></div><div><strong>Удобный контроль расходов</strong><p>Пополнение распределяется автоматически, а траты списываются только из выбранного конверта.</p></div></section>
    </div>
  );
}

function QuickAction({ icon, label, tone, onClick }: { icon: ReactNode; label: string; tone: string; onClick: () => void }) {
  return <button className="quick-action" onClick={onClick}><span className={`quick-action-icon tone-${tone}`}>{icon}</span><span>{label}</span></button>;
}

function EnvelopeCard({ envelope, index, onClick }: { envelope: Envelope; index: number; onClick: () => void }) {
  const color = `envelope-color-${index % 4}`;
  return (
    <button className="envelope-card" onClick={onClick}>
      <div className="envelope-card-top"><span className={`envelope-icon-box ${color}`}>{envelopeIcon(envelope)}</span><span className="allocation-pill">{envelope.allocation_percent}% <span>доля</span></span></div>
      <span className="envelope-name">{envelope.name}</span><span className="envelope-category">{envelope.category}</span>
      <strong className="envelope-amount">{money(rubles(envelope.balance_kopecks))}</strong>
      <div className="envelope-card-foot"><span>Баланс конверта</span><ChevronRight size={16} /></div>
    </button>
  );
}

function EnvelopesPage({ envelopes, allocationValues, percentSum, allocationsOpen, onToggleAllocations, onAllocationChange, onAllocationSubmit, onCreate, onEdit, onDelete, onSpend, onIncome }: {
  envelopes: Envelope[];
  allocationValues: Record<number, string>;
  percentSum: number;
  allocationsOpen: boolean;
  onToggleAllocations: () => void;
  onAllocationChange: (id: number, value: string) => void;
  onAllocationSubmit: (event: FormEvent<HTMLFormElement>) => void;
  onCreate: () => void;
  onEdit: (envelope: Envelope) => void;
  onDelete: (envelope: Envelope) => void;
  onSpend: (envelope: Envelope) => void;
  onIncome: () => void;
}) {
  const total = envelopes.reduce((sum, envelope) => sum + envelope.balance_kopecks, 0);
  return (
    <div className="page-stack">
      <section className="envelopes-summary"><div className="summary-icon"><Wallet size={23} /></div><div><span>Всего в конвертах</span><strong>{money(rubles(total))}</strong><small>{envelopes.length} категорий накоплений</small></div><div className="summary-decoration"><span /><span /><span /></div></section>
      {envelopes.length === 0 ? <EmptyState title="Создай первый конверт" description="Например, на продукты, такси или отпуск. Доли распределения можно изменить позже." action="Создать конверт" onAction={onCreate} /> : (
        <section className="section-block">
          <div className="section-heading"><div><h2>Все конверты</h2><p>Нажми на действие, чтобы быстро записать расход</p></div><button className="button button-secondary" onClick={onCreate}><Plus size={17} /> Добавить</button></div>
          <div className="envelope-manage-list">
            {envelopes.map((envelope, index) => (
              <div className="envelope-manage-row" key={envelope.id}>
                <div className={`envelope-icon-box envelope-color-${index % 4}`}>{envelopeIcon(envelope, 21)}</div>
                <div className="manage-envelope-main"><strong>{envelope.name}</strong><span>{envelope.category} <i /> {envelope.allocation_percent}% от пополнения</span><div className="mini-progress"><span style={{ width: `${envelope.allocation_percent}%` }} /></div></div>
                <div className="manage-envelope-balance"><strong>{money(rubles(envelope.balance_kopecks))}</strong><span>остаток</span></div>
                <div className="manage-actions"><button className="icon-button" onClick={() => onSpend(envelope)} aria-label={`Потратить из конверта ${envelope.name}`} title="Записать расход"><ArrowUpFromLine size={17} /></button><button className="icon-button" onClick={() => onEdit(envelope)} aria-label={`Изменить ${envelope.name}`} title="Редактировать"><Pencil size={16} /></button><button className="icon-button danger-hover" onClick={() => onDelete(envelope)} aria-label={`Удалить ${envelope.name}`} title="Удалить"><Trash2 size={16} /></button></div>
              </div>
            ))}
          </div>
        </section>
      )}

      <section className="panel allocation-panel">
        <button className="allocation-panel-trigger" onClick={onToggleAllocations} aria-expanded={allocationsOpen}>
          <span className="panel-icon panel-icon-blue"><SlidersHorizontal size={20} /></span><span className="trigger-copy"><strong>Настроить распределение</strong><small>Как делить каждое новое пополнение</small></span><span className={`percent-total ${percentSum === 100 ? 'valid' : 'invalid'}`}>{percentSum}%</span><ChevronRight className={allocationsOpen ? 'chevron-rotated' : ''} size={19} />
        </button>
        {allocationsOpen && (
          <form className="allocation-editor" onSubmit={onAllocationSubmit}>
            <div className="allocation-editor-intro">Укажи долю для каждого конверта. Сумма процентов должна быть ровно 100%.</div>
            {envelopes.map((envelope, index) => (
              <label className="allocation-input-row" key={envelope.id}>
                <span className={`tiny-envelope-icon envelope-color-${index % 4}`}>{envelopeIcon(envelope, 17)}</span><span className="allocation-input-name">{envelope.name}</span>
                <span className="percentage-input-wrap"><input type="number" min="0" max="100" step="1" value={allocationValues[envelope.id] ?? '0'} onChange={(event) => onAllocationChange(envelope.id, event.target.value)} aria-label={`Процент для ${envelope.name}`} /><span>%</span></span>
              </label>
            ))}
            <div className={`allocation-total-row ${percentSum === 100 ? 'valid' : 'invalid'}`}><span>{percentSum === 100 ? <Check size={16} /> : <CircleHelp size={16} />}{percentSum === 100 ? 'Распределение готово' : 'Осталось распределить'}</span><strong>{percentSum === 100 ? '100%' : `${100 - percentSum}%`}</strong></div>
            <button type="submit" className="button button-primary full-width" disabled={percentSum !== 100 || envelopes.length === 0}>Сохранить доли</button>
          </form>
        )}
      </section>
      <div className="callout-note"><CircleHelp size={18} /><span>Конверт с ненулевым остатком или историей операций удалить нельзя. Чтобы удалить пустой конверт без истории, сначала установи его долю в 0%.</span></div>
      <div className="envelopes-bottom-actions"><button className="button button-light" onClick={onIncome}><ArrowDownToLine size={17} /> Пополнить общий счёт</button></div>
    </div>
  );
}

function HistoryPage({ transactions, filter, onFilterChange, searchText, onSearchChange, hasMore, loadingMore, onLoadMore }: {
  transactions: Transaction[];
  filter: TransactionFilter;
  onFilterChange: (filter: TransactionFilter) => void;
  searchText: string;
  onSearchChange: (value: string) => void;
  hasMore: boolean;
  loadingMore: boolean;
  onLoadMore: () => void;
}) {
  const groups = transactions.reduce<Record<string, Transaction[]>>((acc, item) => {
    const key = formatDay(item.created_at);
    (acc[key] ??= []).push(item);
    return acc;
  }, {});
  return (
    <div className="page-stack">
      <section className="history-summary-row"><div className="history-summary-icon"><ReceiptText size={23} /></div><div><strong>Операции по конвертам</strong><span>Все пополнения и расходы в одном месте</span></div><span className="history-summary-badge"><HistoryIcon size={15} /> Журнал</span></section>
      <section className="panel history-panel">
        <div className="history-search"><SearchIcon /><input value={searchText} onChange={(event) => onSearchChange(event.target.value)} placeholder="Поиск по операции или конверту" aria-label="Поиск по истории" />{searchText && <button onClick={() => onSearchChange('')} aria-label="Очистить поиск"><X size={16} /></button>}</div>
        <div className="filter-pills"><FilterPill active={filter === 'all'} onClick={() => onFilterChange('all')}>Все операции</FilterPill><FilterPill active={filter === 'income'} onClick={() => onFilterChange('income')}>Пополнения</FilterPill><FilterPill active={filter === 'expense'} onClick={() => onFilterChange('expense')}>Расходы</FilterPill></div>
        {transactions.length === 0 ? <div className="history-empty"><span className="empty-icon"><FileText size={24} /></span><strong>Операций пока нет</strong><p>Пополнения и расходы появятся здесь после первой операции.</p></div> : (
          <div className="history-groups">
            {Object.entries(groups).map(([day, items]) => <section className="history-day-group" key={day}><h3>{day}</h3><div className="transaction-list">{items.map((item) => <TransactionRow key={item.id} transaction={item} detailed />)}</div></section>)}
          </div>
        )}
        {hasMore && <button className="button button-light load-more" onClick={onLoadMore} disabled={loadingMore}>{loadingMore ? <RefreshCw className="spin" size={16} /> : <ChevronRight size={17} className="chevron-down" />}{loadingMore ? 'Загружаем…' : 'Загрузить ещё'}</button>}
      </section>
      <p className="history-footnote"><Clock3 size={15} /> История операций хранится в журнале и не редактируется.</p>
    </div>
  );
}

function FilterPill({ active, onClick, children }: { active: boolean; onClick: () => void; children: ReactNode }) {
  return <button className={`filter-pill ${active ? 'active' : ''}`} onClick={onClick}>{children}</button>;
}

function SearchIcon() { return <span className="search-icon" aria-hidden="true"><span /></span>; }

function TransactionRow({ transaction, detailed = false }: { transaction: Transaction; detailed?: boolean }) {
  const isIncome = transaction.type === 'income';
  const allocationNames = [...new Set(transaction.allocations.map((item) => item.envelope_name))];
  const title = transaction.description?.trim() || (isIncome ? 'Пополнение счёта' : 'Расход из конверта');
  return (
    <div className={`transaction-row ${detailed ? 'transaction-row-detailed' : ''}`}>
      <span className={`transaction-icon ${isIncome ? 'income-icon' : 'expense-icon'}`}>{isIncome ? <ArrowDownLeft size={19} /> : <ArrowUpFromLine size={18} />}</span>
      <div className="transaction-main"><strong>{title}</strong><span>{allocationNames.length ? allocationNames.join(', ') : (isIncome ? 'Пополнение' : 'Расход')}{detailed ? ` · ${new Intl.DateTimeFormat('ru-RU', { hour: '2-digit', minute: '2-digit' }).format(new Date(transaction.created_at))}` : ''}</span></div>
      <div className="transaction-amount-block"><strong className={isIncome ? 'amount-positive' : 'amount-negative'}>{isIncome ? '+' : '−'}{money(rubles(transaction.amount_kopecks))}</strong>{detailed && <span>{formatDate(transaction.created_at)}</span>}</div>
    </div>
  );
}

function TransactionForm({ kind, envelopes, initialEnvelopeId, onClose, onSubmit }: {
  kind: 'income' | 'expense';
  envelopes: Envelope[];
  initialEnvelopeId?: number;
  onClose: () => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
}) {
  const [amount, setAmount] = useState('');
  const selectedInitial = initialEnvelopeId ?? envelopes[0]?.id ?? 0;
  const [selectedEnvelope, setSelectedEnvelope] = useState(selectedInitial);
  const selected = envelopes.find((item) => item.id === selectedEnvelope);
  const isIncome = kind === 'income';
  const totalPercent = envelopes.reduce((sum, item) => sum + item.allocation_percent, 0);
  const disabled = envelopes.length === 0 || (!isIncome && !selected);
  return (
    <>
      <ModalHeader icon={isIncome ? <ArrowDownToLine size={21} /> : <ArrowUpFromLine size={21} />} title={isIncome ? 'Пополнить счёт' : 'Записать расход'} description={isIncome ? 'Сумма автоматически распределится по конвертам.' : 'Расход будет списан из выбранного конверта.'} onClose={onClose} />
      <form className="modal-form" onSubmit={onSubmit}>
        <label className="form-label">Сумма, ₽<span className="amount-input-wrap"><input name="amount" type="number" min="0.01" step="0.01" required autoFocus value={amount} onChange={(event) => setAmount(event.target.value)} placeholder="0,00" /><span>₽</span></span></label>
        {!isIncome && <label className="form-label">Списать из конверта<select name="envelope_id" value={selectedEnvelope} onChange={(event) => setSelectedEnvelope(Number(event.target.value))} required>{envelopes.map((item) => <option key={item.id} value={item.id}>{item.name} — {money(rubles(item.balance_kopecks))}</option>)}</select>{selected && <span className="field-hint">Доступно: {money(rubles(selected.balance_kopecks))}</span>}</label>}
        {isIncome && <div className="income-preview"><div className="income-preview-heading"><span>Как распределится пополнение</span><span className={totalPercent === 100 ? 'preview-valid' : 'preview-invalid'}>{totalPercent}%</span></div>{envelopes.map((item, index) => <div className="income-preview-row" key={item.id}><span className={`legend-dot segment-${index % 4}`} /><span>{item.name}</span><strong>{item.allocation_percent}%</strong></div>)}{totalPercent !== 100 && <p className="form-warning">Сумма долей должна составлять 100%. Сначала настрой распределение.</p>}</div>}
        <label className="form-label">Комментарий <span className="optional-label">необязательно</span><input name="description" type="text" maxLength={200} placeholder={isIncome ? 'Например, зарплата' : 'Например, обед'} /></label>
        <div className="modal-actions"><button type="button" className="button button-secondary" onClick={onClose}>Отмена</button><button type="submit" className="button button-primary" disabled={disabled || (isIncome && totalPercent !== 100)}>{isIncome ? 'Пополнить' : 'Записать расход'} <ArrowRight size={16} /></button></div>
      </form>
    </>
  );
}

function EnvelopeForm({ editing, onClose, onSubmit }: {
  editing?: Envelope;
  onClose: () => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
}) {
  return (
    <>
      <ModalHeader icon={editing ? <Pencil size={20} /> : <Plus size={21} />} title={editing ? 'Изменить конверт' : 'Новый конверт'} description={editing ? 'Обнови название или категорию конверта.' : 'Создай категорию для контроля расходов.'} onClose={onClose} />
      <form className="modal-form" onSubmit={onSubmit}>
        <label className="form-label">Название конверта<input name="name" type="text" required maxLength={80} defaultValue={editing?.name ?? ''} placeholder="Например, Путешествия" autoFocus /></label>
        <label className="form-label">Категория<input name="category" type="text" required maxLength={40} defaultValue={editing?.category ?? ''} placeholder="Например, Отдых" /></label>
        {!editing && <div className="form-info-note"><Sparkles size={17} /><span>Новый конверт создаётся с долей 0%. После создания не забудь обновить распределение, если хочешь направлять в него часть пополнений.</span></div>}
        <div className="modal-actions"><button type="button" className="button button-secondary" onClick={onClose}>Отмена</button><button type="submit" className="button button-primary">{editing ? 'Сохранить' : 'Создать конверт'} <ArrowRight size={16} /></button></div>
      </form>
    </>
  );
}

function ModalHeader({ icon, title, description, onClose }: { icon: ReactNode; title: string; description: string; onClose: () => void }) {
  return <div className="modal-heading"><span className="modal-heading-icon">{icon}</span><div><h2>{title}</h2><p>{description}</p></div><button className="icon-button modal-close" onClick={onClose} aria-label="Закрыть"><X size={19} /></button></div>;
}

function Modal({ children, onClose }: { children: ReactNode; onClose: () => void }) {
  useEffect(() => {
    const closeOnEscape = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose(); };
    window.addEventListener('keydown', closeOnEscape);
    return () => window.removeEventListener('keydown', closeOnEscape);
  }, [onClose]);
  return <div className="modal-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}><section className="modal-card" role="dialog" aria-modal="true">{children}</section></div>;
}

function EmptyState({ title, description, action, onAction }: { title: string; description: string; action: string; onAction: () => void }) {
  return <div className="empty-state"><span className="empty-icon"><Wallet size={26} /></span><strong>{title}</strong><p>{description}</p><button className="button button-primary" onClick={onAction}><Plus size={16} /> {action}</button></div>;
}
function EmptyMini({ text }: { text: string }) { return <div className="empty-mini"><FileText size={19} /><span>{text}</span></div>; }
function LoadingState() { return <div className="loading-state"><span className="loading-spinner" /><strong>Загружаем ваши финансы</strong><p>Получаем конверты и историю операций</p></div>; }
function ShieldLine() { return <span className="footer-shield"><BadgeCheck size={15} /></span>; }
function plural(value: number, one: string, few: string, many: string) {
  const absolute = Math.abs(value) % 100;
  const last = absolute % 10;
  if (absolute > 10 && absolute < 20) return many;
  if (last > 1 && last < 5) return few;
  if (last === 1) return one;
  return many;
}

export default App;
