export type Envelope = {
  id: number;
  name: string;
  category: string;
  allocation_percent: number;
  balance_kopecks: number;
  daily_balance: number;
  created_at: string;
  updated_at: string;
};

export type TransactionType = 'income' | 'expense';

export type TransactionAllocation = {
  envelope_id: number;
  envelope_name: string;
  amount_kopecks: number;
};

export type Transaction = {
  id: number;
  type: TransactionType;
  amount_kopecks: number;
  description: string;
  created_at: string;
  allocations: TransactionAllocation[];
};

export type AllocationUpdate = {
  envelope_id: number;
  percent: number;
};

export type TransactionPage = {
  data: Transaction[];
  pagination: { limit: number; offset: number };
};

/** Ответы анкеты: номер вопроса (с 1) -> номера выбранных вариантов (с 1). */
export type Answers = Record<number, number[]>;

/** Конверт, который алгоритм подбора предлагает по результатам анкеты. */
export type RecommendedEnvelope = {
  name: string;
  category: string;
  description: string;
  /** 1 — самый высокий приоритет, 5 — самый низкий */
  priority: number;
  allocation_percent: number;
  reasons: string[];
};
