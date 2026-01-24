package realtime

import "errors"

var (
	ErrSubscriptionLimit   = errors.New("subscription limit reached")
	ErrCollectionNotFound  = errors.New("collection not found")
	ErrInvalidFilter       = errors.New("invalid filter")
	ErrSubscriptionExists  = errors.New("subscription already exists")
	ErrSubscriptionMissing = errors.New("subscription not found")
)
