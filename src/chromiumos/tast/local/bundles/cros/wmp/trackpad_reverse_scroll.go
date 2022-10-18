// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/trackpad"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TrackpadReverseScroll,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that track pad reverse scrolling works properly",
		Contacts:     []string{"dandersson@chromium.org", "zxdan@chromium.org", "chromeos-wmp@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		SearchFlags: []*testing.StringPair{
			{
				Key:   "feature_id",
				Value: "screenplay-ac83aef4-5602-49b7-8ef8-088f6660d52a",
			},
		},
		Params: []testing.Param{
			{
				Name: "reverse_on",
				Val:  true,
			},
			{
				Name: "reverse_off",
				Val:  false,
			},
		},
	})
}

// TrackpadReverseScroll tests the trackpad reverse scrolling.
func TrackpadReverseScroll(ctx context.Context, s *testing.State) {
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	ui := uiauto.New(tconn)

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	tpw, err := input.Trackpad(ctx)
	if err != nil {
		s.Fatal("Failed to initialize the trackpad: ", err)
	}
	defer tpw.Close()

	reverseOn := s.Param().(bool)

	switchReverse := trackpad.TurnOffReverseScroll
	if reverseOn {
		switchReverse = trackpad.TurnOnReverseScroll
	}

	if err := switchReverse(ctx, tconn); err != nil {
		s.Fatal("Failed to switch trackpad reverse scroll: ", err)
	}
	// -------------- Outside Overview Mode ---------------------
	// 1. Swiping down with 3 fingers won't enter overview.
	// 2. If reverse scroll is off, consecutively swiping down twice with 3
	//    fingers will trigger a system toast.
	// 3. Swiping up with 3 fingers will enter overview.

	// If reverse scrolling is off, swiping down twice with 3 fingers will
	// trigger a system toast.
	if !reverseOn {
		if err := swipeTwice(ctx, tconn, tpw, trackpad.DownSwipe, 3); err != nil {
			s.Fatal("Failed to swipe down twice with 3 fingers: ", err)
		}

		if err := waitForSystemToast(ctx, ui); err != nil {
			s.Fatal("Failed to show wrong overview gesture toast: ", err)
		}
	}

	// Swiping up with 3 fingers will enter Overview.
	if err := swipeToEnterOverview(ctx, tconn, tpw); err != nil {
		s.Fatal("Failed to swipe up to enter Overview: ", err)
	}
	defer ash.SetOverviewModeAndWait(cleanupCtx, tconn, false)
	// ---------------------------------------------------------
	// Wait for an interval for the next swipe gesture.
	if err := testing.Sleep(ctx, swipeInterval); err != nil {
		s.Fatal("Failed to wait for the swipe interval: ", err)
	}

	// -------------- Inside Overview Mode ---------------------
	// 1. Swiping up with 3 fingers won't exit overview.
	// 2. If reverse scroll is off, consecutively swiping up twice with 3
	//    fingers will trigger a system toast.
	// 3. Swiping down with 3 fingers will exit overview.

	// If reverse scrolling is off, swiping up twice with 3 fingers will
	// trigger a system toast.
	if !reverseOn {
		if err := swipeTwice(ctx, tconn, tpw, trackpad.UpSwipe, 3); err != nil {
			s.Fatal("Failed to swipe down twice with 3 fingers: ", err)
		}

		if err := waitForSystemToast(ctx, ui); err != nil {
			s.Fatal("Failed to show wrong overview gesture toast: ", err)
		}
	}

	// Swiping down with 3 fingers will exit Overview.
	if err := swipeToExitOverview(ctx, tconn, tpw); err != nil {
		s.Fatal("Failed to swipe down to exit Overview: ", err)
	}
	// ---------------------------------------------------------

	// Create a new desk.
	if err := ash.CreateNewDesk(ctx, tconn); err != nil {
		s.Fatal("Failed to create a new desk: ", err)
	}
	defer ash.CleanUpDesks(cleanupCtx, tconn)

	// ------------ On The Leftmost Desk ----------------------
	// 1. If reverse scroll is off, consecutively swiping left twice with 4
	//    fingers will trigger a system toast (only for non reverse scrolling).
	// 2. If reverse scroll is off, Swiping right with 4 fingers will switch to
	//    the right desk. Otherwise, swiping left with 4 fingers will switch to
	//    the right desk.

	// If reverse scroll is off, swiping left twice with 4 fingers will trigger a system toast.
	if !reverseOn {
		if err := swipeTwice(ctx, tconn, tpw, trackpad.LeftSwipe, 4); err != nil {
			s.Fatal("Failed to swipe left twice with 4 fingers: ", err)
		}

		if err := waitForSystemToast(ctx, ui); err != nil {
			s.Fatal("Failed to show wrong switching desk gesture toast: ", err)
		}
	}

	// If reverse scroll is off (on), swiping right (left) with 4 fingers will switch to the right desk.
	swipeDirection := trackpad.RightSwipe
	if reverseOn {
		swipeDirection = trackpad.LeftSwipe
	}

	if err := swipeToDesk(ctx, tconn, tpw, swipeDirection, 1); err != nil {
		s.Fatal("Failed to switch to the right desk: ", err)
	}
	//-------------------------------------------------------

	// ------------ On The Rightmost Desk ---------------------
	// 1. If reverse scroll is off, consecutively swiping right twice with 4
	//    fingers will trigger a system toast (only for non reverse scrolling).
	// 2. If reverse scroll is off, Swiping left with 4 fingers will switch to
	//    the left desk. Otherwise, swiping right with 4 fingers will switch to
	//    the left desk.

	// If reverse scroll is off, swiping right twice with 4 fingers will trigger a system toast.
	if !reverseOn {
		if err := swipeTwice(ctx, tconn, tpw, trackpad.RightSwipe, 4); err != nil {
			s.Fatal("Failed to swipe right twice with 4 fingers: ", err)
		}

		if err := waitForSystemToast(ctx, ui); err != nil {
			s.Fatal("Failed to show wrong switching desk gesture toast: ", err)
		}
	}

	// If reverse scroll is off (on), swiping left (right) with 4 fingers will switch to the left desk.
	swipeDirection = trackpad.LeftSwipe
	if reverseOn {
		swipeDirection = trackpad.RightSwipe
	}

	if err := swipeToDesk(ctx, tconn, tpw, swipeDirection, 0); err != nil {
		s.Fatal("Failed to switch to the left desk: ", err)
	}
}

