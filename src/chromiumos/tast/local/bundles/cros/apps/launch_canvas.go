// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package apps contains functionality and test cases for ChromeOS essential Apps.
package apps

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/apps/fixture"
	"chromiumos/tast/local/bundles/cros/apps/pre"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchCanvas,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Launches Chrome Canvas APP through the launcher after user login",
		Contacts: []string{
			"blick-swe@google.com",
			"shengjun@chromium.org",
		},
		Attr:         []string{"group:mainline"},
		Timeout:      5 * time.Minute,
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Params: []testing.Param{{
			Name:              "stable",
			Fixture:           fixture.LoggedIn,
			ExtraHardwareDeps: hwdep.D(pre.AppsStableModels),
		}, {
			Name:              "lacros",
			Fixture:           fixture.LacrosLoggedIn,
			ExtraSoftwareDeps: []string{"lacros"},
			ExtraAttr:         []string{"informational"},
			ExtraHardwareDeps: hwdep.D(pre.AppsStableModels),
		}},
	})
}

// LaunchCanvas verifies launching Canvas after OOBE
func LaunchCanvas(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn

	cleanupCtx := ctx
	// Use a shortened context for test operations to reserve time for cleanup.
	ctx, shortCancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer shortCancel()

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.Canvas.ID, 2*time.Minute); err != nil {
		s.Fatal("Failed to wait for installed app: ", err)
	}

	if err := apps.Launch(ctx, tconn, apps.Canvas.ID); err != nil {
		s.Fatal("Failed to launch Canvas: ", err)
	}
	defer apps.Close(cleanupCtx, tconn, apps.Canvas.ID)

	if err := ash.WaitForApp(ctx, tconn, apps.Canvas.ID, time.Minute); err != nil {
		s.Fatalf("Fail to wait for %s by app id %s: %v", apps.Canvas.Name, apps.Canvas.ID, err)
	}

	// Use welcome page to verify page rendering
	ui := uiauto.New(tconn).WithTimeout(60 * time.Second)
	canvasFinder := nodewith.Role(role.Heading).Name("Welcome to Canvas!")

	if err := ui.WaitUntilExists(canvasFinder)(ctx); err != nil {
		s.Fatal("Failed to render Canvas: ", err)
	}
}
