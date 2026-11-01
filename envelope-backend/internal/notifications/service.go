package notifications

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"envelope-backend/internal/domain"
)

const (
	// boostPoints — на сколько процентных пунктов увеличивается доля опустевшего конверта.
	boostPoints = 5
	// surveySnooze — через сколько снова напоминать об анкете после «Позже».
	surveySnooze = 24 * time.Hour
	// defaultPriority — приоритет конверта, о котором анкета ничего не знает.
	defaultPriority = 3
)

// EnvelopeStore — «общие методы» проекта, которыми пользуется механизм уведомлений.
// Его реализует repository.Repository, поэтому отдельных заглушек не нужно.
type EnvelopeStore interface {
	ListEnvelopes(ctx context.Context) ([]domain.Envelope, error)
	CreateEnvelope(ctx context.Context, name, category string) (domain.Envelope, error)
	SetAllocations(ctx context.Context, updates []domain.AllocationUpdate) ([]domain.Envelope, error)
	ListTransactions(ctx context.Context, limit, offset int) ([]domain.Transaction, error)
}

type Config struct {
	// DataDir — каталог для operations_history.json и notifications_state.json.
	DataDir string
	// CheckInterval — как часто планировщик проверяет, не пора ли отправить уведомление.
	CheckInterval time.Duration
	// SurveyRefreshAfter — через какое время после анкеты предложить пройти её снова.
	SurveyRefreshAfter time.Duration
}

// ConfigFromEnv читает настройки из переменных окружения:
// NOTIFICATIONS_DATA_DIR, NOTIFICATIONS_CHECK_INTERVAL (например, 30s), NOTIFICATIONS_SURVEY_REFRESH_DAYS.
func ConfigFromEnv() Config {
	cfg := Config{
		DataDir:            "data/notifications",
		CheckInterval:      time.Minute,
		SurveyRefreshAfter: 30 * 24 * time.Hour,
	}
	if v := os.Getenv("NOTIFICATIONS_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("NOTIFICATIONS_CHECK_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.CheckInterval = d
		}
	}
	if v := os.Getenv("NOTIFICATIONS_SURVEY_REFRESH_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.SurveyRefreshAfter = time.Duration(n) * 24 * time.Hour
		}
	}
	return cfg
}

type Service struct {
	envelopes EnvelopeStore
	history   *History
	store     *store
	cfg       Config
	now       func() time.Time
	// opMu не даёт планировщику и действиям пользователя менять состояние одновременно.
	opMu sync.Mutex
}

func NewService(envelopes EnvelopeStore, cfg Config) (*Service, error) {
	defaults := ConfigFromEnv()
	if cfg.DataDir == "" {
		cfg.DataDir = defaults.DataDir
	}
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = defaults.CheckInterval
	}
	if cfg.SurveyRefreshAfter <= 0 {
		cfg.SurveyRefreshAfter = defaults.SurveyRefreshAfter
	}
	history, err := OpenHistory(filepath.Join(cfg.DataDir, "operations_history.json"))
	if err != nil {
		return nil, err
	}
	st, err := openStore(filepath.Join(cfg.DataDir, "notifications_state.json"))
	if err != nil {
		return nil, err
	}
	return &Service{envelopes: envelopes, history: history, store: st, cfg: cfg, now: time.Now}, nil
}

// History возвращает журнал изменений конвертов, сделанных через уведомления.
func (s *Service) History() []Operation { return s.history.List() }

// Run запускает планировщик: проверка сразу при старте и затем каждые CheckInterval.
// Блокируется до отмены ctx, поэтому запускать нужно в отдельной горутине.
func (s *Service) Run(ctx context.Context) {
	s.scanAndLog(ctx)
	ticker := time.NewTicker(s.cfg.CheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.scanAndLog(ctx)
		}
	}
}

func (s *Service) scanAndLog(ctx context.Context) {
	if err := s.Scan(ctx); err != nil && ctx.Err() == nil {
		log.Printf("notifications: scan failed: %v", err)
	}
}

// Scan проверяет правила и создаёт уведомления. Вызывается планировщиком, но
// безопасно вызывать и вручную (например, в тестах).
func (s *Service) Scan(ctx context.Context) error {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	envelopes, err := s.envelopes.ListEnvelopes(ctx)
	if err != nil {
		return fmt.Errorf("list envelopes: %w", err)
	}
	transactions, err := s.envelopes.ListTransactions(ctx, 100, 0)
	if err != nil {
		return fmt.Errorf("list transactions: %w", err)
	}
	now := s.now()
	return s.store.update(func(st *state) (bool, error) {
		changed := st.scanSurvey(now, s.cfg.SurveyRefreshAfter)
		if st.scanEmptyEnvelopes(now, envelopes, transactions) {
			changed = true
		}
		if st.scanIncome(now, transactions) {
			changed = true
		}
		if st.prune(now) {
			changed = true
		}
		return changed, nil
	})
}

