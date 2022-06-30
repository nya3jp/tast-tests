// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HideContinueSectionTablet,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests that the tablet launcher continue section can be hidden",
		Contacts: []string{
			"cros-system-ui-eng@google.com",
			"chromeos-sw-engprod@google.com",
			"jamescook@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
	})
}

func HideContinueSectionTablet(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	opt := chrome.EnableFeatures(
		"ProductivityLauncher",        // Enable productivity launcher
		"ForceShowContinueSection",    // Add fake continue tasks
		"LauncherHideContinueSection") // Enable the hide continue section menu item
	cr, err := chrome.New(ctx, opt)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	// Switch to tablet mode.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)

	// Show the launcher.
	if err := launcher.OpenProductivityLauncher(ctx, tconn, true); err != nil {
		s.Fatal("Failed to open launcher: ", err)
	}

	// Ensure continue section exists.
	ui := uiauto.New(tconn)
	continueSection := nodewith.ClassName("ContinueSectionView")
	if err := ui.WaitUntilExists(continueSection)(ctx); err != nil {
		s.Fatal("Failed to show continue section: ", err)
	}

	// Dismiss the privacy notice.
	if err := launcher.DismissPrivacyNotice(ctx, tconn); err != nil {
		s.Fatal("Failed to dismiss privacy notice: ", err)
	}

	// Ensure at least one continue task is visible.
	continueTask := nodewith.Ancestor(continueSection).ClassName("ContinueTaskView").First()
	if err := ui.WaitUntilExists(continueTask)(ctx); err != nil {
		s.Fatal("Failed to find continue tasks: ", err)
	}

	// Select "Hide all suggestions" from the desktop context menu.
	if err := uiauto.Combine("Hide all suggestions",
		mouse.Click(tconn, coords.Point{X: 4, Y: 4}, mouse.RightButton),
		ui.LeftClick(nodewith.ClassName("MenuItemView").Name("Hide all suggestions")),
	)(ctx); err != nil {
		s.Fatal("Failed to select hide suggestions context menu item: ", err)
	}

	// Continue task should be hidden.
	if err := ui.WaitUntilGone(continueTask)(ctx); err != nil {
		s.Fatal("Failed to hide continue task: ", err)
	}

	// Select "Show all suggestions" from the desktop context menu.
	if err := uiauto.Combine("Show all suggestions",
		mouse.Click(tconn, coords.Point{X: 4, Y: 4}, mouse.RightButton),
		ui.LeftClick(nodewith.ClassName("MenuItemView").Name("Show all suggestions")),
	)(ctx); err != nil {
		s.Fatal("Failed to select show suggestions context menu item: ", err)
	}

	// Continue task should be visible again.
	if err := ui.WaitUntilExists(continueTask)(ctx); err != nil {
		s.Fatal("Failed to show continue task: ", err)
	}
}
