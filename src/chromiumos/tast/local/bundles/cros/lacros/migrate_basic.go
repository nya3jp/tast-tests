// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"os"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/lacros/migrate"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MigrateBasic,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test basic functionality of Ash-to-Lacros profile migration",
		Contacts: []string{
			"ythjkt@google.com", // Test author
			"neis@google.com",
			"hidehiko@google.com",
			"lacros-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Params: []testing.Param{{
			Name: "primary",
			Val:  []lacrosfixt.Option{lacrosfixt.Mode(lacros.LacrosPrimary)},
		}, {
			Name: "only",
			Val:  []lacrosfixt.Option{lacrosfixt.Mode(lacros.LacrosOnly)},
		}},
	})
}

// MigrateBasic tests that migration is run to completion and Lacros is launchable after the migration.
func MigrateBasic(ctx context.Context, s *testing.State) {
	if err := clearMigrationState(ctx); err != nil {
		s.Fatal("Failed to run Chrome to clear migration state: ", err)
	}

	cr, err := migrate.Run(ctx, s.Param().([]lacrosfixt.Option))
	if err != nil {
		s.Fatal("Failed to migrate profile: ", err)
	}
	defer cr.Close(ctx)

	verifyLacrosLaunch(ctx, s, cr)
}

// clearMigrationState resets profile migration by running Ash with Lacros disabled.
func clearMigrationState(ctx context.Context) error {
	// First restart Chrome with Lacros disabled in order to reset profile migration.
	cr, err := chrome.New(ctx, chrome.DisableFeatures("LacrosSupport"))
	if err != nil {
		return errors.Wrap(err, "failed to start chrome")
	}
	defer cr.Close(ctx)

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat(migrate.LacrosFirstRunPath); !os.IsNotExist(err) {
			return errors.Wrap(err, "'First Run' file exists or cannot be read")
		}
		return nil
	}, nil); err != nil {
		return errors.Wrap(err, "'First Run' file exists or cannot be read")
	}

	return nil
}

// verifyLacrosLaunch checks if Lacros is launchable after profile migration.
func verifyLacrosLaunch(ctx context.Context, s *testing.State, cr *chrome.Chrome) {
	if _, err := os.Stat(migrate.LacrosFirstRunPath); err != nil {
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
