// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package apps contains functionality and test cases for Chrome OS essential Apps.
package apps

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/apps/cursive"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// The models that support Cursive.
var cursiveEnabledModels = []string{
	"nautilus",
	"phaser360",
	"krane",
	"bobba360",
	"kohaku",
	"kevin",
	"robo360",
	"vayne",
	"scarlet",
	"eve",
	"nocturne",
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         CursiveSmoke,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Cursive smoke test app launching and basic function",
		Contacts: []string{
			"shengjun@chromium.org",
			"gabpalado@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromeLoggedInForEA",
		Timeout:      5 * time.Minute,
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		HardwareDeps: hwdep.D(hwdep.Model(cursiveEnabledModels...)),
	})
}

func CursiveSmoke(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	if _, err := apps.InstallPWAForURL(ctx, cr, cursive.AppURL, 2*time.Minute); err != nil {
		s.Fatal("Failed to install Cursive: ", err)
	}

	if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.Cursive.ID, 5*time.Minute); err != nil {
		s.Fatal("Failed to wait for installed app: ", err)
	}

	// Cursive is automatically launched after installation.
	// Reset Chrome state will close all opened targets.
	if err := cr.ResetState(ctx); err != nil {
		s.Fatal("Failed to reset Chrome state: ", err)
	}

	// Validate Cursive can be launched from shelf.
	if err := apps.Launch(ctx, tconn, apps.Cursive.ID); err != nil {
		s.Fatal("Failed to launch Cursive: ", err)
	}

	if err := ash.WaitForApp(ctx, tconn, apps.Cursive.ID, time.Minute); err != nil {
		s.Fatalf("Fail to wait for %s by app id %s: %v", apps.Cursive.Name, apps.Cursive.ID, err)
	}

	if err := cursive.WaitForAppRendered(tconn)(ctx); err != nil {
		s.Fatal("Failed to render Cursive: ", err)
	}
}
