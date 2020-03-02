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
func Nested(ctx context.Context, name string, nestedSetup func(s *Setup) error) (CleanupCallback, error) {
	s, nestedCallback := New()
	callback := func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Cleaning up %q", name)
		if err := nestedCallback(ctx); err != nil {
			return errors.Wrapf(err, "nested cleanup %q failed", name)
		}
		return nil
	}
	succeeded := false
	defer func() {
		if !succeeded {
			callback(ctx)
		}
	}()
	testing.ContextLogf(ctx, "Setting up %q", name)
	if err := nestedSetup(s); err != nil {
		return nil, errors.Wrapf(err, "nested setup %q failed", name)
	}
	if err := s.Check(ctx); err != nil {
		return nil, errors.Wrapf(err, "nested setup %q failed", name)
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
			return errors.Wrapf(err, "cleanup for %d items failed, first failure", count)
		}
		return nil
	}
}

// cleanUp is a helper that runs all cleanup callbacks and logs any failures.
// Returns true if all cleanup
func (s *Setup) cleanUp(ctx context.Context) (errorCount int, firstError error) {
	for _, c := range s.callbacks {
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
		return errors.Wrapf(s.errs[0], "setup for %d items failed, first failure", len(s.errs))
	}
	return nil
}

// PowerTest configures a DUT to run a power test by disabling features that add
// noise, and consistently configuring components that change power draw.
func PowerTest(ctx context.Context) (CleanupCallback, error) {
	return Nested(ctx, "power test", func(s *Setup) error {
		s.Add(DisableService(ctx, "powerd"))
		s.Add(DisableService(ctx, "update-engine"))
		s.Add(DisableServiceIfExists(ctx, "vnc"))
		s.Add(DisableService(ctx, "dptf"))
		s.Add(SetBacklightLux(ctx, 150))
		s.Add(SetKeyboardBrightness(ctx, 24))
		s.Add(MuteAudio(ctx))
		s.Add(DisableWiFiInterfaces(ctx))
		s.Add(SetBatteryDischarge(ctx, 2.0))
		s.Add(DisableBluetooth(ctx))

		// TODO(b:150572487): Disable Night Light for power tests
		// TODO(b:150571709): Close all ChromeOS windows for power tests

		return nil
	})
}
