package domain

import "fmt"

// questionSpec описывает только структуру вопроса анкеты: сколько у него вариантов
// ответа и можно ли выбрать несколько. Тексты вопросов живут на фронтенде
// (fintech-vtb-frontend/src/lib/questionnaire.ts), поэтому при изменении числа
// вариантов или признака Multi нужно править оба файла.
type questionSpec struct {
	Options int
	Multi   bool
}

// questionSpecs — 15 вопросов анкеты по порядку (номер вопроса = индекс + 1).
var questionSpecs = []questionSpec{
	{Options: 7, Multi: false}, // 1
	{Options: 7, Multi: false}, // 2
	{Options: 5, Multi: false}, // 3
	{Options: 11, Multi: true}, // 4
	{Options: 6, Multi: false}, // 5
	{Options: 5, Multi: true},  // 6
	{Options: 12, Multi: true}, // 7
	{Options: 5, Multi: false}, // 8
	{Options: 10, Multi: true}, // 9
	{Options: 6, Multi: false}, // 10
	{Options: 5, Multi: false}, // 11
	{Options: 11, Multi: true}, // 12
	{Options: 6, Multi: false}, // 13
	{Options: 7, Multi: false}, // 14
	{Options: 5, Multi: false}, // 15
}

// QuestionCount возвращает число вопросов анкеты.
func QuestionCount() int {
	return len(questionSpecs)
}

// ValidateAnswers проверяет, что анкета заполнена полностью и корректно:
// на каждый вопрос есть ответ, у вопроса с одним ответом выбран ровно один вариант,
// номера вариантов существуют и не повторяются.
func ValidateAnswers(a Answers) error {
	for question := range a {
		if question < 1 || question > len(questionSpecs) {
			return fmt.Errorf("%w: unknown question %d", ErrInvalidAnswers, question)
		}
	}

	for i, spec := range questionSpecs {
		number := i + 1
		selected := a[number]
		if len(selected) == 0 {
			return fmt.Errorf("%w: question %d is not answered", ErrInvalidAnswers, number)
		}
		if !spec.Multi && len(selected) != 1 {
			return fmt.Errorf("%w: question %d accepts exactly one option", ErrInvalidAnswers, number)
		}

		seen := make(map[int]struct{}, len(selected))
		for _, option := range selected {
			if option < 1 || option > spec.Options {
				return fmt.Errorf("%w: question %d has no option %d", ErrInvalidAnswers, number, option)
			}
			if _, duplicate := seen[option]; duplicate {
				return fmt.Errorf("%w: question %d has option %d twice", ErrInvalidAnswers, number, option)
			}
			seen[option] = struct{}{}
		}
	}
	return nil
}
