package domain

import "time"

// Envelope is a virtual part of the user's bank account.
type Envelope struct {
	ID                int64     `json:"id"`
	Name              string    `json:"name"`
	Category          string    `json:"category"`
	AllocationPercent int       `json:"allocation_percent"`
	BalanceKopecks    int64     `json:"balance_kopecks"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type TransactionType string

const (
	TransactionIncome  TransactionType = "income"
	TransactionExpense TransactionType = "expense"
)

type Transaction struct {
	ID            int64                   `json:"id"`
	Type          TransactionType         `json:"type"`
	AmountKopecks int64                   `json:"amount_kopecks"`
	Description   string                  `json:"description"`
	CreatedAt     time.Time               `json:"created_at"`
	Allocations   []TransactionAllocation `json:"allocations"`
}

type TransactionAllocation struct {
	EnvelopeID    int64  `json:"envelope_id"`
	EnvelopeName  string `json:"envelope_name"`
	AmountKopecks int64  `json:"amount_kopecks"`
}

type AllocationUpdate struct {
	EnvelopeID int64 `json:"envelope_id" binding:"required,gt=0"`
	Percent    int   `json:"percent" binding:"gte=0,lte=100"`
}

// IncomeShare is the result of distributing one incoming payment among envelopes.
type IncomeShare struct {
	EnvelopeID    int64
	AmountKopecks int64
}
