// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SplitChromeTabsTabletMode,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that Chrome tabs are draggable to split screen",
		Contacts: []string{
			"sophiewen@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      2 * time.Minute,
		Params: []testing.Param{{
			Val:     browser.TypeAsh,
			Fixture: "chromeLoggedIn",
		}, {
			Name:              "lacros",
			Val:               browser.TypeLacros,
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

func SplitChromeTabsTabletMode(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue().(chrome.HasChrome).Chrome(), s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to set up browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Open a Chrome window with three new tabs.
	url := "https://chrome://version"
	numNewTabs := 3
	// Lacro creates an extra blank tab in `browserfixt.SetUp`.
	if s.Param().(browser.Type) == browser.TypeLacros {
		numNewTabs--
	}
	for i := 0; i < numNewTabs; i++ {
		conn, err := br.NewConn(ctx, url)
		if err != nil {
			s.Fatalf("Failed to open new tab with url %q: %v", url, err)
		}
		defer conn.Close()
	}

	_, err = br.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get browser test API connection: ", err)
	}

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get all windows: ", err)
	}
	s.Log("num ash windows: ", len(ws))

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure tablet mode: ", err)
	}
	defer cleanup(cleanupCtx)

	pc, err := pointer.NewTouch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create a touch controller: ", err)
	}
	defer pc.Close()

	// Open the Chrome browser tab strip.
	tabStripButton := nodewith.Role(role.Button).HasClass("WebUITabCounterButton").First()
	if err := pc.Click(tabStripButton)(ctx); err != nil {
		s.Fatal("Failed to tap the tab strip button: ", err)
	}

	expectedNumWindows := 2
	if err := dragToSnap(ctx, tconn, pc, expectedNumWindows, ash.WindowStateRightSnapped); err != nil {
		s.Fatal("Failed to drag to snap right: ", err)
	}

	if err := dragToSnap(ctx, tconn, pc, expectedNumWindows+1, ash.WindowStateLeftSnapped); err != nil {
		s.Fatal("Failed to drag to snap left: ", err)
	}
}

func dragToSnap(ctx context.Context, tconn *chrome.TestConn, pc pointer.Context, expectedNumWindows int, snappedState ash.WindowStateType) error {
	// Get the first tab thumbnail in the tab strip.
	tabRect, err := uiauto.New(tconn).Location(ctx, nodewith.Role(role.Tab).First())
	if err != nil {
		return errors.Wrap(err, "failed to get the tab thumbnail")
	}

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the primary display info")
	}
	snapPoint := coords.NewPoint(info.WorkArea.Right()-1, info.WorkArea.CenterY())
	if snappedState == ash.WindowStateLeftSnapped {
		snapPoint = coords.NewPoint(info.WorkArea.Left, info.WorkArea.CenterY())
	}

	// Drag the first tab in the window and snap it to the side. Add sleep
	// to long press the tab thumbnail to be able to grab it.
	if err := pc.Drag(tabRect.CenterPoint(),
		action.Sleep(time.Second),
		pc.DragTo(snapPoint, 3*time.Second),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to drag the tab")
	}

	// Lacros needs to wait for both the dragged tab to be snapped in its new
	// window and the remaining tab to become its own window. Since the
	// remaining window does not exist until after the dragged tab is snapped
	// and therefore cannot be accessed, wait for it to be created by checking
	// `ash.GetAllWindows`.
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the window list")
	}
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		if ws, err = ash.GetAllWindows(ctx, tconn); err != nil {
			return false
		}
		return len(ws) == expectedNumWindows
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for drag to finish and the new window to be created")
	}

	if ws[0].State != snappedState {
		return errors.New("failed to snap the tab")
	}

	return nil
}
