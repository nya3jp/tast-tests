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
		Func: HotseatScalable,
		Desc: "Tests the basic features of hotseat",
		Contacts: []string{
			"victor.chen@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

// HotseatScalable tests the basic features of hotseat in clamshell mode and tablet mode.
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
		return errors.Wrap(err, "failed to verify launcher in clamshell mode")
	}

	pindedApp := apps.Files
	appInitialState, err := isAppPinned(ctx, tconn, pindedApp.ID)
	if err != nil {
		return errors.Wrap(err, "failed to verify that the specified pinned app is displayed on the shelf in the initial state")
	}

	if err := ash.PinApps(ctx, tconn, []string{pindedApp.ID}); err != nil {
		return errors.Wrap(err, "failed to pin apps to the shelf")
	}

	appExist, err := isAppPinned(ctx, tconn, pindedApp.ID)
	if err != nil {
		return errors.Wrap(err, "failed to verify that the specified pinned app is displayed on the shelf")
	}
	if !appExist {
		return errors.New("the specified app is not pinned")
	}

	if !appInitialState {
		if err := ash.UnpinApps(ctx, tconn, []string{pindedApp.ID}); err != nil {
			return errors.Wrapf(err, "failed to unpin apps %v", pindedApp)
		}
	}

	if err := ui.WaitUntilExists(nodewith.HasClass("UnifiedSystemTray").Role(role.Button))(ctx); err != nil {
		return errors.Wrap(err, "failed to verify status menu in laptop mode")
	}

	return nil
}

// isAppPinned verifies if specific app appears on the shelf.
func isAppPinned(ctx context.Context, tconn *chrome.TestConn, appID string) (bool, error) {
	items, err := ash.ShelfItems(ctx, tconn)
	if err != nil {
		return false, errors.Wrap(err, "failed to get items")
	}

	for _, item := range items {
		if item.Type == ash.ShelfItemTypePinnedApp && item.AppID == appID {
			return true, nil
		}
	}

	return false, nil
}

// verifyTabletMode verifies that hotseat is hidden after activating a window.
// Hotseat should be extended after gesture swipe when an app is opened.
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
