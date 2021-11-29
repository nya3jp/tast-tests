// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/apps/helpapp"
	"chromiumos/tast/local/bundles/cros/apps/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchHelpAppFromShortcut,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Help app can be launched using shortcut Ctrl+Shift+/",
		Contacts: []string{
			"showoff-eng@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Params: []testing.Param{
			{
				Name:              "stable",
				Fixture:           "chromeLoggedInForEA",
				ExtraHardwareDeps: hwdep.D(pre.AppsStableModels),
			}, {
				Name:              "unstable",
				Fixture:           "chromeLoggedInForEA",
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(pre.AppsUnstableModels),
			},
			{
				Name:              "stable_guest",
				Fixture:           "chromeLoggedInGuestForEA",
				ExtraHardwareDeps: hwdep.D(pre.AppsStableModels),
			}, {
				Name:              "unstable_guest",
				Fixture:           "chromeLoggedInGuestForEA",
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(pre.AppsUnstableModels),
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

	helpCtx := helpapp.NewContext(cr, tconn)

	shortcuts := []string{"Ctrl+Shift+/", "Ctrl+/"}
	for index, shortcut := range shortcuts {
		// Using 'shortcut_{index} as test name.
		testName := "shortcut_" + strconv.Itoa(index)
		s.Run(ctx, testName, func(ctx context.Context, s *testing.State) {
			defer func() {
				outDir := filepath.Join(s.OutDir(), testName)
				faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, s.HasError, cr, "ui_tree_"+testName)

				if err := helpCtx.Close()(ctx); err != nil {
					s.Log("Failed to close the app, may not have been opened: ", err)
				}
			}()

			ui := uiauto.New(tconn).WithTimeout(time.Minute)
			if err := ui.Retry(5, func(ctx context.Context) error {
				if err := kw.Accel(ctx, shortcut); err != nil {
					return errors.Wrapf(err, "failed to press %q keys", shortcut)
				}
				return helpapp.NewContext(cr, tconn).WaitForApp()(ctx)
			})(ctx); err != nil {
				s.Fatalf("Failed to launch or render Help app by shortcut %q: %v", shortcut, err)
			}
		})
	}
}
