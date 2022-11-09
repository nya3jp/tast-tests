// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/bundles/cros/lacros/migrate"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BackwardMigratePolicy,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test policy triggering of Lacros-to-Ash profile migration",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"lacros-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      fixture.FakeDMS,
	})
}

func BackwardMigratePolicy(ctx context.Context, s *testing.State) {
	fdms := s.FixtData().(*fakedms.FakeDMS)

	if err := migrate.ClearMigrationState(ctx); err != nil {
		s.Fatal("Failed to run Chrome to clear migration state: ", err)
	}

	blob := policy.NewBlob()

	// TODO: blob.AddPolicy(policy.Lacros)

	if err := fdms.WritePolicyBlob(blob); err != nil {
		s.Fatal("Failed to write policy blob: ", err)
	}

	forwardMigrate(ctx, fdms, s)
	backwardMigrate(ctx, fdms, s)
}

func forwardMigrate(ctx context.Context, fdms *fakedms.FakeDMS, s *testing.State) {
	cr, err := migrate.RunWithOptions(ctx, []chrome.Options{
		chrome.DMSPolicy(fdms.URL),
	}, lacrosfixt.Mode(lacros.LacrosOnly))
	if err != nil {
		s.Fatal("Failed to migrate profile: ", err)
	}
	defer cr.Close(ctx)

	migrate.VerifyLacrosLaunch(ctx, s, cr)
}

func backwardMigrate(ctx context.Context, fdms *fakedms.FakeDMS, s *testing.State) {
	cr, err := migrate.BackwardRun(ctx, []chrome.Options{
		chrome.DMSPolicy(fdms.URL),
	}, lacrosfixt.Mode(lacros.LacrosOnly))
	if err != nil {
		s.Fatal("Failed to backward migrate profile: ", err)
	}
	defer cr.Close(ctx)
}
