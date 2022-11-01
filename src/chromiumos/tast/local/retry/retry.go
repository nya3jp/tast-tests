// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package retry

import (
	"chromiumos/tast/errors"
)

// Loop is a representation of retry loop state for a test.
type Loop struct {
	Attempts    int
	MaxAttempts int
	DoRetries   bool
	Fatalf      func(format string, args ...interface{})
	Logf        func(format string, args ...interface{})
}

// Exit ends the retry loop. This is used in places where the failure is in the core feature.
func (rl *Loop) Exit(desc string, err error) error {
	rl.Fatalf("Failed to %s: %v", desc, err)
	return nil
}

// RetryForAll retries the loop even if retries are disabled. This is used for unrelated failures.
func (rl *Loop) RetryForAll(desc string, err error) error {
	if rl.Attempts < rl.MaxAttempts {
		rl.Attempts++
		err = errors.Wrap(err, "failed to "+desc)
		rl.Logf("%s. Retrying", err)
		return err
	}
	return rl.Exit(desc, err)
}

// Retry retries the loop. This is used for temporary retires to stabilize the test.
func (rl *Loop) Retry(desc string, err error) error {
	if rl.DoRetries {
		return rl.RetryForAll(desc, err)
	}
	return rl.Exit(desc, err)
}
