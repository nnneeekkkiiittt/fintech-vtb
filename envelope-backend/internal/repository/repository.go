package repository

import (
	"context"

	"envelope-backend/internal/domain"
)

type Repository interface {
	Ping(ctx context.Context) error
	ListEnvelopes(ctx context.Context) ([]domain.Envelope, error)
	GetEnvelope(ctx context.Context, id int64) (domain.Envelope, error)
	CreateEnvelope(ctx context.Context, name, category string) (domain.Envelope, error)
	UpdateEnvelope(ctx context.Context, id int64, name, category string) (domain.Envelope, error)
	DeleteEnvelope(ctx context.Context, id int64) error
	SetAllocations(ctx context.Context, updates []domain.AllocationUpdate) ([]domain.Envelope, error)
	CreateIncome(ctx context.Context, amountKopecks int64, description string) (domain.Transaction, error)
	CreateExpense(ctx context.Context, envelopeID, amountKopecks int64, description string) (domain.Transaction, error)
	ListTransactions(ctx context.Context, limit, offset int) ([]domain.Transaction, error)
	GetTransaction(ctx context.Context, id int64) (domain.Transaction, error)
}
