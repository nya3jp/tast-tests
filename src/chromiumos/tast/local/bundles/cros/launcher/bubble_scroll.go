// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BubbleScroll,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests scrolling in the bubble launcher",
		Contacts: []string{
			"cros-system-ui-eng@google.com",
			"chromeos-sw-engprod@google.com",
			"jamescook@chromium.org",
			"tbarzic@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInWith100FakeAppsNoAppSort",
	})
}

func BubbleScroll(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Some models (e.g. magister) don't synthesize touch events when the
	// display is turned off. Ensure the display is on.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	cleanup, err := launcher.SetUpLauncherTest(ctx, tconn, false /*tabletMode*/, true /*stabilizeAppCount*/)
	if err != nil {
		s.Fatal("Failed to set up launcher test case: ", err)
	}
	defer cleanup(cleanupCtx)

	// On failure, take the screenshot before the above cleanup() happens.
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	// Get the expected browser, which might be "Chromium" on unbranded builds.
	chromeApp, err := apps.ChromeOrChromium(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to find the chrome app: ", err)
	}

	// Wait for the chrome icon in the main apps grid (not recent apps) to stabilize.
	appsGrid := nodewith.HasClass(launcher.BubbleAppsGridViewClass)
	chromeItem := nodewith.HasClass(launcher.ExpandedItemsClass).
		Ancestor(appsGrid).Name(chromeApp.Name)
	ui := uiauto.New(tconn)
	if err := ui.WaitForLocation(chromeItem)(ctx); err != nil {
		s.Fatal("Failed to wait for Chrome item location to be idle: ", err)
	}

	// Chrome icon should be onscreen by default.
	if err := waitUntilOnscreen(ctx, ui, chromeItem); err != nil {
		s.Fatal("Chrome item not onscreen at test start: ", err)
	}

	///////////////////////////////////////////////////////////////////////////
	// Test scrolling by touch dragging in apps grid.

	// Set up touch context.
	touch, err := touch.New(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to set up touch context: ", err)
	}

	// Get apps grid information, including its bounds rect.
	appsGridInfo, err := ui.Info(ctx, appsGrid)
	if err != nil {
		s.Fatal("Failed to get apps grid info: ", err)
	}
	appsGridRect := appsGridInfo.Location

	// Start swipe just inside bottom-left.
	swipeStart := coords.NewPoint(appsGridRect.Left+4, appsGridRect.Bottom()-4)
	// End swipe just inside top-left.
	swipeEnd := coords.NewPoint(appsGridRect.Left+4, appsGridRect.Top+4)
	const swipeDuration = 250 * time.Millisecond
	// Swipe up to scroll the grid. This won't drag an item because the touch
	// does not press-and-hold.
	if err := touch.Swipe(swipeStart, touch.SwipeTo(swipeEnd, swipeDuration))(ctx); err != nil {
		s.Fatal("Failed to swipe up: ", err)
	}

	// Chrome icon should move offscreen.
	if err := waitUntilOffscreen(ctx, ui, chromeItem); err != nil {
		s.Fatal("Chrome item not offscreen after touch scroll up: ", err)
	}

	// Swipe down in the reverse direction.
	if err := touch.Swipe(swipeEnd, touch.SwipeTo(swipeStart, swipeDuration))(ctx); err != nil {
		s.Fatal("Failed to swipe down: ", err)
	}

	// Chrome icon should move back onscreen.
	if err := waitUntilOnscreen(ctx, ui, chromeItem); err != nil {
		s.Fatal("Chrome item not onscreen after touch scroll down: ", err)
	}

	///////////////////////////////////////////////////////////////////////////
	// Test scrolling by dragging scroll thumb.

	scrollBar := nodewith.Role(role.ScrollBar)
	scrollBarInfo, err := ui.Info(ctx, scrollBar)
	if err != nil {
		s.Fatal("Failed to get scroll bar info")
	}

	thumb := nodewith.ClassName("BaseScrollBarThumb").Ancestor(scrollBar)
	thumbInfo, err := ui.Info(ctx, thumb)
	if err != nil {
		s.Fatal("Failed to get thumb info")
	}

	// If the thumb fills the scroll bar, it's not possible to scroll.
	if thumbInfo.Location.Height >= scrollBarInfo.Location.Height {
		s.Fatalf("Scroll thumb height %d is not less than scroll bar height %d",
			thumbInfo.Location.Height, scrollBarInfo.Location.Height)
	}

	// Drag the scroll thumb all the way down to the bottom.
	dragStart := thumbInfo.Location.CenterPoint()
	dragDelta := scrollBarInfo.Location.Height - thumbInfo.Location.Height
	dragEnd := coords.NewPoint(thumbInfo.Location.CenterX(), thumbInfo.Location.CenterY()+dragDelta)
	const dragDuration = 250 * time.Millisecond
	if err := mouse.Drag(tconn, dragStart, dragEnd, dragDuration)(ctx); err != nil {
		s.Fatalf("Failed to drag scroll thumb down from %v to %v: %v", dragStart, dragEnd, err)
	}

	// Chrome icon should scroll offscreen.
	if err := waitUntilOffscreen(ctx, ui, chromeItem); err != nil {
		s.Fatal("Chrome item did not scroll offscreen: ", err)
	}

	// Drag scroll thumb up to the top by doing the same drag in reverse.
	if err := mouse.Drag(tconn, dragEnd, dragStart, dragDuration)(ctx); err != nil {
		s.Fatalf("Failed to drag scroll thumb up from %v to %v: %v", dragEnd, dragStart, err)
	}

	// Chrome item should scroll back onscreen.
	if err := waitUntilOnscreen(ctx, ui, chromeItem); err != nil {
		s.Fatal("Chrome item did not scroll back onscreen: ", err)
	}

	///////////////////////////////////////////////////////////////////////////
	// Test scrolling by highlighting items with the keyboard.

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	// Highlight an item in the last row by pressing the up key.
	if err := kb.TypeKey(ctx, input.KEY_UP); err != nil {
		s.Fatal("Failed to send up key: ", err)
	}

	// Chrome icon should scroll offscreen.
	if err := waitUntilOffscreen(ctx, ui, chromeItem); err != nil {
		s.Fatal("Chrome item did not scroll offscreen with keyboard: ", err)
	}

	// Highlight recent apps by pressing the down key twice.
	if err := kb.TypeKey(ctx, input.KEY_DOWN); err != nil {
		s.Fatal("Failed to send down key: ", err)
	}
	if err := kb.TypeKey(ctx, input.KEY_DOWN); err != nil {
		s.Fatal("Failed to send down key again: ", err)
	}

	// Chrome icon should be onscreen again.
	if err := waitUntilOnscreen(ctx, ui, chromeItem); err != nil {
		s.Fatal("Chrome item did not scroll onscreen with keyboard: ", err)
	}
}

func waitUntilOffscreen(ctx context.Context, ui *uiauto.Context, targetItem *nodewith.Finder) error {
	return waitUntilOffscreenState(ctx, ui, targetItem, true)
}

func waitUntilOnscreen(ctx context.Context, ui *uiauto.Context, targetItem *nodewith.Finder) error {
	return waitUntilOffscreenState(ctx, ui, targetItem, false)
}

func waitUntilOffscreenState(ctx context.Context, ui *uiauto.Context,
	targetItem *nodewith.Finder, expectedOffscreen bool) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		info, err := ui.Info(ctx, targetItem)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get target item info"))
		}
		if info.State[state.Offscreen] != expectedOffscreen {
			return errors.Errorf("Item does not have expected offscreen state %v",
				expectedOffscreen)
		}
		return nil

	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 500 * time.Millisecond})
}
