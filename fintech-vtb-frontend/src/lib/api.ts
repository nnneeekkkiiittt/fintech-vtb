import type { AllocationUpdate, Envelope, Transaction, TransactionPage } from './types';

const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080').replace(/\/$/, '');
export const START_IN_DEMO_MODE = import.meta.env.VITE_USE_MOCK_DATA === 'true';

const initialTime = Date.now();
let demoEnvelopes: Envelope[] = [
  { id: 1, name: 'Рестораны', category: 'Еда вне дома', allocation_percent: 40, balance_kopecks: 725000, created_at: new Date(initialTime - 30 * 86400000).toISOString(), updated_at: new Date().toISOString() },
  { id: 2, name: 'Такси', category: 'Транспорт', allocation_percent: 25, balance_kopecks: 240000, created_at: new Date(initialTime - 30 * 86400000).toISOString(), updated_at: new Date().toISOString() },
  { id: 3, name: 'Продукты', category: 'Покупки', allocation_percent: 35, balance_kopecks: 560000, created_at: new Date(initialTime - 30 * 86400000).toISOString(), updated_at: new Date().toISOString() },
];
let nextEnvelopeId = 4;
let nextTransactionId = 5;
let demoTransactions: Transaction[] = [
  { id: 4, type: 'expense', amount_kopecks: 84235, description: 'Покупки в супермаркете', created_at: new Date(initialTime - 2 * 3600000).toISOString(), allocations: [{ envelope_id: 3, envelope_name: 'Продукты', amount_kopecks: 84235 }] },
  { id: 3, type: 'income', amount_kopecks: 8750000, description: 'Пополнение счёта', created_at: new Date(initialTime - 8 * 3600000).toISOString(), allocations: [{ envelope_id: 1, envelope_name: 'Рестораны', amount_kopecks: 3500000 }, { envelope_id: 2, envelope_name: 'Такси', amount_kopecks: 2187500 }, { envelope_id: 3, envelope_name: 'Продукты', amount_kopecks: 3062500 }] },
  { id: 2, type: 'expense', amount_kopecks: 46000, description: 'Кофе', created_at: new Date(initialTime - 28 * 3600000).toISOString(), allocations: [{ envelope_id: 1, envelope_name: 'Рестораны', amount_kopecks: 46000 }] },
  { id: 1, type: 'expense', amount_kopecks: 53000, description: 'Поездка по городу', created_at: new Date(initialTime - 2 * 86400000).toISOString(), allocations: [{ envelope_id: 2, envelope_name: 'Такси', amount_kopecks: 53000 }] },
];

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`${API_BASE_URL}${path}`, {
      ...init,
      headers: {
        Accept: 'application/json',
        ...(init?.body ? { 'Content-Type': 'application/json' } : {}),
        ...init?.headers,
      },
    });
  } catch {
    throw new Error(`Не удалось подключиться к API (${API_BASE_URL}). Проверь, что бэкенд запущен.`);
  }

  if (!response.ok) {
    let message = `Ошибка API: ${response.status}`;
    try {
      const payload = await response.json() as { error?: { message?: string } };
      if (payload.error?.message) message = payload.error.message;
    } catch {
      // The API may return an empty or non-JSON error body.
    }
    throw new Error(message);
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}

function demoCreateTransaction(type: 'income' | 'expense', amount: number, description: string, allocations: Transaction['allocations']): Transaction {
  const transaction: Transaction = {
    id: nextTransactionId++,
    type,
    amount_kopecks: amount,
    description,
    created_at: new Date().toISOString(),
    allocations,
  };
  demoTransactions = [transaction, ...demoTransactions];
  return transaction;
}

export async function getEnvelopes(useMock: boolean): Promise<Envelope[]> {
  if (useMock) return [...demoEnvelopes].sort((a, b) => a.id - b.id);
  const result = await request<{ data: Envelope[] }>('/api/v1/envelopes');
  return result.data;
}

export async function getTransactions(limit = 20, offset = 0, useMock = false): Promise<TransactionPage> {
  if (useMock) {
    const sorted = [...demoTransactions].sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
    return { data: sorted.slice(offset, offset + limit), pagination: { limit, offset } };
  }
  return request<TransactionPage>(`/api/v1/transactions?limit=${limit}&offset=${offset}`);
}

export async function createEnvelope(name: string, category: string, useMock: boolean): Promise<Envelope> {
  if (useMock) {
    const now = new Date().toISOString();
    const envelope: Envelope = { id: nextEnvelopeId++, name, category, allocation_percent: 0, balance_kopecks: 0, created_at: now, updated_at: now };
    demoEnvelopes = [...demoEnvelopes, envelope];
    return envelope;
  }
  const result = await request<{ data: Envelope }>('/api/v1/envelopes', {
    method: 'POST',
    body: JSON.stringify({ name, category }),
  });
  return result.data;
}

