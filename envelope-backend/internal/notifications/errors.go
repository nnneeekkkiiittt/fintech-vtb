package notifications

import "errors"

var (
	ErrNotificationNotFound = errors.New("notification not found")
	ErrUnknownAction        = errors.New("unknown notification action")
	ErrAlreadyResolved      = errors.New("notification is already resolved")
	ErrInvalidAnswers       = errors.New("invalid survey answers")
	ErrNothingToMove        = errors.New("no envelope can give up part of its allocation")
)
