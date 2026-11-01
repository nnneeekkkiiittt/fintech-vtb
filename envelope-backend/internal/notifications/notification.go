package notifications

import "time"

type Kind string

const (
	// KindSurveyInvite — приглашение пройти анкету.
	KindSurveyInvite Kind = "survey_invite"
	// KindProposal — набор конвертов и долей по результатам анкеты; можно применить.
	KindProposal Kind = "envelope_proposal"
	// KindEnvelopeEmpty — в конверте закончились деньги, можно увеличить его долю.
	KindEnvelopeEmpty Kind = "envelope_empty"
	// KindIncome — пополнение распределено по конвертам (информационное).
	KindIncome Kind = "income_received"
)

// Идентификаторы действий. start_survey и review выполняет сам фронтенд
// (открывает анкету или предложение), остальные отправляются в POST /notifications/:id/action.
const (
	ActionStartSurvey = "start_survey"
	ActionReview      = "review"
	ActionApply       = "apply"
	ActionBoost       = "boost"
	ActionDismiss     = "dismiss"
)

type Action struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type Notification struct {
	ID         int64        `json:"id"`
	Kind       Kind         `json:"kind"`
	Title      string       `json:"title"`
	Body       string       `json:"body"`
	CreatedAt  time.Time    `json:"created_at"`
	Read       bool         `json:"read"`
	Resolved   bool         `json:"resolved"`
	Actions    []Action     `json:"actions"`
	EnvelopeID int64        `json:"envelope_id,omitempty"`
	Proposal   []Suggestion `json:"proposal,omitempty"`
	// Key защищает от дублей: пока есть неразрешённое уведомление с тем же ключом,
	// новое не создаётся.
	Key string `json:"key,omitempty"`
}
