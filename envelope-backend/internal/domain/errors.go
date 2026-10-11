package domain

import "errors"

var (
	ErrNotFound           = errors.New("resource not found")
	ErrInvalidAmount      = errors.New("amount must be positive")
	ErrInvalidAllocation  = errors.New("allocation percentages must total 100 and include every envelope exactly once")
	ErrInsufficientFunds  = errors.New("insufficient funds in envelope")
	ErrEnvelopeHasBalance = errors.New("cannot delete an envelope with a non-zero balance")
	ErrEnvelopeHasHistory = errors.New("cannot delete an envelope that has transaction history")
	ErrAllocationNotZero  = errors.New("set the envelope allocation to 0 before deleting it")
)
