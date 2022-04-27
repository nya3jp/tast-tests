// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package action provides the interface and utilities for funnctions which
// takes a context and returns an error on failure.
package action

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Action is a function that takes a context and returns an error.
type Action = func(context.Context) error

// Named gives a name to an action. It logs when an action starts,
// and if the action fails, tells you the name of the failing action.
func Named(name string, fn Action) Action {
	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Start action %s", name)
		if err := fn(ctx); err != nil {
			return errors.Wrapf(err, "failed action %s", name)
		}
		return nil
	}
}

// Combine combines a list of functions from Context to error into one function.
// Combine adds the name of the operation into the error message to clarify the step.
// It is recommended to start the name of operations with a verb, e.g.,
//     "open Downloads and right click a folder"
// Then the failure msg would be like:
//     "failed to open Downloads and right click a folder on step ..."
func Combine(name string, steps ...Action) Action {
	return func(ctx context.Context) error {
		for i, f := range steps {
			if err := f(ctx); err != nil {
				return errors.Wrapf(err, "failed to %s on step %d", name, i+1)
			}
		}
		return nil
	}
}

// Retry returns a function that retries a given action if it returns error.
// The action will be executed up to n times, including the first attempt.
// The last error will be returned.  Any other errors will be silently logged.
// Between each run of the loop, it will sleep according the specified interval.
func Retry(n int, action Action, interval time.Duration) Action {
	return retry(n, action, interval, 1, true)
}

// RetrySilently returns a function that retries a given action if it returns error.
// The action will be executed up to n times, including the first attempt.
// The last error will be returned.  Any other errors will be ignored.
// Between each run of the loop, it will sleep according the specified interval.
func RetrySilently(n int, action Action, interval time.Duration) Action {
	return retry(n, action, interval, 1, false)
}

// RetryWithExponentialBackoff returns a function that retries a given action if it returns error.
// The action will be executed up to n times, including the first attempt.
// The last error will be returned.  Any other errors will be silently logged.
// Between each run of the loop, the interval increases exponentially.
// i.e. waiting time = interval * base ^ i where 0 <= i < n-1.
func RetryWithExponentialBackoff(n int, action Action, interval time.Duration, base int64) Action {
	return retry(n, action, interval, base, true)
}

func retry(n int, action Action, interval time.Duration, base int64, verboseLog bool) Action {
	return func(ctx context.Context) error {
		var err error
		backoff := interval
		for i := 0; i < n; i++ {
			if err = action(ctx); err == nil {
				// Print a success log to clear confusing.
				// Retry logs are sometimes mistaken as errors.
				if i > 0 && verboseLog {
					testing.ContextLogf(ctx, "Retry succeed in attempt %d", i+1)
				}
				return nil
			}
			if verboseLog {
				testing.ContextLogf(ctx, "Retry failed attempt %d: %v", i+1, err)
			}
			// Sleep between all iterations.
			if i < n-1 {
				if err := testing.Sleep(ctx, backoff); err != nil && verboseLog {
					testing.ContextLog(ctx, "Failed to sleep between retry iterations: ", err)
				}
				backoff *= time.Duration(base)
			}
		}
		return err
	}
}

// IfSuccessThen returns a function that runs action only if the first function succeeds.
// The function returns an error only if the preFunc succeeds but action fails,
// It returns nil in all other situations.
// Example:
//   dialog := nodewith.Name("Dialog").Role(role.Dialog)
//   button := nodewith.Name("Ok").Role(role.Button).Ancestor(dialog)
//   ui := uiauto.New(tconn)
//   if err := action.IfSuccessThen(ui.Withtimeout(5*time.Second).WaitUntilExists(dialog), ui.LeftClick(button))(ctx); err != nil {
//	    ...
//   }
func IfSuccessThen(preFunc, action Action) Action {
	return func(ctx context.Context) error {
		if err := preFunc(ctx); err == nil {
			if err := action(ctx); err != nil {
				return err
			}
		}
		return nil
	}
}

// Sleep returns a function that sleeps for the specified duration.
func Sleep(duration time.Duration) Action {
	return func(ctx context.Context) error {
		return testing.Sleep(ctx, duration)
	}
}
