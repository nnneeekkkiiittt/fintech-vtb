export type NotificationKind = 'survey_invite' | 'envelope_proposal' | 'envelope_empty' | 'income_received';

export type NotificationAction = {
  id: string;
  label: string;
};

export type ProposalItem = {
  name: string;
  category: string;
  priority: number;
  percent: number;
  reasons: string[];
};

// Не называем тип Notification, чтобы не путать с браузерным Notification.
export type AppNotification = {
  id: number;
  kind: NotificationKind;
  title: string;
  body: string;
  created_at: string;
  read: boolean;
  resolved: boolean;
  actions: NotificationAction[];
  envelope_id?: number;
  proposal?: ProposalItem[];
};

export type SurveyQuestion = {
  id: number;
  text: string;
  options: string[];
  multi: boolean;
};

// Номер вопроса → номера выбранных вариантов (оба с единицы).
export type SurveyAnswers = Record<number, number[]>;
