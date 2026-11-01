package domain

import (
	"errors"
	"testing"
)

// validAnswers возвращает корректную анкету: в каждом вопросе выбран первый вариант.
func validAnswers() Answers {
	answers := Answers{}
	for number := 1; number <= QuestionCount(); number++ {
		answers[number] = []int{1}
	}
	return answers
}

func findEnvelope(items []RecommendedEnvelope, name string) (RecommendedEnvelope, bool) {
	for _, item := range items {
		if item.Name == name {
			return item, true
		}
	}
	return RecommendedEnvelope{}, false
}

func totalPercent(items []RecommendedEnvelope) int {
	total := 0
	for _, item := range items {
		total += item.AllocationPercent
	}
	return total
}

func TestBuildRecommendationBaseEnvelopesOnly(t *testing.T) {
	items := BuildRecommendation(Answers{})

	if len(items) != 2 {
		t.Fatalf("expected 2 base envelopes, got %d", len(items))
	}
	// Веса 5 и 2 из 7: 71.43% и 28.57%, остаток достаётся второму конверту.
	if items[0].Name != envGroceries || items[0].Priority != 1 || items[0].AllocationPercent != 71 {
		t.Fatalf("unexpected first envelope: %+v", items[0])
	}
	if items[1].Name != envFree || items[1].Priority != 4 || items[1].AllocationPercent != 29 {
		t.Fatalf("unexpected second envelope: %+v", items[1])
	}
}

func TestBuildRecommendationOverspendRaisesPriority(t *testing.T) {
	items := BuildRecommendation(Answers{7: {2}, 9: {2}})

	leisure, ok := findEnvelope(items, envLeisure)
	if !ok {
		t.Fatal("expected the leisure envelope to be recommended")
	}
	// Приоритет 3 из-за вопроса 7 и ещё минус один из-за перерасхода в вопросе 9.
	if leisure.Priority != 2 {
		t.Fatalf("expected priority 2 after overspend, got %d", leisure.Priority)
	}

	want := []int{46, 36, 18}
	for i, percent := range want {
		if items[i].AllocationPercent != percent {
			t.Fatalf("envelope %d (%s): expected %d%%, got %d%%", i, items[i].Name, percent, items[i].AllocationPercent)
		}
	}
}

func TestBuildRecommendationOverspendDoesNotCreateEnvelope(t *testing.T) {
	items := BuildRecommendation(Answers{9: {2, 3, 5}})

	for _, name := range []string{envLeisure, envTransport, envShopping} {
		if _, ok := findEnvelope(items, name); ok {
			t.Fatalf("overspend answer must not create envelope %q", name)
		}
	}
}

func TestBuildRecommendationReserveAndGoals(t *testing.T) {
	tests := []struct {
		name         string
		answers      Answers
		wantReserve  bool
		wantGoals    bool
		reservePrior int
	}{
		{"reserve is very important", Answers{15: {1}}, true, false, 2},
		{"reserve already exists", Answers{15: {4}}, false, false, 0},
		{"goal selected", Answers{12: {3}}, false, true, 0},
		{"no concrete goal", Answers{12: {11}}, false, false, 0},
		{"emergency fund goal", Answers{12: {1, 3}}, true, true, 2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			items := BuildRecommendation(test.answers)
			reserve, hasReserve := findEnvelope(items, envReserve)
			_, hasGoals := findEnvelope(items, envGoals)
			if hasReserve != test.wantReserve || hasGoals != test.wantGoals {
				t.Fatalf("reserve=%v goals=%v, want reserve=%v goals=%v", hasReserve, hasGoals, test.wantReserve, test.wantGoals)
			}
			if hasReserve && reserve.Priority != test.reservePrior {
				t.Fatalf("expected reserve priority %d, got %d", test.reservePrior, reserve.Priority)
			}
		})
	}
}

func TestBuildRecommendationPercentsAlwaysTotal100(t *testing.T) {
	everything := Answers{}
	for number, spec := range questionSpecs {
		if spec.Multi {
			for option := 1; option <= spec.Options; option++ {
				everything[number+1] = append(everything[number+1], option)
			}
		} else {
			everything[number+1] = []int{1}
		}
	}

	cases := map[string]Answers{
		"empty":      {},
		"first only": validAnswers(),
		"everything": everything,
	}
	for name, answers := range cases {
		t.Run(name, func(t *testing.T) {
			items := BuildRecommendation(answers)
			if got := totalPercent(items); got != 100 {
				t.Fatalf("percents add up to %d, want 100", got)
			}
			seen := map[string]bool{}
			for _, item := range items {
				if item.AllocationPercent < 1 {
					t.Fatalf("envelope %q got %d%%", item.Name, item.AllocationPercent)
				}
				if item.Category == "" || len(item.Category) > 40 {
					t.Fatalf("envelope %q has invalid category %q", item.Name, item.Category)
				}
				if seen[item.Name] {
					t.Fatalf("envelope %q recommended twice", item.Name)
				}
				seen[item.Name] = true
			}
		})
	}
}

func TestBuildRecommendationSortedByPriority(t *testing.T) {
	items := BuildRecommendation(Answers{4: {1, 5}, 7: {2, 6, 9}, 12: {1}})
	for i := 1; i < len(items); i++ {
		if items[i-1].Priority > items[i].Priority {
			t.Fatalf("envelopes are not sorted by priority: %+v", items)
		}
	}
}

func TestValidateAnswers(t *testing.T) {
	if err := ValidateAnswers(validAnswers()); err != nil {
		t.Fatalf("valid answers rejected: %v", err)
	}

	withMulti := validAnswers()
	withMulti[4] = []int{1, 3, 5}
	if err := ValidateAnswers(withMulti); err != nil {
		t.Fatalf("several options in a multi-choice question rejected: %v", err)
	}

	broken := map[string]func(Answers){
		"question not answered":      func(a Answers) { delete(a, 3) },
		"empty answer":               func(a Answers) { a[3] = []int{} },
		"two options in single":      func(a Answers) { a[1] = []int{1, 2} },
		"option out of range":        func(a Answers) { a[3] = []int{99} },
		"option zero":                func(a Answers) { a[3] = []int{0} },
		"unknown question":           func(a Answers) { a[16] = []int{1} },
		"duplicate option in multi":  func(a Answers) { a[4] = []int{2, 2} },
		"option out of range, multi": func(a Answers) { a[4] = []int{12} },
	}
	for name, mutate := range broken {
		t.Run(name, func(t *testing.T) {
			answers := validAnswers()
			mutate(answers)
			if err := ValidateAnswers(answers); !errors.Is(err, ErrInvalidAnswers) {
				t.Fatalf("expected ErrInvalidAnswers, got %v", err)
			}
		})
	}
}
