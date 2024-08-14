package retry

import (
	"fmt"

	"github.com/cenkalti/backoff/v4"
)

var (
	_ operation
	_ backOffContext
)

type (
	operation      func() error // for generate mocks
	backOffContext interface {  // for generate mocks
		backoff.BackOffContext
	}
)

// Retry
//
// Returns error joined with previous error.
// We need this because backoff.Retry can return error like context.DeadlineExceeded
// without any information about previous error.
func Retry(o backoff.Operation, b backoff.BackOffContext) (err error) {
	var prevErr error
	err = backoff.Retry(func() error {
		prevErr = err
		err = o()
		return err
	}, b)
	if err != nil {
		if prevErr != nil {
			return fmt.Errorf("%w; previous error: %w", err, prevErr)
		}
		return err
	}

	return nil
}
