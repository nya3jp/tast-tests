// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package updateengine provides ways to interact with update_engine daemon and utilities.
package updateengine

import (
	"context"
	"os"

	"chromiumos/tast/errors"
)

// OOBE releated constants.
const (
	oobeCompletedFlag     = "/home/chronos/.oobe_completed"
	oobeCompletedFlagPerm = 0644
)

// MarkOobeCompletion will fake being OOBE complete, so update-engine won't
// block update checks.
func MarkOobeCompletion(ctx context.Context) error {
	if _, err := os.OpenFile(oobeCompletedFlag, os.O_RDONLY|os.O_CREATE, oobeCompletedFlagPerm); err != nil {
		return errors.Wrap(err, "failed to touch OOBE completed flag")
	}
	return nil
}

// ClearOobeCompletion will remove the OOBE completed flag.
func ClearOobeCompletion(ctx context.Context) error {
	if err := os.Remove(oobeCompletedFlag); err != nil {
		return errors.Wrap(err, "failed to clear OOBE completed flag")
	}
	return nil
}
