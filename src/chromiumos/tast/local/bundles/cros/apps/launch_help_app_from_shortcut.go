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
				Fixture:           "chromeLoggedInForEA",
				ExtraHardwareDeps: pre.AppsStableModels,
			}, {
				Name:              "unstable",
				Fixture:           "chromeLoggedInForEA",
				ExtraHardwareDeps: pre.AppsUnstableModels,
			},
			{
				Name:              "stable_guest",
				Fixture:           "chromeLoggedInGuestForEA",
				ExtraHardwareDeps: pre.AppsStableModels,
			}, {
				Name:              "unstable_guest",
				Fixture:           "chromeLoggedInGuestForEA",
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

	// On some low-end devices and guest mode sometimes Chrome is still
	// initializing when the shortcut keys are emitted. Check that the
	// app is showing up as installed before emitting the shortcut keys.
	if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.Help.ID, 30*time.Second); err != nil {
		s.Fatal("Failed to wait for Explore to be installed: ", err)
	}

	shortcuts := []string{"Ctrl+Shift+/", "Ctrl+/"}
	for _, shortcut := range shortcuts {
		if err := kw.Accel(ctx, shortcut); err != nil {
			s.Errorf("Failed to press %q keys: %v", shortcut, err)
		}

		if err := helpapp.NewContext(cr, tconn).WaitForApp()(ctx); err != nil {
			s.Errorf("Failed to launch or render Help app by shortcut %q: %v", shortcut, err)
		}

		// Close the Help app, this may error if the app failed to
		// open and we only use it to reset for the next shortcut.
		// Simply log the error out instead of failing.
		if err := apps.Close(ctx, tconn, apps.Help.ID); err != nil {
			s.Log("Failed to close the app, may not have been opened: ", err)
		}
	}
}
