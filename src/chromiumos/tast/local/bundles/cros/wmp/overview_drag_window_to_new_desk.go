// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/wmp/wmputils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

const (
	// zeroStateDesksBarHeight is the height of desks bar when it's at zero state.
	zeroStateDesksBarHeight = 40
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OverviewDragWindowToNewDesk,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that drag window to new desk in overview mode works correctly",
		Contacts: []string{
			"conniekxu@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
		SearchFlags: []*testing.StringPair{{
			Key: "feature_id",
			// Send window to another desk.
			Value: "screenplay-655469b9-efb0-4595-aba4-7d91d265b3dd",
		}},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
	})
}

func OverviewDragWindowToNewDesk(ctx context.Context, s *testing.State) {
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)

	defer ash.CleanUpDesks(cleanupCtx, tconn)
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Ensure there is no window open before test starts.
	if err := ash.CloseAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to ensure no window is open: ", err)
	}

	ac := uiauto.New(tconn)

	pc := pointer.NewMouse(tconn)
	defer pc.Close()

	// Open a browser window.
	browserApp, err := apps.PrimaryBrowser(ctx, tconn)
	if err != nil {
		s.Fatal("Could not find browser app info: ", err)
	}
	if err := apps.Launch(ctx, tconn, browserApp.ID); err != nil {
		s.Fatal("Failed to launch chrome: ", err)
	}
	if err := ash.WaitForApp(ctx, tconn, browserApp.ID, time.Minute); err != nil {
		s.Fatal("Browser did not appear in shelf after launch: ", err)
	}
	// Ensure that there is only one open window that is the primary browser. Wait for the browser to be visible to avoid a race that may cause test flakiness.
	bt := s.Param().(browser.Type)
	bw, err := wmputils.EnsureOnlyBrowserWindowOpen(ctx, tconn, bt)
	if err != nil {
		s.Fatal("Expected the window to be fullscreen but got: ", err)
	}

	// Enter overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}
	defer ash.SetOverviewModeAndWait(cleanupCtx, tconn, false)

	// 1. Tests that desks bar will be transformed to expanded state when dragging a window
	// towards and close enough to the new desk button. And then dropping the window outside
	// of the new desk button will let desk bar go back to zero state.

	newDeskButtonView := nodewith.ClassName("ZeroStateIconButton")
	newDeskButtonViewLoc, err := ac.Location(ctx, newDeskButtonView)
	if err != nil {
		s.Fatal(err, "failed to get the location of new desk button view")
	}

	ws, err := ash.GetAllWindows(ctx, tconn)
	if len(ws) != 1 {
		s.Fatalf("Got %d window(s), Expected 1 window", len(ws))
	}
	bw = ws[0]

	// Drag the window towoard to the new desk button without dropping it. Since it's close
	// enough to the new desk button, the desks bar view should be transformed to its expanded
	// state.
	if err := uiauto.Combine("move mouse on the chrome window and then drag the window to the new desk button",
		mouse.Move(tconn, bw.BoundsInRoot.CenterPoint(), 0),
		mouse.Press(tconn, mouse.LeftButton),
		mouse.Move(tconn, newDeskButtonViewLoc.CenterPoint(), 2*time.Second),
	)(ctx); err != nil {
		s.Fatal("Failed to drag browser to the new desk button")
	}

	// Desks bar should be at expanded state now.
	desksBarView := nodewith.ClassName("DesksBarView")
	desksBarViewLoc, err := ac.Location(ctx, desksBarView)
	if err != nil {
		s.Fatal("Failed to get the location of the desks bar view: ", err)
	}
	if desksBarViewLoc.Height == zeroStateDesksBarHeight {
		s.Fatal("Failed to go to desks bar's expanded state")
	}

	// Continue dragging the window to the outside of the new desk button and then release mouse
	// which will drop the window. Since the window is dropped outside of the new desk button,
	// it will fall back to the current desk and the desks bar should be back to zero state.
	if err := uiauto.Combine("drag the window to the outside of the new desk button and then release mouse",
		mouse.Move(tconn, newDeskButtonViewLoc.CenterPoint().Add(coords.NewPoint(100, 100)), time.Second),
		mouse.Release(tconn, mouse.LeftButton),
	)(ctx); err != nil {
		s.Fatal("Failed to drag browser to the new desk button")
	}

	// Desks bar should be transformed back to the zero state.
	desksBarView = nodewith.ClassName("DesksBarView")
	desksBarViewLoc, err = ac.Location(ctx, desksBarView)
	if err != nil {
		s.Fatal("Failed to get the location of the desks bar view: ", err)
	}
	if desksBarViewLoc.Height != zeroStateDesksBarHeight {
		s.Fatal("Failed to go back to desks bar's zero state")
	}

	// 2. Tests that dragging and dropping a window to the new desk button will create a new
	// desk and the window being dragged is moved to the new desk at the same time.

	// Drag browser window to the new desk button.
	newDeskButtonView = nodewith.ClassName("ZeroStateIconButton")
	newDeskButtonViewLoc, err = ac.Location(ctx, newDeskButtonView)
	if err != nil {
		s.Fatal(err, "Failed to get the location of the new desk button view")
	}

	if err := pc.Drag(
		bw.BoundsInRoot.CenterPoint(),
		pc.DragTo(newDeskButtonViewLoc.CenterPoint(), 2*time.Second))(ctx); err != nil {
		s.Fatal("Failed to drag browser window into the new desk button: ", err)
	}

	// Verifies that a new desk is created.
	deskMiniViewsInfo, err := ash.FindDeskMiniViews(ctx, ac)
	if err != nil {
		s.Fatal("Failed to find desks: ", err)
	}
	if len(deskMiniViewsInfo) != 2 {
		s.Fatalf("Got %v desks, want 2 desks", len(deskMiniViewsInfo))
	}

	// Checks that the browser window is in the new desk. The new desk is inactive.
	ws, err = ash.GetAllWindows(ctx, tconn)
	if len(ws) != 1 {
		s.Fatalf("Got %d window(s), Expected 1 window", len(ws))
	}
	bw = ws[0]
	if bw.OnActiveDesk == true {
		s.Fatal("Browser window should be in the inactive desk")
	}
}
