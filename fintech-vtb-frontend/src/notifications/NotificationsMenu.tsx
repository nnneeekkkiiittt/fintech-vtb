import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { Bell, X } from 'lucide-react';
import { markNotificationRead, runNotificationAction } from './api';
import { SurveyDialog } from './SurveyDialog';
import type { AppNotification } from './types';
import { useNotifications } from './useNotifications';
import './notifications.css';

type DialogState = { kind: 'survey' } | { kind: 'proposal'; notification: AppNotification };

type Props = {
  useMock: boolean;
  // Меняется после операций пользователя — тогда бэкенд проверяет правила вне расписания.
  refreshToken: number;
  onEnvelopesChanged: () => void | Promise<void>;
  onToast: (message: string) => void;
};

const SUCCESS_MESSAGES: Record<string, string> = {
  boost: 'Доля конверта увеличена',
};

const formatTime = (isoDate: string) => new Intl.DateTimeFormat('ru-RU', {
  day: 'numeric', month: 'long', hour: '2-digit', minute: '2-digit',
}).format(new Date(isoDate));

export function NotificationsMenu({ useMock, refreshToken, onEnvelopesChanged, onToast }: Props) {
  const [open, setOpen] = useState(false);
  const [dialog, setDialog] = useState<DialogState | null>(null);
  const [busyId, setBusyId] = useState<number | null>(null);
  const ref = useRef<HTMLDivElement>(null);
  const { items, unread, error, refresh } = useNotifications(!useMock, refreshToken);

  useEffect(() => {
    if (!open) return;
    const onClick = (event: MouseEvent) => {
      if (ref.current && !ref.current.contains(event.target as Node)) setOpen(false);
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', onClick);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onClick);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  async function togglePanel() {
    const next = !open;
    setOpen(next);
    if (!next) return;
    const unreadItems = items.filter((item) => !item.read);
    if (unreadItems.length === 0) return;
    try {
      await Promise.all(unreadItems.map((item) => markNotificationRead(item.id)));
      await refresh();
    } catch {
      // Не критично: уведомления останутся непрочитанными до следующего открытия.
    }
  }

  async function handleAction(notification: AppNotification, actionId: string) {
    if (actionId === 'start_survey') {
      setOpen(false);
      setDialog({ kind: 'survey' });
      return;
    }
    if (actionId === 'review') {
      setOpen(false);
      setDialog({ kind: 'proposal', notification });
      return;
    }
    setBusyId(notification.id);
    try {
      await runNotificationAction(notification.id, actionId);
      if (actionId === 'boost' || actionId === 'apply') await onEnvelopesChanged();
      const message = SUCCESS_MESSAGES[actionId];
      if (message) onToast(message);
    } catch (cause) {
      onToast(cause instanceof Error ? cause.message : 'Не удалось выполнить действие');
    } finally {
      setBusyId(null);
      await refresh();
    }
  }

  return (
    <div className="notifications" ref={ref}>
      <button
        className="notifications-button"
        onClick={() => void togglePanel()}
        aria-label={unread > 0 ? `Уведомления, непрочитанных: ${unread}` : 'Уведомления'}
        aria-expanded={open}
      >
        <Bell size={19} />
      </button>
      {unread > 0 && <span className="notifications-badge" aria-hidden="true">{unread > 9 ? '9+' : unread}</span>}
      {open && (
        <div className="notifications-panel notifications-panel-wide" role="dialog" aria-label="Уведомления">
          <div className="notifications-title">Уведомления</div>
          {useMock && <p className="notifications-empty">В демо-режиме уведомления недоступны. Переключитесь на реальный API.</p>}
          {!useMock && error && <p className="notifications-empty">{error}</p>}
          {!useMock && !error && items.length === 0 && <p className="notifications-empty">Новых уведомлений пока нет</p>}
          {!useMock && items.length > 0 && (
            <ul className="notification-list">
              {items.map((item) => {
                // «Применить» доступно только внутри окна с предложением, где видны все доли.
                const actions = item.actions.filter((action) => !(item.kind === 'envelope_proposal' && action.id === 'apply'));
                return (
                  <li key={item.id} className={`notification-item ${item.read ? '' : 'unread'}`}>
                    <p className="notification-title">{item.title}</p>
                    <p className="notification-body">{item.body}</p>
                    <time className="notification-time" dateTime={item.created_at}>{formatTime(item.created_at)}</time>
                    {actions.length > 0 && (
                      <div className="notification-actions">
                        {actions.map((action, index) => (
                          <button
                            key={action.id}
                            className={`notification-action ${index === 0 ? 'primary' : ''}`}
                            disabled={busyId === item.id}
                            onClick={() => void handleAction(item, action.id)}
                          >
                            {action.label}
                          </button>
                        ))}
                      </div>
                    )}
                    <button className="notification-close" aria-label="Скрыть уведомление" disabled={busyId === item.id} onClick={() => void handleAction(item, 'dismiss')}><X size={14} /></button>
                  </li>
                );
              })}
            </ul>
          )}
        </div>
      )}
      {/* Окно рендерим в body: у шапки есть backdrop-filter, из-за которого position: fixed считался бы от шапки. */}
      {dialog && createPortal(
        <SurveyDialog
          initialProposal={dialog.kind === 'proposal' ? dialog.notification : undefined}
          onClose={() => setDialog(null)}
          onChanged={() => { void refresh(); }}
          onEnvelopesChanged={onEnvelopesChanged}
          onToast={onToast}
        />,
        document.body,
      )}
    </div>
  );
}
