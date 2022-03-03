// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// #14 Change position of display relative to Chromebook

// Pre-Condition:
// (Please note: Brand / Model number on test result)
// 1. External displays (Single /Dual)
// 2. Docking station /Hub /Dongle
// 3. Connection Type (HDMI/DP/VGA/DVI/USB-C on test result)

// Procedure:
// 1) Boot-up and Sign-In to the device
// 2) Connect ext-display to (Docking station or Hub)
// 3) Connect (Docking station or Hub) to Chromebook
// 4) Go to "Quick Settings Menu and Setting /Device /Displays
//  By default "Primary" (Built-in displays) show on the Left side of the (Ext-Displays)
// 5) Click+Hold the displays (Primary) or (Extended) ext-displays icon around (i.e. Left/Right/Top/Bottom)
// 6) On "Primary" (Built-in displays) open Chrome browser window and drag the browser window onto (Extended) ext- displays

// Verification:
// 5) Make sure the display screen show "BLUE" highlighted border around the display and able to drag around without any issue
// 6) Make sure able to drag the browser window around to the (Primary or Extended) displays

// Package wwcb contains local Tast tests that work with chromebook
package wwcb

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/wwcb/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

type directionType int

const (
	directionTop    directionType = 0
	directionBottom directionType = 1
	directionLeft   directionType = 2
	directionRight  directionType = 3
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock14ChangePosition,
		Desc:         "Change position of display relative to Chromebook",
		Contacts:     []string{"flin@google.com", "newmanliu19020@allion.corp-partner.google.com"},
		SoftwareDeps: []string{"arc", "chrome"},
		Timeout:      10 * time.Minute,
		Pre:          arc.Booted(),
		Vars:         []string{"WWCBIP", "DockingID", "1stExtDispID"},
	})
}

func Dock14ChangePosition(ctx context.Context, s *testing.State) {
	extDispID := s.RequiredVar("1stExtDispID")
	dockingID := s.RequiredVar("DockingID")

	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)

	settings, err := ossettings.LaunchAtPage(ctx, tconn, nodewith.Name("Device").Role(role.Link))
	if err != nil {
		s.Fatal("Failed to launch os-settings Device page: ", err)
	}
	defer settings.Close(ctx)

	displayFinder := nodewith.Name("Displays").Role(role.Link).Ancestor(ossettings.WindowFinder)
	if err := ui.LeftClickUntil(displayFinder, ui.WithTimeout(3*time.Second).WaitUntilGone(displayFinder))(ctx); err != nil {
		s.Fatal("Failed to launch display page: ", err)
	}

	pc := pointer.NewMouse(tconn)

	for _, relayout := range []struct {
		name      string
		direction directionType
	}{
		{"Relayout external display on top of internal display", directionTop},
		{"Relayout external display on bottom of internal display", directionBottom},
		{"Relayout external display to the left side of internal display", directionLeft},
		{"Relayout external display to the right side of internal display", directionRight},
	} {
		testing.ContextLog(ctx, relayout.name)

		finder := nodewith.ClassName("display elevate")

		intDispFinder := finder.Nth(0)
		intDispFinderLoc, err := ui.Location(ctx, intDispFinder)
		if err != nil {
			s.Fatal("Failed to find internal display finder location: ", err)
		}

		extDispFinder := finder.Nth(1)
		extDispFinderLoc, err := ui.Location(ctx, extDispFinder)
		if err != nil {
			s.Fatal("Failed to find external display finder location: ", err)
		}

		// Calculate internal display new position.
		crossMargin := 20
		var destination coords.Point
		switch relayout.direction {
		case directionTop:
			height := (intDispFinderLoc.Height+extDispFinderLoc.Height)/2 + crossMargin
			destination = coords.NewPoint(extDispFinderLoc.CenterPoint().X, extDispFinderLoc.CenterPoint().Y-height)
		case directionBottom:
			height := (intDispFinderLoc.Height+extDispFinderLoc.Height)/2 + crossMargin
			destination = coords.NewPoint(extDispFinderLoc.CenterPoint().X, extDispFinderLoc.CenterPoint().Y+height)
		case directionLeft:
			width := (intDispFinderLoc.Width+extDispFinderLoc.Width)/2 + crossMargin
			destination = coords.NewPoint(extDispFinderLoc.CenterPoint().X-width, extDispFinderLoc.CenterPoint().Y)
		case directionRight:
			width := (intDispFinderLoc.Width+extDispFinderLoc.Width)/2 + crossMargin
			destination = coords.NewPoint(extDispFinderLoc.CenterPoint().X+width, extDispFinderLoc.CenterPoint().Y)
		default:
			s.Fatal("unable to find out destination")
		}

		// Move internal display to desitnation.
		if err := pc.Drag(
			intDispFinderLoc.CenterPoint(),
			pc.DragTo(destination, time.Second))(ctx); err != nil {
			s.Fatal("Failed to drag finder to position: ", err)
		}

		// Waiting for Chromebook adjust display settings.
		testing.Sleep(ctx, 5*time.Second)

		infos, err := utils.GetInternalAndExternalDisplays(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get internal and external display: ", err)
		}
		testing.ContextLog(ctx, infos.External.Bounds)

		// Open the Files App.
		files, err := filesapp.Launch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to launch Files app: ", err)
		}

		win, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool { return w.Title == "Files - My files" })
		if err != nil {
			s.Fatal("Failed to find files: ", err)
		}

		headerPoint := coords.NewPoint(win.BoundsInRoot.Left+win.BoundsInRoot.Width/2, win.BoundsInRoot.Top+win.CaptionHeight/2)

		// Calculate window on external display position.
		crossDisplayMargin := 100
		var winDestination coords.Point
		switch relayout.direction {
		case directionTop:
			height := infos.Internal.Bounds.Height + crossDisplayMargin
			winDestination = coords.NewPoint(headerPoint.X, headerPoint.Y+height)
		case directionBottom:
			height := infos.Internal.Bounds.Height + crossDisplayMargin
			winDestination = coords.NewPoint(headerPoint.X, headerPoint.Y-height)
		case directionLeft:
			width := infos.Internal.Bounds.Width + crossDisplayMargin
			winDestination = coords.NewPoint(headerPoint.X+width, headerPoint.Y)
		case directionRight:
			width := infos.Internal.Bounds.Width + crossDisplayMargin
			winDestination = coords.NewPoint(headerPoint.X-width, headerPoint.Y)
		default:
			s.Fatal("unable to find out window destination")
		}

		// Move window to desitnation.
		if err := pc.Drag(
			headerPoint,
			pc.DragTo(winDestination, 5*time.Second))(ctx); err != nil {
			s.Fatal("Failed to drag window to destination: ", err)
		}

		// Verify window bounds on external display
		winMoved, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool { return w.Title == "Files - My files" })
		if err != nil {
			s.Fatal("Failed to find files: ", err)
		}
		if winMoved.DisplayID != infos.External.ID {
			s.Fatal("window was not showing on external display")
		}

		files.Close(ctx)
	}
	return

	cleanup, err := utils.InitFixtures(ctx)
	if err != nil {
		s.Fatal("Failed to initialize fixtures: ", err)
	}
	defer cleanup(ctx)

	testing.ContextLog(ctx, "Step 1 - Boot-up and Sign-In to the device")

	// Step 2 - Connect ext-display to docking station.
	if err := dock14ChangePositionStep2(ctx, extDispID); err != nil {
		s.Fatal("Failed to execute 2 : ", err)
	}

	// Step 3 - Connect docking station to Chromebook.
	if err := dock14ChangePositionStep3(ctx, dockingID); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}

	if err := utils.VerifyExternalDisplay(ctx, tconn, true); err != nil {
		s.Fatal("Failed to verify a external display is connected: ", err)
	}

	// Step 4, 5 - Change display relative position (top/bottom/left/right).
	if err := dock14ChangePositionStep4To5(ctx, s, tconn, a); err != nil {
		s.Fatal("Failed to execute step 4, 5: ", err)
	}

	// Step 6 - Drag window to ext-display and check window on ext-display.
	if err := dock14ChangePositionStep6(ctx, cr, tconn, a); err != nil {
		s.Fatal("Failed to execute step 6: ", err)
	}
}

