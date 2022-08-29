// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"

	"chromiumos/tast/local/bundles/cros/lacros/migrate"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BackwardMigrateBasic,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test basic functionality of Lacros-to-Ash profile migration",
		Contacts: []string{
			"vsavu@google.com", // Test author
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

// BackwardMigrateBasic tests forward and then backward lacros migration.
func BackwardMigrateBasic(ctx context.Context, s *testing.State) {
	if err := migrate.ClearMigrationState(ctx); err != nil {
		s.Fatal("Failed to run Chrome to clear migration state: ", err)
	}

	forwardMigrate(ctx, s)
	backwardMigrate(ctx, s)
}

func forwardMigrate(ctx context.Context, s *testing.State) {
	cr, err := migrate.Run(ctx, s.Param().([]lacrosfixt.Option))
	if err != nil {
		s.Fatal("Failed to migrate profile: ", err)
	}
	defer cr.Close(ctx)

	migrate.VerifyLacrosLaunch(ctx, s, cr)
}

func backwardMigrate(ctx context.Context, s *testing.State) {
	cr, err := migrate.BackwardRun(ctx, s.Param().([]lacrosfixt.Option))
	if err != nil {
		s.Fatal("Failed to backward migrate profile: ", err)
	}
	defer cr.Close(ctx)
}
