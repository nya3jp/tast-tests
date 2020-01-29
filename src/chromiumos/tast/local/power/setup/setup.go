// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package setup contains helpers to set up a DUT for a power test.
package setup

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// CleanupCallback cleans up a single setup item.
type CleanupCallback func(context.Context) error

// Result stores the result of a single setup item for later logging and
// error reporting.
type Result struct {
	callback CleanupCallback
	err      error
}

// ResultSucceeded is used by setup items to report a success.
func ResultSucceeded(callback CleanupCallback) Result {
	return Result{
		callback: callback,
		err:      nil,
	}
}

// ResultFailed is used by setup items to report a failure.
func ResultFailed(err error) Result {
	return Result{
		callback: nil,
		err:      err,
	}
}

// ResultNoCleanup is used by setup items to report that no error
// happened and no cleanup is necessary.
func ResultNoCleanup() Result {
	return Result{
		callback: nil,
		err:      nil,
	}
}

// ResultNested is used by setup items that have multiple stages that need
// separate cleanup callbacks.
func ResultNested(ctx context.Context, nestedSetup func(s *Setup) error) Result {
	s, callback := NewSetup()
	succeeded := false
	defer func() {
		if !succeeded {
			callback(ctx)
		}
	}()
	if err := nestedSetup(s); err != nil {
		return ResultFailed(err)
	}
	if err := s.CheckAndLog(ctx); err != nil {
		return ResultFailed(errors.Wrap(err, "setup for nested items failed"))
	}
	succeeded = true
	return ResultSucceeded(callback)
}

// Setup accumulates the results of setup items so that their results can be
// checked, errors logged, and cleaned up.
type Setup struct {
	results []Result
}

// NewSetup creates a Setup object to collect the results of setup items, and a
// cleanup function that should be immediately deferred to make sure cleanup
// callbacks are called.
func NewSetup() (*Setup, CleanupCallback) {
	s := &Setup{
		results: []Result{},
	}
	cleanedUp := false
	return s, func(ctx context.Context) error {
		failed := false
		if cleanedUp {
			return errors.New("cleanup has already been called")
		}
		cleanedUp = true
		s.cleanupAndLog(ctx, &failed)
		if failed {
			return errors.New("cleanup some items failed, see logs")
		}
		return nil
	}
}

// cleanupAndLog is a helper that runs all cleanup callbacks and logs any
// failures. Whether any failures ocurred is returned via reference so that it
// can be updated by deferred calls, guaranteeing that each callback is run.
func (s *Setup) cleanupAndLog(ctx context.Context, failed *bool) {
	*failed = false
	for _, r := range s.results {
		if r.callback == nil {
			continue
		}
		defer func(callback CleanupCallback) {
			if err := callback(ctx); err != nil {
				*failed = true
				testing.ContextLog(ctx, "Cleanup failed: ", err)
			}
		}(r.callback)
	}
}

// Add adds a result to be checked or cleaned up later.
func (s *Setup) Add(result Result) {
	s.results = append(s.results, result)
}

// CheckAndLog checks if any Result shows a failure happened. All failures
// are logged, and a summary of failures is returned.
func (s *Setup) CheckAndLog(ctx context.Context) error {
	failed := false
	for _, r := range s.results {
		if r.err != nil {
			failed = true
			testing.ContextLog(ctx, "Setup failed: ", r.err)
		}
	}
	if failed {
		return errors.New("setup for some items failed, see logs")
	}
	return nil
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
