// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/apps/helpapp"
	"chromiumos/tast/local/bundles/cros/apps/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchHelpAppFromShortcut,
		Desc: "Help app can be launched using shortcut Ctrl+Shift+/",
		Contacts: []string{
			"showoff-eng@google.com",
			"benreich@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name:              "stable",
				Fixture:           "chromeLoggedIn",
				ExtraHardwareDeps: pre.AppsStableModels,
			}, {
				Name:              "unstable",
				Fixture:           "chromeLoggedIn",
				ExtraHardwareDeps: pre.AppsUnstableModels,
			},
			{
				Name:              "stable_guest",
				Fixture:           "chromeLoggedInGuest",
				ExtraHardwareDeps: pre.AppsStableModels,
			}, {
				Name:              "unstable_guest",
				Fixture:           "chromeLoggedInGuest",
				ExtraHardwareDeps: pre.AppsUnstableModels,
			},
		},
	})
}

// LaunchHelpAppFromShortcut verifies launching Help app from Ctrl+Shift+/.
func LaunchHelpAppFromShortcut(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard handle: ", err)
	}
	defer kw.Close()

	if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.Help.ID, 30*time.Second); err != nil {
		s.Fatal("Failed to wait for Explore to be installed: ", err)
	}

	const accel = "Ctrl+Shift+/"
	if err := kw.Accel(ctx, accel); err != nil {
		s.Fatalf("Failed to press %q keys: %v", accel, err)
	}

	if err := helpapp.WaitForApp(ctx, tconn); err != nil {
		s.Fatal("Failed to launch or render Help app: ", err)
	}
}
