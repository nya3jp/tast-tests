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

// OOBE related constants.
const (
	// Flag that indicates OOBE completion.
	oobeCompletedFlag = "/home/chronos/.oobe_completed"
	// Permission for OOBE completed flag.
	oobeCompletedFlagPerm = 0644
)

// MarkOobeCompletion will fake being OOBE complete, so update-engine won't
// block update checks.
func MarkOobeCompletion(ctx context.Context) error {
	if err := os.WriteFile(oobeCompletedFlag, []byte{}, oobeCompletedFlagPerm); err != nil {
		return errors.Wrap(err, "failed to touch OOBE completed flag")
	}
	return nil
}

// ClearOobeCompletion will remove the OOBE completed flag.
func ClearOobeCompletion(ctx context.Context) error {
	if err := os.RemoveAll(oobeCompletedFlag); err != nil {
		return errors.Wrap(err, "failed to clear OOBE completed flag")
	}
	return nil
}
