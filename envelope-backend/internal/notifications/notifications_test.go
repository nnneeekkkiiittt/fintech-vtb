package notifications

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"envelope-backend/internal/domain"
)

// fakeStore — минимальная замена репозитория для тестов; повторяет проверки SetAllocations.
type fakeStore struct {
	envelopes []domain.Envelope
	txs       []domain.Transaction
	nextID    int64
}

func (f *fakeStore) ListEnvelopes(_ context.Context) ([]domain.Envelope, error) {
	return append([]domain.Envelope(nil), f.envelopes...), nil
}

func (f *fakeStore) CreateEnvelope(_ context.Context, name, category string) (domain.Envelope, error) {
	f.nextID++
	e := domain.Envelope{ID: f.nextID, Name: name, Category: category}
	f.envelopes = append(f.envelopes, e)
	return e, nil
}

func (f *fakeStore) SetAllocations(_ context.Context, updates []domain.AllocationUpdate) ([]domain.Envelope, error) {
	total := 0
	provided := make(map[int64]int, len(updates))
	for _, u := range updates {
		provided[u.EnvelopeID] = u.Percent
		total += u.Percent
	}
	if total != 100 || len(provided) != len(updates) || len(provided) != len(f.envelopes) {
		return nil, domain.ErrInvalidAllocation
	}
	for i := range f.envelopes {
		percent, ok := provided[f.envelopes[i].ID]
		if !ok {
			return nil, domain.ErrInvalidAllocation
		}
		f.envelopes[i].AllocationPercent = percent
	}
	return append([]domain.Envelope(nil), f.envelopes...), nil
}

func (f *fakeStore) ListTransactions(_ context.Context, _, _ int) ([]domain.Transaction, error) {
	return append([]domain.Transaction(nil), f.txs...), nil
}

func newTestService(t *testing.T, store EnvelopeStore) *Service {
	t.Helper()
	svc, err := NewService(store, Config{DataDir: filepath.Join(t.TempDir(), "nested", "data")})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	svc.now = func() time.Time { return time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC) }
	return svc
}

func baseAnswers() Answers {
	answers := Answers{}
	for i := range questions {
		answers[i+1] = []int{1}
	}
	return answers
}

func TestValidateAnswers(t *testing.T) {
	if err := ValidateAnswers(baseAnswers()); err != nil {
		t.Fatalf("ValidateAnswers(valid) error = %v", err)
	}
	missing := baseAnswers()
	delete(missing, 3)
	if err := ValidateAnswers(missing); err == nil {
		t.Fatal("ValidateAnswers() accepted answers with a missing question")
	}
	tooMany := baseAnswers()
	tooMany[1] = []int{1, 2} // вопрос 1 — с одним вариантом ответа
	if err := ValidateAnswers(tooMany); err == nil {
		t.Fatal("ValidateAnswers() accepted two options for a single-choice question")
	}
	outOfRange := baseAnswers()
	outOfRange[2] = []int{99}
	if err := ValidateAnswers(outOfRange); err == nil {
		t.Fatal("ValidateAnswers() accepted a non-existent option")
	}
}

func TestRecommendPercentsSumTo100(t *testing.T) {
	items := Recommend(baseAnswers())
	if len(items) < 2 {
		t.Fatalf("Recommend() returned %d envelopes, want at least the 2 base ones", len(items))
	}
	sum := 0
	for _, item := range items {
		if item.Percent <= 0 {
			t.Fatalf("envelope %q got %d%%", item.Name, item.Percent)
		}
		sum += item.Percent
	}
	if sum != 100 {
		t.Fatalf("percents sum to %d, want 100", sum)
	}
}

func TestRecommendOverspendRaisesPriority(t *testing.T) {
	priorityOfCafe := func(a Answers) int {
		for _, item := range Recommend(a) {
			if item.Name == "Кафе, рестораны и развлечения" {
				return item.Priority
			}
		}
		t.Fatal("cafe envelope was not recommended")
		return 0
	}
	plain := baseAnswers()
	plain[7] = []int{2}
	plain[9] = []int{10}
	overspend := baseAnswers()
	overspend[7] = []int{2}
	overspend[9] = []int{2}
	if got, want := priorityOfCafe(overspend), priorityOfCafe(plain)-1; got != want {
		t.Fatalf("priority with overspend = %d, want %d", got, want)
	}
}

func TestOpenHistoryCreatesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "operations_history.json")
	history, err := OpenHistory(path)
	if err != nil {
		t.Fatalf("OpenHistory() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("history file was not created: %v", err)
	}
	if err := history.Record(Operation{Type: "Тест", Description: "первая запись"}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	reopened, err := OpenHistory(path)
	if err != nil {
		t.Fatalf("OpenHistory(existing) error = %v", err)
	}
	if got := len(reopened.List()); got != 1 {
		t.Fatalf("reopened history has %d records, want 1", got)
	}
}

func TestOpenHistoryKeepsCorruptedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operations_history.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	history, err := OpenHistory(path)
	if err != nil {
		t.Fatalf("OpenHistory() error = %v", err)
	}
	if got := len(history.List()); got != 0 {
		t.Fatalf("history has %d records, want 0", got)
	}
	backups, _ := filepath.Glob(path + ".corrupt-*")
	if len(backups) != 1 {
		t.Fatalf("expected one backup of the corrupted file, got %v", backups)
	}
}