const swipeInterval = time.Second

// swipeTwice performs trackpad swipe twice in the given direction with indicated number of touches.
func swipeTwice(ctx context.Context, tconn *chrome.TestConn, tpw *input.TrackpadEventWriter, swipeDirection trackpad.SwipeDirection, touches int) error {
	if err := trackpad.Swipe(ctx, tconn, tpw, swipeDirection, touches); err != nil {
		return errors.Wrapf(err, "failed to swipe twice with %d fingers", touches)
	}

	if err := testing.Sleep(ctx, swipeInterval); err != nil {
		return errors.Wrap(err, "failed to wait for the swipe interval")
	}

	if err := trackpad.Swipe(ctx, tconn, tpw, swipeDirection, touches); err != nil {
		return errors.Wrapf(err, "failed to swipe twice with %d fingers", touches)
	}

	return nil
}

// swipeToEnterOverview performs the swiping up with 3 fingers to enter Overview and validates that Overview is entered.
func swipeToEnterOverview(ctx context.Context, tconn *chrome.TestConn, tpw *input.TrackpadEventWriter) error {
	if err := trackpad.Swipe(ctx, tconn, tpw, trackpad.UpSwipe, 3); err != nil {
		return errors.Wrap(err, "failed to swipe up with 3 fingers")
	}

	if err := ash.WaitForOverviewState(ctx, tconn, ash.Shown, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to enter overview mode")
	}

	return nil
}

// swipeToExitOverview performs the swiping down with 3 fingers to exit Overview and validates that Overview is exited.
func swipeToExitOverview(ctx context.Context, tconn *chrome.TestConn, tpw *input.TrackpadEventWriter) error {
	if err := trackpad.Swipe(ctx, tconn, tpw, trackpad.DownSwipe, 3); err != nil {
		return errors.Wrap(err, "failed to swipe down with 3 fingers")
	}

	if err := ash.WaitForOverviewState(ctx, tconn, ash.Hidden, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to exit overview mode")
	}

	return nil
}

// swipeToDesk performs swiping on given direction with 4 fingers to switch to the target desk with the given desk index.
func swipeToDesk(ctx context.Context, tconn *chrome.TestConn, tpw *input.TrackpadEventWriter, swipeDirection trackpad.SwipeDirection, deskIndex int) error {
	ac := uiauto.New(tconn)
	if err := trackpad.Swipe(ctx, tconn, tpw, swipeDirection, 4); err != nil {
		return errors.Wrapf(err, "failed to swipe to: %v", swipeDirection)
	}

	// Make sure the desk animiation is finished.
	if err := ac.WithInterval(2*time.Second).WithTimeout(10*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		return errors.Wrap(err, "failed to swipe with 4 fingers")
	}

	desksInfo, err := ash.GetDesksInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the desks info")
	}

	// Verify that the active desk after swipe is the desk we're looking for.
	if desksInfo.ActiveDeskIndex != deskIndex {
		return errors.Wrapf(err, "unexepcted active desk: got %d, want %d", desksInfo.ActiveDeskIndex, deskIndex)
	}

	return nil
}

// waitForSystemToast waits for the system toast showing up.
func waitForSystemToast(ctx context.Context, ui *uiauto.Context) error {
	// The system toast for trackpad gesture will show up for a while and then disappear.
	if err := uiauto.Combine(
		"wait for system toast",
		ui.WaitUntilExists(nodewith.ClassName("SystemToastInnerLabel")),
		ui.WaitUntilGone(nodewith.ClassName("SystemToastInnerLabel")),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to show the trackpad gesture toast")
	}

	return nil
}
