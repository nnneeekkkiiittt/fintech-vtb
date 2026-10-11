package domain

import (
	"errors"
	"testing"
)

func TestSplitIncomePreservesAmount(t *testing.T) {
	envelopes := []Envelope{
		{ID: 1, AllocationPercent: 40},
		{ID: 2, AllocationPercent: 25},
		{ID: 3, AllocationPercent: 35},
	}

	shares, err := SplitIncome(101, envelopes)
	if err != nil {
		t.Fatalf("SplitIncome() error = %v", err)
	}
	var total int64
	got := make(map[int64]int64)
	for _, share := range shares {
		total += share.AmountKopecks
		got[share.EnvelopeID] = share.AmountKopecks
	}
	if total != 101 {
		t.Fatalf("allocated %d kopecks, want 101", total)
	}
	if got[1] != 41 || got[2] != 25 || got[3] != 35 {
		t.Fatalf("unexpected distribution: %#v", got)
	}
}

func TestSplitIncomeOneKopeck(t *testing.T) {
	envelopes := []Envelope{
		{ID: 1, AllocationPercent: 40},
		{ID: 2, AllocationPercent: 25},
		{ID: 3, AllocationPercent: 35},
	}
	shares, err := SplitIncome(1, envelopes)
	if err != nil {
		t.Fatalf("SplitIncome() error = %v", err)
	}
	var total int64
	for _, share := range shares {
		total += share.AmountKopecks
	}
	if total != 1 {
		t.Fatalf("allocated %d kopecks, want 1", total)
	}
	if shares[0].EnvelopeID != 1 || shares[0].AmountKopecks != 1 {
		t.Fatalf("the extra kopeck should go to the largest fractional remainder: %#v", shares)
	}
}

func TestSplitIncomeRejectsPercentagesNotEqualTo100(t *testing.T) {
	_, err := SplitIncome(100, []Envelope{
		{ID: 1, AllocationPercent: 40},
		{ID: 2, AllocationPercent: 30},
	})
	if !errors.Is(err, ErrInvalidAllocation) {
		t.Fatalf("SplitIncome() error = %v, want ErrInvalidAllocation", err)
	}
}
