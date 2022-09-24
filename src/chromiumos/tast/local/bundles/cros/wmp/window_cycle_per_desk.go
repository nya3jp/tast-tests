// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/wmp/wmputils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowCyclePerDesk,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks Alt+Tab and Alt+Shift+Tab functionality for cycling windows for each desk",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"chromeos-wmp@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "no_kernel_upstream"},
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
		}, {
			Name:              "lacros",
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

func WindowCyclePerDesk(ctx context.Context, s *testing.State) {
	// Reserve for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Ensure there is no window open before test starts.
	if err := ash.CloseAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to ensure no window is open: ", err)
	}

	// Open one browser on the Desk1.
	browserApp, err := apps.PrimaryBrowser(ctx, tconn)
	if err != nil {
		s.Fatal("Could not find browser app info: ", err)
	}
	if err := apps.Launch(ctx, tconn, browserApp.ID); err != nil {
		s.Fatal("Failed to launch browser: ", err)
	}
	if err := ash.WaitForApp(ctx, tconn, browserApp.ID, time.Minute); err != nil {
		s.Fatal("Browser did not appear in shelf after launch: ", err)
	}

	// Create 7 desks, and totally have 8 desks. Create a browser window for each desk.
	totalDesks := 8
	for i := 1; i < totalDesks; i++ {
		if err = ash.CreateNewDesk(ctx, tconn); err != nil {
			s.Fatal("Failed to create a new desk: ", err)
		}

		// Active the new created desk.
		if err = ash.ActivateDeskAtIndex(ctx, tconn, i); err != nil {
			s.Fatalf("Failed to activate desk with index %d: %v", i, err)
		}
		// Open one browser on the desk.
		browserApp, err := apps.PrimaryBrowser(ctx, tconn)
		if err != nil {
			s.Fatal("Could not find browser app info: ", err)
		}
		if err := apps.Launch(ctx, tconn, browserApp.ID); err != nil {
			s.Fatal("Failed to launch browser: ", err)
		}
		if err := ash.WaitForApp(ctx, tconn, browserApp.ID, time.Minute); err != nil {
			s.Fatal("Browser did not appear in shelf after launch: ", err)
		}
	}

	ac := uiauto.New(tconn)
	// 3. Tests that when we active any desk (desk 5) and press alt-tab keys and enable the "Current desk" option.
	if err := ash.ActivateDeskAtIndex(ctx, tconn, 5); err != nil {
		s.Fatal("Failed to activate desk 5: ", err)
	}
	if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		s.Fatal("Failed to wait for active desk animation to be completed")
	}

	// Get the keyboard
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	if err := clickCurrentDeskButton(ctx, ac); err != nil {
		s.Fatal("Failed to open Alt+Tab window to click Current desk button: ", err)
	}

	// 4. Tests that when we traverse all of the desks and enter the alt-tab window, the "Current desk" should be
	// enabled based on the test 3.
	for i := 0; i < totalDesks; i++ {
		if err = ash.ActivateDeskAtIndex(ctx, tconn, i); err != nil {
			s.Fatalf("Failed to activate desk %d: %v", i, err)
		}
		// FindAllWindows returns the windows existing on the active desk.
		windows, err := ash.FindAllWindows(ctx, tconn, func(w *ash.Window) bool {
			return w.OnActiveDesk
		})
		if err != nil {
			s.Fatal("Failed to get all windows on active desk: ", err)
		}
		// Cycle windows for the active desk.
		if err := wmputils.VerifyWindowsForCycleMenu(ctx, tconn, ac, windows); err != nil {
			s.Fatalf("Failed to cycle windows for the active desk with index %d: %v", i, err)
		}
	}

	// 5. Delete all desks and press alt-tab keys. All the active apps and browsers should be shown without any option
	// at the top of the alt-tab window.
	if err := ash.CleanUpDesks(ctx, tconn); err != nil {
		s.Fatal("Failed to close all desks: ", err)
	}
	// Finder for the window cycle menu when there is only one desk.
	windowCycleView := nodewith.ClassName("WindowCycleView")
	// WindowCycleTabSlider contains the All desks button and Current desk button.
	windowCycleTabSlider := nodewith.ClassName("WindowCycleTabSlider").Ancestor(windowCycleView)

	// Make sure the cycle menu isn't open already before we try to alt+tab.
	if err := ac.WithTimeout(5 * time.Second).WaitUntilGone(windowCycleView)(ctx); err != nil {
		s.Fatal("Cycle menu unexpectedly open before pressing alt+tab: ", err)
	}
	if err := uiauto.Combine(
		"check no window cycle tab slider when there is only one desk",
		keyboard.AccelPressAction("Alt+Tab"),
		ac.WithTimeout(5*time.Second).WaitUntilExists(windowCycleView),
		ac.WithTimeout(5*time.Second).WaitUntilGone(windowCycleTabSlider),
		keyboard.AccelReleaseAction("Alt+Tab"),
	)(ctx); err != nil {
		s.Fatal("Failed to bring up the cycle window: ", err)
	}
}

func clickCurrentDeskButton(ctx context.Context, ac *uiauto.Context) error {
	// Finder for the window cycle menu when there is only one desk.
	cycleMenu := nodewith.ClassName("WindowCycleView")

	// Get the keyboard
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard")
	}
	defer keyboard.Close()

	// Make sure the cycle menu isn't open already before we try to alt+tab.
	if err := ac.WithTimeout(5 * time.Second).WaitUntilGone(cycleMenu)(ctx); err != nil {
		return errors.Wrap(err, "cycle menu unexpectedly open before pressing Alt+Tab")
	}

	// Open cycle window and get app order.
	if err := keyboard.AccelPress(ctx, "Alt"); err != nil {
		return errors.Wrap(err, "failed to long press Alt")
	}
	defer keyboard.AccelRelease(ctx, "Alt")

	if err := testing.Sleep(ctx, 500*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to sleep before press tab to open Alt+Tab window")
	}

	if err := keyboard.Accel(ctx, "Tab"); err != nil {
		return errors.Wrap(err, "failed to press Tab")
	}

	currentDeskToggleButton := nodewith.HasClass("WindowCycleTabSliderButton").Name("Current desk")
	if err := ac.WithTimeout(5 * time.Second).WaitUntilExists(currentDeskToggleButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to get Current desk button")
	}

	// Click Current desk button.
	if err := ac.LeftClick(currentDeskToggleButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to left click Current desk button")
	}

	return nil
}
