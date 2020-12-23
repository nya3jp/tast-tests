// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"image"
	"image/color"
	"time"

	"chromiumos/tast/errors"
	androidui "chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/screenshot"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShelfIcons,
		Desc:         "Tests that ARC++ windows are represented in the shelf correctly, including grouping of windows and custom icons",
		Contacts:     []string{"phweiss@chromium.org", "giovax@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Data:         []string{"ArcShelfIconTest.apk"},
		Timeout:      30 * time.Second,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

// ShelfIcons opens 4 windows of an app that will set custom icons and window
// titles. The 4 windows will be grouped in groups of 2. The test then checks
// that the grouping is respected by the shelf, and that the icons and titles
// are displayed correctly when clicking the shelf entries.
func ShelfIcons(ctx context.Context, s *testing.State) {
	const (
		apk = "ArcShelfIconTest.apk"
		pkg = "org.chromium.arc.testapp.arcshelficontest"
		cls = "org.chromium.arc.testapp.arcshelficontest.ShelfIconActivity"

		idPrefix       = "org.chromium.arc.testapp.arcshelficontest:id/"
		titleID        = idPrefix + "windowtitle"
		titleSuffix    = "_title"
		appTitle       = "ArcShelfIconTest"
		menuIconOffset = 8

		colorMaxDiff = 32
	)
	var (
		iconColors     = []string{"white", "red", "green", "blue"}
		windowGroups   = []string{"group1", "group2", "group2", "group1"}
		expGroupTitles = [][]string{{"blue", "white"}, {"green", "red"}}
		expGroupColor  = [][]color.RGBA{{color.RGBA{0, 0, 255, 255}, color.RGBA{255, 255, 255, 255}}, {color.RGBA{0, 255, 0, 255}, color.RGBA{255, 0, 0, 255}}}
	)

	p := s.FixtValue().(*arc.PreData)
	cr := p.Chrome
	a := p.ARC

	s.Log("Creating Test API connection")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	s.Log("Initializing UI Automator")
	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	s.Log("Installing app")
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}
	for i, color := range iconColors {
		// This command creates a new app window.
		if err := a.Command(ctx,
			"am", "start", "-W", "-n", pkg+"/"+cls,
			// These flags (NEW_TASK, MULTIPLE_TASK) ensure that a new window is created.
			"-f", "0x18000000",
			// Specify color, window title and window grouping via extras.
			"-e", "color", color,
			"-e", "title", color+titleSuffix,
			"-e", "org.chromium.arc.shelf_group_id", windowGroups[i],
		).Run(); err != nil {
			s.Fatal("Failed starting app: ", err)
		}
		s.Log("Waiting for " + color + " app to show up")
		if err := d.Object(androidui.ID(titleID), androidui.Text(color+titleSuffix)).WaitForExists(ctx, 30*time.Second); err != nil {
			s.Fatal("Failed to wait for the app shown: ", err)
		}
	}
	// In the end, close all windows.
	defer a.Command(ctx, "pm", "clear", pkg).Run()

	// Get the root view to translate coordinates in different resolutions.
	root, err := ui.Root(ctx, tconn)
	if err != nil {
		s.Fatal("Could not find root node: ", err)
	}

	params := ui.FindParams{ClassName: "ash/ShelfAppButton", Name: "ArcShelfIconTest"}
	buttons, err := ui.FindAll(ctx, tconn, params)
	if err != nil {
		s.Fatal("Failed to find shelf button: ", err)
	}
	defer buttons.Release(ctx)
	if len(buttons) != 2 {
		s.Fatalf("Unexpected number of ArcShelfIconTest buttons: got %d, expected 2", len(buttons))
	}
	// There should be 2 buttons in the shelf that belong to the testing app.
	// First, group1 with a blue icon (with the white and blue window
	// belonging to this group), then group2 with the green icon (with the
	// red and green window belong to this group). The order of the buttons
	// is fixed, since the white window of group1 was created first.
	// We will click both of these buttons and check the context menu
	// if the right windows are being listed, and then check the button color.
	for i, button := range buttons {
		if err := button.LeftClick(ctx); err != nil {
			s.Fatalf("Failed to click button %d: %s", i+1, err)
		}
		// Wait for menu animation to finish. It should take less than 150ms.
		// If we didn't wait long enough, we will retry screenshotting
		// when we start comparing colors.
		if err := testing.Sleep(ctx, 150*time.Millisecond); err != nil {
			s.Fatal("Waiting for finished animation failed: ", err)
		}
		img, err := screenshot.GrabScreenshot(ctx, cr)
		if err != nil {
			s.Fatal("Failed to grab screenshot: ", err)
		}
		menuItemsParams := ui.FindParams{ClassName: "MenuItemView"}
		menuItems, err := ui.FindAll(ctx, tconn, menuItemsParams)
		if err != nil {
			s.Fatal("Could not find menu items: ", err)
		}
		// The menu for one group should list 3 items: a header and one per window in the group
		if len(menuItems) != len(expGroupTitles[i])+1 {
			itemLog := "menuItems: "
			for _, item := range menuItems {
				itemLog += "\"" + item.Name + "\" "
			}
			s.Log(itemLog)
			s.Fatalf("Found %d menu items, expected %d", len(menuItems), len(expGroupTitles[i])+1)
		}
		// Check if animation of menu item is finished. The menu is simply fading in, samplePt stays constant.
		samplePt := getIconCoordsFromUINode(menuItems[1], menuIconOffset, img, root)
		expColor := expGroupColor[i][0]
		sampleColor := img.At(samplePt.X, samplePt.Y)
		if !colorcmp.ColorsMatch(sampleColor, expColor, colorMaxDiff) {
			s.Logf("Color %v is not close enough to %v, repeating screenshot in case animation is not done", sampleColor, expColor)
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				img, err = screenshot.GrabScreenshot(ctx, cr)
				if err != nil {
					s.Fatal("Failed to grab screenshot: ", err)
				}
				sampleColor = img.At(samplePt.X, samplePt.Y)
				if !colorcmp.ColorsMatch(sampleColor, expColor, colorMaxDiff) {
					return errors.Errorf("Waiting for color %v to be close to %v", sampleColor, expColor)
				}
				return nil
			}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
				s.Fatal("Failed to wait for context menu animation to finish: ", err)
			}
		}
		// Animation is done, compare all menu titles and icons.
		for j, menuItem := range menuItems {
			// The first item is the header without icon.
			if j == 0 {
				if menuItem.Name != appTitle {
					s.Errorf("Got menu title %s, expected %s", menuItem.Name, appTitle)
				}
				continue
			}
			expColor := expGroupColor[i][j-1]
			expTitle := expGroupTitles[i][j-1] + titleSuffix
			if menuItem.Name != expTitle {
				s.Errorf("Got menu title %d, %d as %s, expected %s", i, j, menuItem.Name, expTitle)
			}
			samplePt := getIconCoordsFromUINode(menuItem, menuIconOffset, img, root)
			iconColor := img.At(samplePt.X, samplePt.Y)
			if !colorcmp.ColorsMatch(iconColor, expColor, colorMaxDiff) {
				s.Errorf("Got icon color %v, not close enough to %v", iconColor, expColor)
			}
		}
		// Check icon on the shelf button.
		buttonCoords := getIconCoordsFromUINode(button, 0 /*iconOffset*/, img, root)
		buttonColor := img.At(buttonCoords.X, buttonCoords.Y)
		if !colorcmp.ColorsMatch(buttonColor, expGroupColor[i][0], colorMaxDiff) {
			s.Errorf("Button color %v was not close enough to %v", buttonColor, expGroupColor[i])
		}
		if err := button.LeftClick(ctx); err != nil {
			s.Fatal("Could not close context menu: ", err)
		}
	}
}

// adjustResolution is a helper function to convert a point from the resolution
// of the ui.Node coordinate system to the coordinate system of a screenshot.
func adjustResolution(p coords.Point, img image.Image, root *ui.Node) coords.Point {
	return coords.NewPoint(p.X*img.Bounds().Dx()/root.Location.Width, p.Y*img.Bounds().Dy()/root.Location.Height)
}

// getIconCoordsFromUINode takes a UI Node, and outputs the coordinates of
// a pixel that should be close to the center of the item's icon.
func getIconCoordsFromUINode(item *ui.Node, iconOffset int, img image.Image, root *ui.Node) coords.Point {
	// This is the layout of one menu item, we want the color of O:
	// /-------------------\
	// |  /-\              |
	// |  |O| green_title  |
	// |  \-/              |
	// \-------------------/
	//
	// |--| iconOffset
	//
	// This is the layout of a shelf button, iconOffset=0:
	// /---\
	// |/-\|
	// ||O||
	// |\-/|
	// \---/
	topLeftPt := item.Location.TopLeft()
	offsetPt := coords.NewPoint(iconOffset+item.Location.Height/2, item.Location.Height/2)
	samplePt := adjustResolution(topLeftPt.Add(offsetPt), img, root)
	return samplePt
}