// scanSurvey: первое приглашение на анкету и повторное — спустя SurveyRefreshAfter.
func (st *state) scanSurvey(now time.Time, refreshAfter time.Duration) bool {
	if st.hasOpen(KindSurveyInvite) || st.hasOpen(KindProposal) {
		return false
	}
	if st.SurveySnoozedUntil != nil && now.Before(*st.SurveySnoozedUntil) {
		return false
	}
	switch {
	case st.SurveyCompletedAt == nil:
		return st.add(Notification{
			Kind:    KindSurveyInvite,
			Title:   "Настроим конверты под вас",
			Body:    "Ответьте на 15 коротких вопросов — мы предложим набор конвертов и доли для распределения пополнений.",
			Actions: []Action{{ID: ActionStartSurvey, Label: "Пройти анкету"}, {ID: ActionDismiss, Label: "Позже"}},
		}, now)
	case now.Sub(*st.SurveyCompletedAt) >= refreshAfter:
		return st.add(Notification{
			Kind:    KindSurveyInvite,
			Title:   "Пора обновить анкету",
			Body:    "С прошлого раза прошло много времени. Пройдите анкету ещё раз, чтобы конверты соответствовали вашим расходам.",
			Actions: []Action{{ID: ActionStartSurvey, Label: "Пройти анкету"}, {ID: ActionDismiss, Label: "Позже"}},
		}, now)
	}
	return false
}

// scanEmptyEnvelopes: конверт получал деньги и теперь пуст, а доля у него ненулевая.
// Повторно уведомляем только после того, как в конверте снова появятся деньги.
func (st *state) scanEmptyEnvelopes(now time.Time, envelopes []domain.Envelope, txs []domain.Transaction) bool {
	funded := make(map[int64]bool)
	for _, tx := range txs {
		if tx.Type != domain.TransactionIncome {
			continue
		}
		for _, a := range tx.Allocations {
			funded[a.EnvelopeID] = true
		}
	}
	if st.EmptyNotified == nil {
		st.EmptyNotified = make(map[int64]bool)
	}
	changed := false
	for _, e := range envelopes {
		if e.BalanceKopecks > 0 {
			if st.EmptyNotified[e.ID] {
				delete(st.EmptyNotified, e.ID)
				changed = true
			}
			continue
		}
		if e.AllocationPercent == 0 || !funded[e.ID] || st.EmptyNotified[e.ID] {
			continue
		}
		st.add(Notification{
			Kind:       KindEnvelopeEmpty,
			EnvelopeID: e.ID,
			Key:        fmt.Sprintf("envelope_empty:%d", e.ID),
			Title:      fmt.Sprintf("В конверте «%s» закончились деньги", e.Name),
			Body: fmt.Sprintf("Следующие пополнения поступят в него по текущей доле — %d%%. "+
				"Можно увеличить долю на %d п. п. за счёт менее важных конвертов.", e.AllocationPercent, boostPoints),
			Actions: []Action{
				{ID: ActionBoost, Label: fmt.Sprintf("Увеличить долю на %d п. п.", boostPoints)},
				{ID: ActionDismiss, Label: "Позже"},
			},
		}, now)
		st.EmptyNotified[e.ID] = true
		changed = true
	}
	return changed
}

// scanIncome: сообщает о пополнениях, появившихся с прошлой проверки.
// При самой первой проверке старые операции только запоминаются, но не озвучиваются.
func (st *state) scanIncome(now time.Time, txs []domain.Transaction) bool {
	var newest int64
	for _, tx := range txs {
		if tx.ID > newest {
			newest = tx.ID
		}
	}
	if !st.Initialized {
		st.Initialized = true
		st.LastTransactionID = newest
		return true
	}
	if newest <= st.LastTransactionID {
		return false
	}
	// txs отсортированы от новых к старым; уведомления создаём от старых к новым.
	for i := len(txs) - 1; i >= 0; i-- {
		tx := txs[i]
		if tx.ID <= st.LastTransactionID || tx.Type != domain.TransactionIncome {
			continue
		}
		parts := make([]string, 0, len(tx.Allocations))
		for _, a := range tx.Allocations {
			parts = append(parts, fmt.Sprintf("%s — %s", a.EnvelopeName, formatRubles(a.AmountKopecks)))
		}
		st.add(Notification{
			Kind:    KindIncome,
			Key:     fmt.Sprintf("income:%d", tx.ID),
			Title:   fmt.Sprintf("Пополнение на %s распределено", formatRubles(tx.AmountKopecks)),
			Body:    strings.Join(parts, ", "),
			Actions: []Action{{ID: ActionDismiss, Label: "Понятно"}},
		}, now)
	}
	st.LastTransactionID = newest
	return true
}

