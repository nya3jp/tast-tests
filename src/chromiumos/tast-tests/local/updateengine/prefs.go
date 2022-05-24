// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package updateengine provides ways to interact with update_engine daemon and utilities.
package updateengine

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	prefsDir  = "/var/lib/update_engine/prefs"
	prefsPerm = 0600
)

// Pref is the name prefs update_engine supports.
type Pref string

// List of prefs that update_engine utilizes.
const (
	// Pref which allows update_engine to perform background checks (non-interactive) in seconds.
	TestUpdateCheckIntervalTimeout Pref = "test-update-check-interval-timeout"
)

// SetPref will set the update_engine pref.
// Multi-level prefs will create prefPath's missing directories.
func SetPref(ctx context.Context, prefPath Pref, prefVal string) error {
	path := filepath.Join(prefsDir, string(prefPath))
	if err := os.MkdirAll(filepath.Dir(path), os.ModeDir); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(prefVal), prefsPerm)
}

// ForceClearPrefs will force clearing the pref without caring about update_engine being shutdown.
func ForceClearPrefs(ctx context.Context) error {
	testing.ContextLog(ctx, "Force clearing all ", JobName, " prefs")
	if err := os.RemoveAll(prefsDir); err != nil {
		return err
	}
	return nil
}

// ClearPrefs will clear the update_engine prefs + have update_engine refresh
// the cleared prefs.
func ClearPrefs(ctx context.Context) error {
	testing.ContextLog(ctx, "Clearing all ", JobName, " prefs")
	if err := StopDaemon(ctx); err != nil {
		return err
	}

	var err error
	// Always try to respawn update_engine in case of pref clearing failure.
	// Users of this package might not spawn update_engine back up or use
	// the correct fixtures/etc.
	defer func() {
		wrapErr := func(newErr error) {
			if err != nil {
				err = errors.Wrapf(err, "%s", newErr)
			} else {
				err = newErr
			}
		}
		if tmperr := StartDaemon(ctx); tmperr != nil {
			wrapErr(tmperr)
			return
		}
		if tmperr := WaitForService(ctx); tmperr != nil {
			wrapErr(tmperr)
		}
	}()

	err = ForceClearPrefs(ctx)
	return err
}
