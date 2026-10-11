package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"envelope-backend/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

func (p *Postgres) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

func (p *Postgres) ListEnvelopes(ctx context.Context) ([]domain.Envelope, error) {
	rows, err := p.pool.Query(ctx, envelopeSelect+` ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list envelopes: %w", err)
	}
	defer rows.Close()

	envelopes := make([]domain.Envelope, 0)
	for rows.Next() {
		var envelope domain.Envelope
		if err := scanEnvelope(rows, &envelope); err != nil {
			return nil, fmt.Errorf("scan envelope: %w", err)
		}
		envelopes = append(envelopes, envelope)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate envelopes: %w", err)
	}
	return envelopes, nil
}

func (p *Postgres) GetEnvelope(ctx context.Context, id int64) (domain.Envelope, error) {
	var envelope domain.Envelope
	err := scanEnvelope(p.pool.QueryRow(ctx, envelopeSelect+` WHERE id = $1`, id), &envelope)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Envelope{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Envelope{}, fmt.Errorf("get envelope: %w", err)
	}
	return envelope, nil
}

const envelopeSelect = `SELECT id, name, category, allocation_percent, balance_kopecks, created_at, updated_at FROM envelopes`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEnvelope(row rowScanner, envelope *domain.Envelope) error {
	return row.Scan(&envelope.ID, &envelope.Name, &envelope.Category, &envelope.AllocationPercent,
		&envelope.BalanceKopecks, &envelope.CreatedAt, &envelope.UpdatedAt)
}

func (p *Postgres) CreateEnvelope(ctx context.Context, name, category string) (domain.Envelope, error) {
	var envelope domain.Envelope
	err := scanEnvelope(p.pool.QueryRow(ctx, `
		INSERT INTO envelopes (name, category) VALUES ($1, $2)
		RETURNING id, name, category, allocation_percent, balance_kopecks, created_at, updated_at`, name, category), &envelope)
	if err != nil {
		return domain.Envelope{}, fmt.Errorf("create envelope: %w", err)
	}
	return envelope, nil
}

func (p *Postgres) UpdateEnvelope(ctx context.Context, id int64, name, category string) (domain.Envelope, error) {
	var envelope domain.Envelope
	err := scanEnvelope(p.pool.QueryRow(ctx, `
		UPDATE envelopes SET name = $2, category = $3, updated_at = NOW()
		WHERE id = $1
		RETURNING id, name, category, allocation_percent, balance_kopecks, created_at, updated_at`, id, name, category), &envelope)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Envelope{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Envelope{}, fmt.Errorf("update envelope: %w", err)
	}
	return envelope, nil
}

func (p *Postgres) DeleteEnvelope(ctx context.Context, id int64) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete envelope: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var balance int64
	var percent int
	err = tx.QueryRow(ctx, `SELECT balance_kopecks, allocation_percent FROM envelopes WHERE id = $1 FOR UPDATE`, id).Scan(&balance, &percent)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read envelope before delete: %w", err)
	}
	if balance != 0 {
		return domain.ErrEnvelopeHasBalance
	}
	if percent != 0 {
		return domain.ErrAllocationNotZero
	}
	var hasHistory bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM transaction_allocations WHERE envelope_id = $1)`, id).Scan(&hasHistory); err != nil {
		return fmt.Errorf("check envelope history: %w", err)
	}
	if hasHistory {
		return domain.ErrEnvelopeHasHistory
	}
	if _, err := tx.Exec(ctx, `DELETE FROM envelopes WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete envelope: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete envelope: %w", err)
	}
	return nil
}

func (p *Postgres) SetAllocations(ctx context.Context, updates []domain.AllocationUpdate) ([]domain.Envelope, error) {
	if len(updates) == 0 {
		return nil, domain.ErrInvalidAllocation
	}

	provided := make(map[int64]int, len(updates))
	total := 0
	for _, update := range updates {
		if update.EnvelopeID <= 0 || update.Percent < 0 || update.Percent > 100 {
			return nil, domain.ErrInvalidAllocation
		}
		if _, exists := provided[update.EnvelopeID]; exists {
			return nil, domain.ErrInvalidAllocation
		}
		provided[update.EnvelopeID] = update.Percent
		total += update.Percent
	}
	if total != 100 {
		return nil, domain.ErrInvalidAllocation
	}

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin allocation update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `SELECT id FROM envelopes ORDER BY id FOR UPDATE`)
	if err != nil {
		return nil, fmt.Errorf("load envelopes for allocation update: %w", err)
	}
	currentIDs := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan envelope id: %w", err)
		}
		currentIDs = append(currentIDs, id)
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil {
		return nil, fmt.Errorf("iterate envelope ids: %w", rowsErr)
	}
	if len(currentIDs) != len(provided) {
		return nil, domain.ErrInvalidAllocation
	}
	for _, id := range currentIDs {
		if _, exists := provided[id]; !exists {
			return nil, domain.ErrInvalidAllocation
		}
	}

	for id, percent := range provided {
		if _, err := tx.Exec(ctx, `UPDATE envelopes SET allocation_percent = $2, updated_at = NOW() WHERE id = $1`, id, percent); err != nil {
			return nil, fmt.Errorf("update envelope allocation: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit allocation update: %w", err)
	}
	return p.ListEnvelopes(ctx)
}

func (p *Postgres) CreateIncome(ctx context.Context, amountKopecks int64, description string) (domain.Transaction, error) {
	if amountKopecks <= 0 {
		return domain.Transaction{}, domain.ErrInvalidAmount
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("begin income: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, envelopeSelect+` ORDER BY id FOR UPDATE`)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("load envelopes for income: %w", err)
	}
	envelopes := make([]domain.Envelope, 0)
	for rows.Next() {
		var envelope domain.Envelope
		if err := scanEnvelope(rows, &envelope); err != nil {
			rows.Close()
			return domain.Transaction{}, fmt.Errorf("scan envelope for income: %w", err)
		}
		envelopes = append(envelopes, envelope)
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil {
		return domain.Transaction{}, fmt.Errorf("iterate envelopes for income: %w", rowsErr)
	}

	shares, err := domain.SplitIncome(amountKopecks, envelopes)
	if err != nil {
		return domain.Transaction{}, err
	}
	envelopeNames := make(map[int64]string, len(envelopes))
	for _, envelope := range envelopes {
		envelopeNames[envelope.ID] = envelope.Name
	}
	var operation domain.Transaction
	err = tx.QueryRow(ctx, `
		INSERT INTO transactions (type, amount_kopecks, description)
		VALUES ('income', $1, $2)
		RETURNING id, type, amount_kopecks, description, created_at`, amountKopecks, cleanDescription(description)).
		Scan(&operation.ID, &operation.Type, &operation.AmountKopecks, &operation.Description, &operation.CreatedAt)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("insert income transaction: %w", err)
	}

	operation.Allocations = make([]domain.TransactionAllocation, 0, len(shares))
	for _, share := range shares {
		if share.AmountKopecks == 0 {
			continue
		}
		if _, err := tx.Exec(ctx, `UPDATE envelopes SET balance_kopecks = balance_kopecks + $2, updated_at = NOW() WHERE id = $1`, share.EnvelopeID, share.AmountKopecks); err != nil {
			return domain.Transaction{}, fmt.Errorf("credit envelope: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO transaction_allocations (transaction_id, envelope_id, amount_kopecks) VALUES ($1, $2, $3)`, operation.ID, share.EnvelopeID, share.AmountKopecks); err != nil {
			return domain.Transaction{}, fmt.Errorf("record income allocation: %w", err)
		}
		operation.Allocations = append(operation.Allocations, domain.TransactionAllocation{
			EnvelopeID: share.EnvelopeID, EnvelopeName: envelopeNames[share.EnvelopeID], AmountKopecks: share.AmountKopecks,
		})
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Transaction{}, fmt.Errorf("commit income transaction: %w", err)
	}
	return operation, nil
}