// List возвращает неразрешённые уведомления (новые первыми) и число непрочитанных.
func (s *Service) List(unreadOnly bool) ([]Notification, int) {
	items := make([]Notification, 0)
	unread := 0
	s.store.view(func(st *state) {
		for i := len(st.Notifications) - 1; i >= 0; i-- {
			n := st.Notifications[i]
			if n.Resolved {
				continue
			}
			if !n.Read {
				unread++
			}
			if unreadOnly && n.Read {
				continue
			}
			items = append(items, n)
		}
	})
	return items, unread
}

func (s *Service) MarkRead(id int64) (Notification, error) {
	var out Notification
	err := s.store.update(func(st *state) (bool, error) {
		n := st.find(id)
		if n == nil {
			return false, ErrNotificationNotFound
		}
		changed := !n.Read
		n.Read = true
		out = *n
		return changed, nil
	})
	return out, err
}

// SubmitSurvey принимает ответы, подбирает конверты и создаёт уведомление-предложение.
// Конверты при этом не меняются: изменения применяются только после действия «apply».
func (s *Service) SubmitSurvey(answers Answers) (Notification, error) {
	if err := ValidateAnswers(answers); err != nil {
		return Notification{}, err
	}
	proposal := Recommend(answers)

	s.opMu.Lock()
	defer s.opMu.Unlock()
	now := s.now()
	var created Notification
	err := s.store.update(func(st *state) (bool, error) {
		for i := range st.Notifications {
			n := &st.Notifications[i]
			if !n.Resolved && (n.Kind == KindSurveyInvite || n.Kind == KindProposal) {
				n.Resolved = true
				n.Read = true
			}
		}
		st.SurveyCompletedAt = &now
		st.SurveySnoozedUntil = nil
		st.add(Notification{
			Kind:  KindProposal,
			Title: "Мы подобрали конверты для вас",
			Body: fmt.Sprintf("Предлагаем %d %s и доли пополнения. Изменения вступят в силу только после подтверждения.",
				len(proposal), pluralRu(len(proposal), "конверт", "конверта", "конвертов")),
			Proposal: proposal,
			Actions:  []Action{{ID: ActionReview, Label: "Посмотреть"}, {ID: ActionApply, Label: "Применить"}, {ID: ActionDismiss, Label: "Отклонить"}},
		}, now)
		created = st.Notifications[len(st.Notifications)-1]
		return true, nil
	})
	if err != nil {
		return Notification{}, err
	}
	s.record(Operation{Type: "Анкета пройдена", Description: fmt.Sprintf("Подобрано конвертов: %d", len(proposal))})
	return created, nil
}

// Act выполняет действие над уведомлением: apply, boost или dismiss.
func (s *Service) Act(ctx context.Context, id int64, action string) (Notification, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	n, err := s.store.get(id)
	if err != nil {
		return Notification{}, err
	}
	if n.Resolved {
		return Notification{}, ErrAlreadyResolved
	}
	switch {
	case action == ActionDismiss:
		now := s.now()
		return s.store.resolve(n.ID, func(st *state) {
			if n.Kind == KindSurveyInvite {
				until := now.Add(surveySnooze)
				st.SurveySnoozedUntil = &until
			}
		})
	case action == ActionApply && n.Kind == KindProposal:
		return s.applyProposal(ctx, n)
	case action == ActionBoost && n.Kind == KindEnvelopeEmpty:
		return s.boostEnvelope(ctx, n)
	default:
		return Notification{}, ErrUnknownAction
	}
}

// applyProposal создаёт недостающие конверты и выставляет доли из предложения.
// Конверты определяются по названию без учёта регистра. Существующие конверты, которых нет
// в предложении, остаются со всеми деньгами, но получают долю 0%.
// Операция безопасна для повтора: уже созданные конверты второй раз не создаются.
func (s *Service) applyProposal(ctx context.Context, n Notification) (Notification, error) {
	existing, err := s.envelopes.ListEnvelopes(ctx)
	if err != nil {
		return Notification{}, fmt.Errorf("list envelopes: %w", err)
	}
	byName := make(map[string]domain.Envelope, len(existing))
	for _, e := range existing {
		byName[normalizeName(e.Name)] = e
	}

	all := append([]domain.Envelope(nil), existing...)
	percentByID := make(map[int64]int, len(n.Proposal))
	priorities := make(map[int64]int, len(n.Proposal))
	for _, item := range n.Proposal {
		key := normalizeName(item.Name)
		env, ok := byName[key]
		if !ok {
			env, err = s.envelopes.CreateEnvelope(ctx, item.Name, item.Category)
			if err != nil {
				return Notification{}, fmt.Errorf("create envelope %q: %w", item.Name, err)
			}
			byName[key] = env
			all = append(all, env)
			s.record(Operation{Type: "Конверт создан", To: env.Name, Description: "Конверт создан по результатам анкеты"})
		}
		percentByID[env.ID] = item.Percent
		priorities[env.ID] = item.Priority
	}

	updates := make([]domain.AllocationUpdate, 0, len(all))
	for _, e := range all {
		updates = append(updates, domain.AllocationUpdate{EnvelopeID: e.ID, Percent: percentByID[e.ID]})
	}
	if _, err := s.envelopes.SetAllocations(ctx, updates); err != nil {
		return Notification{}, fmt.Errorf("set allocations: %w", err)
	}
	s.record(Operation{Type: "Доли изменены", Description: describeProposal(n.Proposal)})

	return s.store.resolve(n.ID, func(st *state) { st.Priorities = priorities })
}

