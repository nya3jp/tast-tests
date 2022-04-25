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

// List of prefs that update_engine utilizes.
const (
	TestUpdateCheckIntervalTimeout = "test-update-check-interval-timeout"
)

// SetPref will set the updateengine pref.
func SetPref(ctx context.Context, prefPath, prefVal string) error {
	path := filepath.Join(prefsDir, prefPath)
	dir := filepath.Dir(path)
	if path == prefsDir {
		return errors.New("set pref: invalid pref path")
	}
	if err := os.MkdirAll(dir, os.ModeDir); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(prefVal), prefsPerm)
}

// ForceClearPrefs will force clearing the pref without caring about update_engine being shutdown.
func ForceClearPrefs(ctx context.Context) error {
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
	if err := ForceClearPrefs(ctx); err != nil {
		return err
	}
	if err := StartDaemon(ctx); err != nil {
		return err
	}
	if err := WaitForService(ctx); err != nil {
		return err
	}
	return nil
}