func (p *Postgres) CreateExpense(ctx context.Context, envelopeID, amountKopecks int64, description string) (domain.Transaction, error) {
	if amountKopecks <= 0 {
		return domain.Transaction{}, domain.ErrInvalidAmount
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("begin expense: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var balance int64
	var envelopeName string
	err = tx.QueryRow(ctx, `SELECT name, balance_kopecks FROM envelopes WHERE id = $1 FOR UPDATE`, envelopeID).Scan(&envelopeName, &balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Transaction{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("read envelope balance: %w", err)
	}
	if balance < amountKopecks {
		return domain.Transaction{}, domain.ErrInsufficientFunds
	}

	var operation domain.Transaction
	err = tx.QueryRow(ctx, `
		INSERT INTO transactions (type, amount_kopecks, description)
		VALUES ('expense', $1, $2)
		RETURNING id, type, amount_kopecks, description, created_at`, amountKopecks, cleanDescription(description)).
		Scan(&operation.ID, &operation.Type, &operation.AmountKopecks, &operation.Description, &operation.CreatedAt)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("insert expense transaction: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE envelopes SET balance_kopecks = balance_kopecks - $2, updated_at = NOW() WHERE id = $1`, envelopeID, amountKopecks); err != nil {
		return domain.Transaction{}, fmt.Errorf("debit envelope: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO transaction_allocations (transaction_id, envelope_id, amount_kopecks) VALUES ($1, $2, $3)`, operation.ID, envelopeID, amountKopecks); err != nil {
		return domain.Transaction{}, fmt.Errorf("record expense allocation: %w", err)
	}
	operation.Allocations = []domain.TransactionAllocation{{
		EnvelopeID: envelopeID, EnvelopeName: envelopeName, AmountKopecks: amountKopecks,
	}}
	if err := tx.Commit(ctx); err != nil {
		return domain.Transaction{}, fmt.Errorf("commit expense transaction: %w", err)
	}
	return operation, nil
}

func cleanDescription(description string) string {
	return strings.TrimSpace(description)
}

func (p *Postgres) ListTransactions(ctx context.Context, limit, offset int) ([]domain.Transaction, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, type, amount_kopecks, description, created_at
		FROM transactions ORDER BY created_at DESC, id DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	transactions := make([]domain.Transaction, 0)
	ids := make([]int64, 0)
	for rows.Next() {
		var operation domain.Transaction
		if err := rows.Scan(&operation.ID, &operation.Type, &operation.AmountKopecks, &operation.Description, &operation.CreatedAt); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		operation.Allocations = make([]domain.TransactionAllocation, 0)
		transactions = append(transactions, operation)
		ids = append(ids, operation.ID)
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil {
		return nil, fmt.Errorf("iterate transactions: %w", rowsErr)
	}
	if len(ids) == 0 {
		return transactions, nil
	}
	byID := make(map[int64][]domain.TransactionAllocation, len(ids))
	// Load allocations in one query and group them by transaction ID.
	rows2, err := p.pool.Query(ctx, `
		SELECT ta.transaction_id, ta.envelope_id, e.name, ta.amount_kopecks
		FROM transaction_allocations ta JOIN envelopes e ON e.id = ta.envelope_id
		WHERE ta.transaction_id = ANY($1::bigint[]) ORDER BY ta.id`, ids)
	if err != nil {
		return nil, fmt.Errorf("load transaction allocations: %w", err)
	}
	defer rows2.Close()
	for rows2.Next() {
		var transactionID int64
		var allocation domain.TransactionAllocation
		if err := rows2.Scan(&transactionID, &allocation.EnvelopeID, &allocation.EnvelopeName, &allocation.AmountKopecks); err != nil {
			return nil, fmt.Errorf("scan transaction allocation: %w", err)
		}
		byID[transactionID] = append(byID[transactionID], allocation)
	}
	if err := rows2.Err(); err != nil {
		return nil, fmt.Errorf("iterate transaction allocations: %w", err)
	}
	for i := range transactions {
		transactions[i].Allocations = byID[transactions[i].ID]
	}
	return transactions, nil
}

func (p *Postgres) GetTransaction(ctx context.Context, id int64) (domain.Transaction, error) {
	var operation domain.Transaction
	err := p.pool.QueryRow(ctx, `
		SELECT id, type, amount_kopecks, description, created_at FROM transactions WHERE id = $1`, id).
		Scan(&operation.ID, &operation.Type, &operation.AmountKopecks, &operation.Description, &operation.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Transaction{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("get transaction: %w", err)
	}
	operation.Allocations, err = p.loadTransactionAllocations(ctx, id)
	if err != nil {
		return domain.Transaction{}, err
	}
	return operation, nil
}

func (p *Postgres) loadTransactionAllocations(ctx context.Context, singleID int64) ([]domain.TransactionAllocation, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT e.id, e.name, ta.amount_kopecks
		FROM transaction_allocations ta JOIN envelopes e ON e.id = ta.envelope_id
		WHERE ta.transaction_id = $1 ORDER BY ta.id`, singleID)
	if err != nil {
		return nil, fmt.Errorf("query transaction allocations: %w", err)
	}
	defer rows.Close()
	allocations := make([]domain.TransactionAllocation, 0)
	for rows.Next() {
		var allocation domain.TransactionAllocation
		if err := rows.Scan(&allocation.EnvelopeID, &allocation.EnvelopeName, &allocation.AmountKopecks); err != nil {
			return nil, fmt.Errorf("scan transaction allocation: %w", err)
		}
		allocations = append(allocations, allocation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate transaction allocations: %w", err)
	}
	return allocations, nil
}
