import { useCallback, useEffect, useState } from 'react';
import { getNotifications } from './api';
import type { AppNotification } from './types';

// Как часто спрашиваем бэкенд, не появились ли новые уведомления.
const POLL_INTERVAL_MS = 30_000;

/**
 * Загружает уведомления с бэкенда: сразу, затем по таймеру и при возвращении на вкладку.
 * Изменение refreshToken (например, после пополнения или расхода) запрашивает внеплановую
 * проверку правил на бэкенде, чтобы уведомление пришло без задержки.
 */
export function useNotifications(enabled: boolean, refreshToken: number) {
  const [items, setItems] = useState<AppNotification[]>([]);
  const [unread, setUnread] = useState(0);
  const [error, setError] = useState('');

  const refresh = useCallback(async (forceCheck = false) => {
    try {
      const result = await getNotifications(forceCheck);
      setItems(result.items);
      setUnread(result.unread);
      setError('');
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Не удалось загрузить уведомления');
    }
  }, []);

  useEffect(() => {
    if (!enabled) {
      setItems([]);
      setUnread(0);
      setError('');
      return;
    }
    void refresh();
    const timer = window.setInterval(() => { void refresh(); }, POLL_INTERVAL_MS);
    const onFocus = () => { void refresh(); };
    window.addEventListener('focus', onFocus);
    return () => {
      window.clearInterval(timer);
      window.removeEventListener('focus', onFocus);
    };
  }, [enabled, refresh]);

  useEffect(() => {
    if (enabled && refreshToken > 0) void refresh(true);
  }, [enabled, refreshToken, refresh]);

  return { items, unread, error, refresh };
}