export async function updateEnvelope(id: number, name: string, category: string, useMock: boolean): Promise<Envelope> {
  if (useMock) {
    const now = new Date().toISOString();
    demoEnvelopes = demoEnvelopes.map((item) => item.id === id ? { ...item, name, category, updated_at: now } : item);
    demoTransactions = demoTransactions.map((tx) => ({
      ...tx,
      allocations: tx.allocations.map((allocation) => allocation.envelope_id === id ? { ...allocation, envelope_name: name } : allocation),
    }));
    const updated = demoEnvelopes.find((item) => item.id === id);
    if (!updated) throw new Error('Конверт не найден');
    return updated;
  }
  const result = await request<{ data: Envelope }>(`/api/v1/envelopes/${id}`, {
    method: 'PUT',
    body: JSON.stringify({ name, category }),
  });
  return result.data;
}

export async function deleteEnvelope(id: number, useMock: boolean): Promise<void> {
  if (useMock) {
    const envelope = demoEnvelopes.find((item) => item.id === id);
    if (!envelope) throw new Error('Конверт не найден');
    if (envelope.balance_kopecks !== 0) throw new Error('Сначала потрать деньги из конверта, чтобы удалить его');
    if (demoTransactions.some((tx) => tx.allocations.some((allocation) => allocation.envelope_id === id))) {
      throw new Error('Нельзя удалить конверт, по которому уже есть операции');
    }
    if (envelope.allocation_percent !== 0) throw new Error('Сначала установи долю конверта в 0%');
    demoEnvelopes = demoEnvelopes.filter((item) => item.id !== id);
    return;
  }
  await request<void>(`/api/v1/envelopes/${id}`, { method: 'DELETE' });
}

export async function setAllocations(allocations: AllocationUpdate[], useMock: boolean): Promise<Envelope[]> {
  if (allocations.reduce((sum, item) => sum + item.percent, 0) !== 100) {
    throw new Error('Сумма процентов должна быть равна 100%');
  }
  if (useMock) {
    const ids = new Set(allocations.map((item) => item.envelope_id));
    if (ids.size !== demoEnvelopes.length || demoEnvelopes.some((item) => !ids.has(item.id))) {
      throw new Error('Передай долю для каждого конверта ровно один раз');
    }
    demoEnvelopes = demoEnvelopes.map((envelope) => ({
      ...envelope,
      allocation_percent: allocations.find((item) => item.envelope_id === envelope.id)!.percent,
      updated_at: new Date().toISOString(),
    }));
    return [...demoEnvelopes];
  }
  const result = await request<{ data: Envelope[] }>('/api/v1/allocations', {
    method: 'PUT',
    body: JSON.stringify({ allocations }),
  });
  return result.data;
}

export async function createIncome(amountRubles: number, description: string, useMock: boolean): Promise<void> {
  const amount_kopecks = Math.round(amountRubles * 100);
  if (amount_kopecks <= 0) throw new Error('Сумма должна быть больше нуля');
  if (useMock) {
    const percentTotal = demoEnvelopes.reduce((sum, item) => sum + item.allocation_percent, 0);
    if (percentTotal !== 100) throw new Error('Перед пополнением настрой проценты: в сумме должно быть 100%');
    if (demoEnvelopes.length === 0) throw new Error('Сначала создай хотя бы один конверт');
    const shares = demoEnvelopes.map((envelope) => {
      const base = Math.floor(amount_kopecks / 100) * envelope.allocation_percent + Math.floor((amount_kopecks % 100) * envelope.allocation_percent / 100);
      const remainder = ((amount_kopecks % 100) * envelope.allocation_percent) % 100;
      return { envelope, base, remainder };
    });
    let left = amount_kopecks - shares.reduce((sum, item) => sum + item.base, 0);
    [...shares].sort((a, b) => b.remainder - a.remainder || a.envelope.id - b.envelope.id).forEach((item) => {
      if (left > 0) { item.base += 1; left -= 1; }
    });
    shares.forEach(({ envelope, base }) => {
      const found = demoEnvelopes.find((item) => item.id === envelope.id);
      if (found) found.balance_kopecks += base;
    });
    demoCreateTransaction('income', amount_kopecks, description || 'Пополнение счёта', shares.map(({ envelope, base }) => ({ envelope_id: envelope.id, envelope_name: envelope.name, amount_kopecks: base })).filter((item) => item.amount_kopecks > 0));
    return;
  }
  await request<{ data: Transaction }>('/api/v1/transactions/income', {
    method: 'POST',
    body: JSON.stringify({ amount_kopecks, description }),
  });
}

export async function createExpense(envelopeId: number, amountRubles: number, description: string, useMock: boolean): Promise<void> {
  const amount_kopecks = Math.round(amountRubles * 100);
  if (amount_kopecks <= 0) throw new Error('Сумма должна быть больше нуля');
  if (useMock) {
    const envelope = demoEnvelopes.find((item) => item.id === envelopeId);
    if (!envelope) throw new Error('Конверт не найден');
    if (envelope.balance_kopecks < amount_kopecks) throw new Error('В конверте недостаточно денег');
    envelope.balance_kopecks -= amount_kopecks;
    envelope.updated_at = new Date().toISOString();
    demoCreateTransaction('expense', amount_kopecks, description || 'Покупка', [{ envelope_id: envelope.id, envelope_name: envelope.name, amount_kopecks }]);
    return;
  }
  await request<{ data: Transaction }>('/api/v1/transactions/expense', {
    method: 'POST',
    body: JSON.stringify({ envelope_id: envelopeId, amount_kopecks, description }),
  });
}
