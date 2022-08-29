// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package migrate contains functionality shared by tests that tests profile
// migration from Ash to Lacros.
package migrate

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/localstate"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

// ReverseRun migrates user profile from Lacros to Ash and waits until migration is marked as completed by Ash.
// Once the migration is completed, it will relaunch Ash Chrome and returns the new `chrome.Chrome` instance.
func ReverseRun(ctx context.Context, opts []lacrosfixt.Option) (*chrome.Chrome, error) {
	// TODO(chromium:1290297): This is a hack.
	// chrome.New doesn't really support profile migration because it
	// doesn't anticipate the additional Chrome restart that profile
	// migration effects. As a result, the *Chrome return value is already
	// invalid and we must not use it. Moreover, we must disable the
	// RemoveNotification option because otherwise chrome.New will try to
	// interact with Chrome at a time when that is no longer safe.
	// In order to obtain a valid *Chrome value for the test to continue
	// with, we restart Chrome once more after profile migration.
	testing.ContextLog(ctx, "Restarting for profile migration")
	chromeOpts := []chrome.Option{
		chrome.KeepState(),
		chrome.RemoveNotification(false),
		chrome.EnableFeatures("LacrosProfileBackwardMigration"),
		chrome.DisableFeatures("LacrosSupport"),
	}

	crDoNotUse, err := chrome.New(ctx, chromeOpts...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if crDoNotUse != nil {
			crDoNotUse.Close(ctx)
		}
	}()

	testing.ContextLog(ctx, "Waiting for reverse profile migration to complete")
	userHash, err := cryptohome.UserHash(ctx, chrome.DefaultUser)
	if err != nil {
		return nil, err
	}
	pref := "lacros.profile_data_backward_migration_completed_for_user." + userHash
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		completedVal, err := localstate.UnmarshalPref(browser.TypeAsh, pref)
		if err != nil {
			return err
		}
		completed, ok := completedVal.(bool)
		if !ok || !completed {
			return errors.New("profile migration incomplete")
		}
		return nil
	}, nil); err != nil {
		return nil, err
	}

	crDoNotUse.Close(ctx)
	crDoNotUse = nil
	testing.ContextLog(ctx, "Restarting after reverse profile migration")
	return chrome.New(ctx, chromeOpts...)
}
