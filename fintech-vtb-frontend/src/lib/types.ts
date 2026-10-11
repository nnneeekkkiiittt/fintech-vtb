export type Envelope = {
  id: number;
  name: string;
  category: string;
  allocation_percent: number;
  balance_kopecks: number;
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