func dock14ChangePositionStep2(ctx context.Context, extDispID string) error {
	testing.ContextLog(ctx, "Step 2 - Connect ext-display to docking station")
	if err := utils.SwitchFixture(ctx, extDispID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect ext-display")
	}
	return nil
}

func dock14ChangePositionStep3(ctx context.Context, dockingID string) error {
	testing.ContextLog(ctx, "Step 3 - Connect docking station to Chromebook")
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect docking station")
	}
	return nil
}

func dock14ChangePositionStep4To5(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC) error {
	testing.ContextLog(ctx, "Step 4, 5 - Change display relative position")

	if err := utils.VerifyExternalDisplay(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to verify a external display is connected")
	}

	infos, err := utils.GetInternalAndExternalDisplays(ctx, tconn)
	if err != nil {
		return err
	}

	for _, relayout := range []struct {
		name   string
		offset coords.Point
	}{
		{"Relayout external display on top of internal display", coords.NewPoint(0, -infos.External.Bounds.Height)},
		{"Relayout external display on bottom of internal display", coords.NewPoint(0, infos.Internal.Bounds.Height)},
		{"Relayout external display to the left side of internal display", coords.NewPoint(-infos.External.Bounds.Width, 0)},
		{"Relayout external display to the right side of internal display", coords.NewPoint(infos.Internal.Bounds.Width, 0)},
	} {
		p := display.DisplayProperties{BoundsOriginX: &relayout.offset.X, BoundsOriginY: &relayout.offset.Y}
		if err := display.SetDisplayProperties(ctx, tconn, infos.External.ID, p); err != nil {
			return err
		}
	}
	return nil
}

func dock14ChangePositionStep6(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, a *arc.ARC) error {
	testing.ContextLog(ctx, "Step 6 - On Primary (Built-in displays) open Chrome browser window and drag the browser window onto (Extended) ext- displays")

	if err := testing.Poll(ctx, func(c context.Context) error {
		conn, err := cr.NewConn(ctx, "https://www.google.com", browser.WithNewWindow())
		if err != nil {
			return err
		}
		defer conn.Close()

		browser, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
			return w.WindowType == ash.WindowTypeBrowser
		})
		if err != nil {
			return errors.Wrap(err, "failed to find browser")
		}
		// defer browser.CloseWindow(ctx, tconn)

		infos, err := utils.GetInternalAndExternalDisplays(ctx, tconn)
		if err != nil {
			return err
		}

		if err := utils.MoveWindowToDisplay(ctx, tconn, browser, &infos.External); err != nil {
			return errors.Wrap(err, "failed to move window to external display")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return err
	}

	return nil
}
