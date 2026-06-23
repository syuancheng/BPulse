package bprecord

import "errors"

var (
	ErrInvalid       = errors.New("invalid blood pressure record")
	ErrNotFound      = errors.New("blood pressure record not found")
	ErrConflict      = errors.New("idempotency key conflict")
	ErrDecryptFailed = errors.New("blood pressure payload could not be decrypted")
)