// boostEnvelope увеличивает долю опустевшего конверта на boostPoints п. п. за счёт самых
// малоприоритетных конвертов. Конверты с приоритетом 1 (самые важные) долю не отдают.
func (s *Service) boostEnvelope(ctx context.Context, n Notification) (Notification, error) {
	envelopes, err := s.envelopes.ListEnvelopes(ctx)
	if err != nil {
		return Notification{}, fmt.Errorf("list envelopes: %w", err)
	}
	percents := make(map[int64]int, len(envelopes))
	var target *domain.Envelope
	for i := range envelopes {
		percents[envelopes[i].ID] = envelopes[i].AllocationPercent
		if envelopes[i].ID == n.EnvelopeID {
			target = &envelopes[i]
		}
	}
	if target == nil {
		return Notification{}, domain.ErrNotFound
	}

	priorities := s.store.priorities()
	priorityOf := func(id int64) int {
		if p, ok := priorities[id]; ok {
			return p
		}
		return defaultPriority
	}
	donors := make([]domain.Envelope, 0, len(envelopes))
	for _, e := range envelopes {
		if e.ID == target.ID || e.AllocationPercent == 0 || priorityOf(e.ID) <= 1 {
			continue
		}
		donors = append(donors, e)
	}
	sort.SliceStable(donors, func(i, j int) bool {
		pi, pj := priorityOf(donors[i].ID), priorityOf(donors[j].ID)
		if pi != pj {
			return pi > pj
		}
		return donors[i].AllocationPercent > donors[j].AllocationPercent
	})

	remaining := boostPoints
	var sources []string
	for _, d := range donors {
		if remaining == 0 {
			break
		}
		take := min(remaining, percents[d.ID])
		percents[d.ID] -= take
		percents[target.ID] += take
		remaining -= take
		sources = append(sources, d.Name)
	}
	moved := boostPoints - remaining
	if moved == 0 {
		return Notification{}, ErrNothingToMove
	}

	updates := make([]domain.AllocationUpdate, 0, len(envelopes))
	for _, e := range envelopes {
		updates = append(updates, domain.AllocationUpdate{EnvelopeID: e.ID, Percent: percents[e.ID]})
	}
	if _, err := s.envelopes.SetAllocations(ctx, updates); err != nil {
		return Notification{}, fmt.Errorf("set allocations: %w", err)
	}
	s.record(Operation{
		Type:        "Перераспределение долей",
		From:        strings.Join(sources, ", "),
		To:          target.Name,
		Description: fmt.Sprintf("Доля конверта увеличена на %d п. п. за счёт менее приоритетных конвертов", moved),
	})
	return s.store.resolve(n.ID, nil)
}

func (s *Service) record(op Operation) {
	op.Date = s.now()
	if err := s.history.Record(op); err != nil {
		log.Printf("notifications: write history: %v", err)
	}
}

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func describeProposal(items []Suggestion) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, fmt.Sprintf("%s — %d%%", item.Name, item.Percent))
	}
	return "Новые доли: " + strings.Join(parts, ", ")
}

func pluralRu(n int, one, few, many string) string {
	n = n % 100
	last := n % 10
	switch {
	case n > 10 && n < 20:
		return many
	case last == 1:
		return one
	case last > 1 && last < 5:
		return few
	default:
		return many
	}
}

// formatRubles форматирует копейки как «10 000 ₽» или «1 234,50 ₽» (с неразрывным пробелом).
func formatRubles(kopecks int64) string {
	rubles := kopecks / 100
	kop := kopecks % 100
	digits := strconv.FormatInt(rubles, 10)
	var b strings.Builder
	for i := 0; i < len(digits); i++ {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteString("\u00a0")
		}
		b.WriteByte(digits[i])
	}
	if kop != 0 {
		return fmt.Sprintf("%s,%02d\u00a0₽", b.String(), kop)
	}
	return b.String() + "\u00a0₽"
}
