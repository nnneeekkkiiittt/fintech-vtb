package domain

import (
	"sort"
)

// SplitIncome distributes a positive amount in kopecks using the largest-remainder
// method. The result always adds up to the original amount, even when percentages
// do not divide the amount evenly into whole kopecks.
func SplitIncome(amountKopecks int64, envelopes []Envelope) ([]IncomeShare, error) {
	if amountKopecks <= 0 {
		return nil, ErrInvalidAmount
	}
	if len(envelopes) == 0 {
		return nil, ErrInvalidAllocation
	}
	seen := make(map[int64]struct{}, len(envelopes))
	totalPercent := 0
	shares := make([]IncomeShare, 0, len(envelopes))
	remainders := make([]int64, len(envelopes))
	var allocated int64

	for i, envelope := range envelopes {
		if envelope.ID <= 0 || envelope.AllocationPercent < 0 || envelope.AllocationPercent > 100 {
			return nil, ErrInvalidAllocation
		}
		if _, exists := seen[envelope.ID]; exists {
			return nil, ErrInvalidAllocation
		}
		seen[envelope.ID] = struct{}{}
		totalPercent += envelope.AllocationPercent
		if totalPercent > 100 {
			return nil, ErrInvalidAllocation
		}

		// Split amount into quotient and remainder first to avoid amount*percent
		// overflowing int64 for unusually large values.
		percent := int64(envelope.AllocationPercent)
		base := (amountKopecks/100)*percent + ((amountKopecks%100)*percent)/100
		remainder := ((amountKopecks % 100) * percent) % 100
		shares = append(shares, IncomeShare{EnvelopeID: envelope.ID, AmountKopecks: base})
		remainders[i] = remainder
		allocated += base
	}

	if totalPercent != 100 {
		return nil, ErrInvalidAllocation
	}

	left := amountKopecks - allocated
	order := make([]int, len(envelopes))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		leftIndex, rightIndex := order[i], order[j]
		if remainders[leftIndex] == remainders[rightIndex] {
			return envelopes[leftIndex].ID < envelopes[rightIndex].ID
		}
		return remainders[leftIndex] > remainders[rightIndex]
	})
	for i := int64(0); i < left; i++ {
		shares[order[int(i)%len(order)]].AmountKopecks++
	}

	return shares, nil
}