func TestScanCreatesSurveyInviteOnce(t *testing.T) {
	svc := newTestService(t, &fakeStore{})
	for i := 0; i < 3; i++ {
		if err := svc.Scan(context.Background()); err != nil {
			t.Fatalf("Scan() error = %v", err)
		}
	}
	items, unread := svc.List(false)
	if len(items) != 1 || items[0].Kind != KindSurveyInvite || unread != 1 {
		t.Fatalf("unexpected notifications after repeated scans: %+v (unread %d)", items, unread)
	}
}

func TestDismissedSurveyInviteIsSnoozed(t *testing.T) {
	svc := newTestService(t, &fakeStore{})
	ctx := context.Background()
	if err := svc.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	items, _ := svc.List(false)
	if _, err := svc.Act(ctx, items[0].ID, ActionDismiss); err != nil {
		t.Fatalf("Act(dismiss) error = %v", err)
	}
	if err := svc.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if items, _ := svc.List(false); len(items) != 0 {
		t.Fatalf("invite came back right after «Позже»: %+v", items)
	}
	svc.now = func() time.Time { return time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC) }
	if err := svc.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if items, _ := svc.List(false); len(items) != 1 {
		t.Fatalf("invite did not return after the snooze period: %+v", items)
	}
}

func TestApplyProposalCreatesEnvelopesAndSetsAllocations(t *testing.T) {
	fake := &fakeStore{
		nextID:    1,
		envelopes: []domain.Envelope{{ID: 1, Name: "Продукты и товары для дома", AllocationPercent: 100}},
	}
	svc := newTestService(t, fake)
	ctx := context.Background()

	proposal, err := svc.SubmitSurvey(baseAnswers())
	if err != nil {
		t.Fatalf("SubmitSurvey() error = %v", err)
	}
	if proposal.Kind != KindProposal || len(proposal.Proposal) == 0 {
		t.Fatalf("unexpected proposal: %+v", proposal)
	}
	if _, err := svc.Act(ctx, proposal.ID, ActionApply); err != nil {
		t.Fatalf("Act(apply) error = %v", err)
	}

	total := 0
	for _, e := range fake.envelopes {
		total += e.AllocationPercent
	}
	if total != 100 {
		t.Fatalf("allocations sum to %d, want 100", total)
	}
	if len(fake.envelopes) != len(proposal.Proposal) {
		t.Fatalf("got %d envelopes, want %d (the existing one must be reused, not duplicated)", len(fake.envelopes), len(proposal.Proposal))
	}
	if _, err := svc.Act(ctx, proposal.ID, ActionApply); err != ErrAlreadyResolved {
		t.Fatalf("second apply error = %v, want ErrAlreadyResolved", err)
	}
	if got := len(svc.History()); got == 0 {
		t.Fatal("history is empty after applying the proposal")
	}
}

func TestEmptyEnvelopeNotificationAndBoost(t *testing.T) {
	fake := &fakeStore{
		envelopes: []domain.Envelope{
			{ID: 1, Name: "А", AllocationPercent: 50, BalanceKopecks: 1000},
			{ID: 2, Name: "Б", AllocationPercent: 30, BalanceKopecks: 1000},
			{ID: 3, Name: "В", AllocationPercent: 20, BalanceKopecks: 0},
		},
		txs: []domain.Transaction{{
			ID: 1, Type: domain.TransactionIncome, AmountKopecks: 5000,
			Allocations: []domain.TransactionAllocation{{EnvelopeID: 3, EnvelopeName: "В", AmountKopecks: 1000}},
		}},
	}
	svc := newTestService(t, fake)
	ctx := context.Background()
	for i := 0; i < 2; i++ { // второй проход не должен создать дубль
		if err := svc.Scan(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var empty *Notification
	items, _ := svc.List(false)
	count := 0
	for i := range items {
		if items[i].Kind == KindEnvelopeEmpty {
			empty = &items[i]
			count++
		}
	}
	if count != 1 {
		t.Fatalf("got %d empty-envelope notifications, want 1", count)
	}
	if _, err := svc.Act(ctx, empty.ID, ActionBoost); err != nil {
		t.Fatalf("Act(boost) error = %v", err)
	}
	if got := fake.envelopes[2].AllocationPercent; got != 20+boostPoints {
		t.Fatalf("boosted envelope has %d%%, want %d%%", got, 20+boostPoints)
	}
	if got := fake.envelopes[0].AllocationPercent; got != 50-boostPoints {
		t.Fatalf("largest donor has %d%%, want %d%%", got, 50-boostPoints)
	}
}

func TestIncomeNotificationSkipsHistoryOnFirstScan(t *testing.T) {
	fake := &fakeStore{
		envelopes: []domain.Envelope{{ID: 1, Name: "А", AllocationPercent: 100, BalanceKopecks: 1000}},
		txs:       []domain.Transaction{{ID: 1, Type: domain.TransactionIncome, AmountKopecks: 1000}},
	}
	svc := newTestService(t, fake)
	ctx := context.Background()
	if err := svc.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	fake.txs = append([]domain.Transaction{{
		ID: 2, Type: domain.TransactionIncome, AmountKopecks: 1000000,
		Allocations: []domain.TransactionAllocation{{EnvelopeID: 1, EnvelopeName: "А", AmountKopecks: 1000000}},
	}}, fake.txs...)
	if err := svc.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	items, _ := svc.List(false)
	income := 0
	for _, n := range items {
		if n.Kind == KindIncome {
			income++
		}
	}
	if income != 1 {
		t.Fatalf("got %d income notifications, want exactly 1 (for the new transaction)", income)
	}
}
