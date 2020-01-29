// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Example of a setup method that uses CleanupChain:
// 	func SomeSetup(ctx context.Context, <<< Arguments Go Here >>> chain CleanupChain) (CleanupChain, error) {
//		setupFailed, guard := SetupFailureGuard(chain)
//		defer guard(ctx)
//
//		<<< Setup Code Goes Here >>>
//
//		return SetupSucceeded(setupFailed, chain, func(ctx context.Context) error {
//
//			<<< Cleanup Code Goes Here >>>
//
//			return nil
//		})
//	}

// CleanupChain is a function that calls all necessary CleanupItems.
// CleanupChains can be composed, so they use a passed error reference to
// report failures so that multiple deferred calls can all report errors.
type CleanupChain func(context.Context, *error)

// CleanupItem is a callback that cleans up one thing.
type CleanupItem func(context.Context) error

// cleanupGuard is used to call previous cleanup items if there is a failure
// during setup.
type cleanupGuard func(context.Context)

// NewCleanupChain creates a CleanupChain, which can then be extended with work
// items from different setup functions.
func NewCleanupChain() CleanupChain {
	count := 0
	return func(ctx context.Context, errRef *error) {
		if count != 0 {
			// CleanupChains migth be called multiple times if a setup function
			// forgets to call SetupUnnecessary or SetupSuccessful when
			// returning without an error.
			// Look for cleanup log items in the setup phase of the logs.
			err := errors.New("cleanup failed, CleanupChain called multiple times")
			if *errRef != nil {
				*errRef = err
			} else {
				testing.ContextLog(ctx, "Cleanup failed multiple times: ", err)
			}
		}
		count++
	}
}

// SetupFailureGuard creates a closure that will clean up any previously
// created CleanupItems. A bool reference is also returned that can be used
// to disable this cleanup.
func SetupFailureGuard(chain CleanupChain) (*bool, cleanupGuard) {
	setupFailed := true
	return &setupFailed, func(ctx context.Context) {
		if setupFailed {
			if err := RunCleanupChain(ctx, chain); err != nil {
				testing.ContextLog(ctx, "Cleanup failed while rolling back setup failure: ", err)
			}
		}
	}
}

// SetupSucceeded cancels a failure guard cleanup, and appends another
// CleanupItem to the CleanupChain.
func SetupSucceeded(setupFailed *bool, chain CleanupChain, item CleanupItem) (CleanupChain, error) {
	*setupFailed = false
	return func(ctx context.Context, errRef *error) {
		defer func(ctx context.Context) {
			chainErr := RunCleanupChain(ctx, chain)
			if *errRef != nil && chainErr != nil {
				testing.ContextLog(ctx, "Cleanup failed multiple times: ", chainErr)
				return
			}
			*errRef = chainErr
		}(ctx)
		*errRef = item(ctx)
	}, nil
}

// SetupUnnecessary cancels a failure guard cleanup, and doesn't modify the
// passed CleanupChain. Used when a setup action doesn't need to do any setup
// or cleanup.
func SetupUnnecessary(setupFailed *bool, chain CleanupChain) (CleanupChain, error) {
	*setupFailed = false
	return chain, nil
}

// RunCleanupChain executes all CleanupItems on a CleanupChain.
func RunCleanupChain(ctx context.Context, chain CleanupChain) error {
	// TODO: extend the context, so if we fail because we exceeded the deadline, we still have a bit of time to clean up
	var err error
	chain(ctx, &err)
	return err
}

// ShortenDeadline returns a new context.Context with a deadline that is
// shorter by dt. Used by tests to create a shorter deadline to run under so
// that cleanup has time to run if the test times out.
func ShortenDeadline(ctx context.Context, dt time.Duration) context.Context {
	deadline, ok := ctx.Deadline()
	if !ok {
		// There is no deadline, so no need to shorten.
		return ctx
	}
	shorter, _ := context.WithDeadline(ctx, deadline.Add(-dt))
	return shorter
}
