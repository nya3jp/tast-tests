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
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HideContinueSectionClamshell,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests that the clamshell launcher continue section can be hidden",
		Contacts: []string{
			"cros-system-ui-eng@google.com",
			"chromeos-sw-engprod@google.com",
			"jamescook@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

func HideContinueSectionClamshell(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	opt := chrome.EnableFeatures("LauncherHideContinueSection")
	cr, err := chrome.New(ctx, opt)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Create temp files and open them via Files app to populate the continue section.
	cleanupFiles, _, err := launcher.SetupContinueSectionFiles(
		ctx, tconn, cr, false /* tabletMode */)
	if err != nil {
		s.Fatal("Failed to set up continue section: ", err)
	}
	defer cleanupFiles()

	// Bubble launcher requires clamshell mode.
	cleanup, err := launcher.SetUpLauncherTest(ctx, tconn, false /*tabletMode*/, false /*stabilizeAppCount*/)
	if err != nil {
		s.Fatal("Failed to set up launcher test case: ", err)
	}
	defer cleanup(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	// Ensure continue section exists.
	ui := uiauto.New(tconn)
	continueSection := nodewith.ClassName("ContinueSectionView")
	if err := ui.WithTimeout(3 * time.Second).WaitUntilExists(continueSection)(ctx); err != nil {
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

	// Clicking the "Hide all suggestions" button should hide continue tasks.
	bubble := nodewith.ClassName(ash.AppListBubbleClassName)
	hideButton := nodewith.Ancestor(bubble).Name("Hide all suggestions")
	if err := uiauto.Combine("Click hide all suggestions",
		ui.WaitUntilExists(hideButton),
		ui.LeftClick(hideButton),
		ui.WaitUntilGone(continueTask),
	)(ctx); err != nil {
		s.Fatal("Failed to hide continue tasks by clicking hide button: ", err)
	}

	// Clicking the "Show all suggestions" button should show continue tasks.
	showButton := nodewith.Ancestor(bubble).Name("Show all suggestions")
	if err := uiauto.Combine("Click show all suggestions",
		ui.WaitUntilExists(showButton),
		ui.LeftClick(showButton),
		ui.WaitUntilExists(continueTask),
	)(ctx); err != nil {
		s.Fatal("Failed to show continue tasks by clicking show button: ", err)
	}

	if err := launcher.CloseBubbleLauncher(tconn)(ctx); err != nil {
		s.Fatal("Failed to close bubble launcher: ", err)
	}
}
