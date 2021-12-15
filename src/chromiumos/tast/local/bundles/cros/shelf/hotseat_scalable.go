// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shelf

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HotseatScalable,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests the basic features of hotseat",
		Contacts: []string{
			"victor.chen@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"tbarzic@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

// HotseatScalable tests the hiding and pop-up of hotseat, as well as the launcher icon, pinned apps and status menu in clamshell mode and tablet mode.
func HotseatScalable(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get Test API connection: ", err)
	}

	type modeType string

	const (
		clamshell modeType = "clamshell mode"
		tablet    modeType = "tablet mode"
	)

	for _, mode := range []modeType{clamshell, tablet} {
		f := func(ctx context.Context, s *testing.State) {
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
			defer cancel()

			cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, mode == tablet)
			if err != nil {
				s.Fatal("Failed to ensure tablet/clamshell mode: ", err)
			}
			defer cleanup(cleanupCtx)

			if mode == tablet {
				if err := verifyTabletMode(ctx, tconn, cr, s.OutDir()); err != nil {
					s.Fatal("Failed to verify tablet mode: ", err)
				}
			} else {
				if err := verifyClamshellMode(ctx, tconn); err != nil {
					s.Fatal("Failed to verify clamshell mode: ", err)
				}
			}

		}
		if !s.Run(ctx, string(mode), f) {
			s.Fatalf("Failed to finish the test in: %s", string(mode))
		}
	}
}

// verifyClamshellMode verifies that entire shelf is shown in clamshell mode.
// Launcher icon, pinned apps and status menu should be displayed.
func verifyClamshellMode(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)

	if err := ui.WaitUntilExists(nodewith.Name("Launcher").ClassName("ash/HomeButton").Role(role.Button))(ctx); err != nil {
		return errors.Wrap(err, "failed to find launcher button")
	}

	resetPinState, err := ash.ResetShelfPinState(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the function to reset pin states")
	}
	defer resetPinState(ctx)

	apps := []string{apps.Files.ID}

	if err := ash.PinApps(ctx, tconn, apps); err != nil {
		return errors.Wrap(err, "failed to pin apps to the shelf")
	}

	pinned, err := ash.AppsPinned(ctx, tconn, apps)
	if err != nil {
		return errors.Wrap(err, "failed to verify that the specified pinned app is displayed on the shelf")
	}
	for _, p := range pinned {
		if !p {
			return errors.New("app is not pinned")
		}
	}

	if err := ui.WaitUntilExists(nodewith.HasClass("UnifiedSystemTray").Role(role.Button))(ctx); err != nil {
		return errors.Wrap(err, "failed to verify status menu in laptop mode")
	}

	return nil
}

// verifyTabletMode verifies that hotseat is hidden after activating a window.
// Hotseat should be extended after gesture swipe when an app is opened.
// OutDir is the path to which output logs should be written.
func verifyTabletMode(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, outDir string) (retErr error) {
	// Verify tablet mode without any app opened.
	if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfShownHomeLauncher); err != nil {
		return errors.Wrap(err, "failed to verify home mode")
	}

	// Verify tablet mode when an app is opened.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	conn, err := cr.NewConn(ctx, ui.PerftestURL)
	if err != nil {
		return errors.Wrap(err, "failed to open browser windows")
	}
	defer func(ctx context.Context) {
		faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, func() bool { return retErr != nil }, cr, "tablet_ui_dump")
		conn.CloseTarget(ctx)
		conn.Close()
	}(cleanupCtx)

	tc, err := pointer.NewTouchController(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to create the touch controller")
	}
	defer tc.Close()

	if err := ash.WaitForHotseatAnimationToFinish(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for the animation to finish")
	}

	info, err := ash.FetchHotseatInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the hotseat info")
	}

	if info.HotseatState != ash.ShelfHidden {
		return errors.New("hotseat is not hidden after activating a window")
	}

	return ash.SwipeUpHotseatAndWaitForCompletion(ctx, tconn, tc.EventWriter(), tc.TouchCoordConverter())
}
