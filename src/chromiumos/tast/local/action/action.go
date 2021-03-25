// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package action provides the interface and utilities for funnctions which
// takes a context and returns an error on failure.
package action

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Action is a function that takes a context and returns an error.
type Action = func(context.Context) error

// NamedAction gives a name to an action. It logs when an action starts,
// and if the action fails, tells you the name of the failing action.
func NamedAction(name string, fn Action) Action {
	return func(ctx context.Context) error {
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
func Retry(n int, action Action) Action {
	return func(ctx context.Context) error {
		var err error
		for i := 0; i < n; i++ {
			if err = action(ctx); err == nil {
				return nil
			}
			testing.ContextLogf(ctx, "Retry failed attempt %d: %v", i+1, err)
		}
		return err
	}
}
