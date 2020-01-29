// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package setup contains helpers to set up a DUT for a power test.
package setup

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// CleanupCallback cleans up a single setup item.
type CleanupCallback func(context.Context) error

// Nested is used by setup items that have multiple stages that need separate
// cleanup callbacks.
func Nested(ctx context.Context, nestedSetup func(s *Setup) error) (CleanupCallback, error) {
	s, callback := New()
	succeeded := false
	defer func() {
		if !succeeded {
			callback(ctx)
		}
	}()
	if err := nestedSetup(s); err != nil {
		return nil, err
	}
	if err := s.Check(ctx); err != nil {
		return nil, errors.Wrap(err, "setup for nested items failed")
	}
	succeeded = true
	return callback, nil
}

// Setup accumulates the results of setup items so that their results can be
// checked, errors logged, and cleaned up.
type Setup struct {
	callbacks []CleanupCallback
	errs      []error
}

// New creates a Setup object to collect the results of setup items, and a
// cleanup function that should be immediately deferred to make sure cleanup
// callbacks are called.
func New() (*Setup, CleanupCallback) {
	s := &Setup{}
	cleanedUp := false
	return s, func(ctx context.Context) error {
		if cleanedUp {
			return errors.New("cleanup has already been called")
		}
		cleanedUp = true

		if count, err := s.cleanUp(ctx); err != nil {
			return errors.Wrapf(err, "cleanup for %d items failed", count)
		}
		return nil
	}
}

// cleanUp is a helper that runs all cleanup callbacks and logs any failures.
// Returns true if all cleanup
func (s *Setup) cleanUp(ctx context.Context) (errorCount int, firstError error) {
	for _, c := range s.callbacks {
		// Defer cleanup calls so that if any of them panic, the rest still run.
		defer func(callback CleanupCallback) {
			if err := callback(ctx); err != nil {
				errorCount++
				if firstError == nil {
					firstError = err
				}
				testing.ContextLog(ctx, "Cleanup failed: ", err)
			}
		}(c)
	}
	return 0, nil
}

// Add adds a result to be checked or cleaned up later.
func (s *Setup) Add(callback CleanupCallback, err error) {
	if callback != nil {
		s.callbacks = append(s.callbacks, callback)
	}
	if err != nil {
		s.errs = append(s.errs, err)
	}
}

// Check checks if any Result shows a failure happened. All failures are logged,
// and a summary of failures is returned.
func (s *Setup) Check(ctx context.Context) error {
	for _, err := range s.errs {
		testing.ContextLog(ctx, "Setup failed: ", err)
	}
	if len(s.errs) > 0 {
		return errors.Wrapf(s.errs[0], "setup for %d items failed", len(s.errs))
	}
	return nil
}
