// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package migrate contains functionality shared by tests that tests profile
// migration from Ash to Lacros.
package migrate

import (
	"context"
	"os"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/localstate"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

// LacrosFirstRunPath is the path to `First Run` sentinel file in Lacros profile dir.
const LacrosFirstRunPath = "/home/chronos/user/lacros/First Run"

// Run migrates user profile from Ash to Lacros and wait until migration is marked as completed by Ash.
// Once the migration is completed, it will relaunch Ash Chrome and returns the new `chrome.Chrome` instance.
func Run(ctx context.Context, opts []lacrosfixt.Option) (*chrome.Chrome, error) {
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
		chrome.EnableFeatures("LacrosProfileMigrationForAnyUser"),
	}
	opts = append(opts, lacrosfixt.ChromeOptions(chromeOpts...))
	chromeOpts, err := lacrosfixt.NewConfig(opts...).Opts()
	if err != nil {
		return nil, errors.Wrap(err, "failed to compute Chrome options")
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

	testing.ContextLog(ctx, "Waiting for profile migration to complete")
	userHash, err := cryptohome.UserHash(ctx, chrome.DefaultUser)
	if err != nil {
		return nil, err
	}
	pref := "lacros.profile_migration_completed_for_user." + userHash
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
	testing.ContextLog(ctx, "Restarting after profile migration")
	return chrome.New(ctx, chromeOpts...)
}

// ClearMigrationState resets profile migration by running Ash with Lacros disabled.
func ClearMigrationState(ctx context.Context) error {
	// First restart Chrome with Lacros disabled in order to reset profile migration.
	cr, err := chrome.New(ctx, chrome.DisableFeatures("LacrosSupport"))
	if err != nil {
		return errors.Wrap(err, "failed to start chrome")
	}
	defer cr.Close(ctx)

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat(LacrosFirstRunPath); !os.IsNotExist(err) {
			return errors.Wrap(err, "'First Run' file exists or cannot be read")
		}
		return nil
	}, nil); err != nil {
		return errors.Wrap(err, "'First Run' file exists or cannot be read")
	}

	return nil
}

// VerifyLacrosLaunch checks if Lacros is launchable after profile migration.
func VerifyLacrosLaunch(ctx context.Context, s *testing.State, cr *chrome.Chrome) {
	if _, err := os.Stat(LacrosFirstRunPath); err != nil {
		s.Fatal("Error reading 'First Run' file: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	l, err := lacros.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch lacros: ", err)
	}
	l.Close(ctx)
}
