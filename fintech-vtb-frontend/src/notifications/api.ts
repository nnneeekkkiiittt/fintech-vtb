import { request } from '../lib/api';
import type { AppNotification, SurveyAnswers, SurveyQuestion } from './types';

export async function getNotifications(forceCheck = false): Promise<{ items: AppNotification[]; unread: number }> {
  const result = await request<{ data: AppNotification[]; unread_count: number }>(
    `/api/v1/notifications${forceCheck ? '?refresh=1' : ''}`,
  );
  return { items: result.data, unread: result.unread_count };
}

export async function markNotificationRead(id: number): Promise<void> {
  await request<unknown>(`/api/v1/notifications/${id}/read`, { method: 'POST' });
}

export async function runNotificationAction(id: number, action: string): Promise<AppNotification> {
  const result = await request<{ data: AppNotification }>(`/api/v1/notifications/${id}/action`, {
    method: 'POST',
    body: JSON.stringify({ action }),
  });
  return result.data;
}

export async function getSurveyQuestions(): Promise<SurveyQuestion[]> {
  const result = await request<{ data: SurveyQuestion[] }>('/api/v1/survey/questions');
  return result.data;
}

export async function submitSurvey(answers: SurveyAnswers): Promise<AppNotification> {
  const result = await request<{ data: AppNotification }>('/api/v1/survey', {
    method: 'POST',
    body: JSON.stringify({ answers }),
  });
  return result.data;
}
