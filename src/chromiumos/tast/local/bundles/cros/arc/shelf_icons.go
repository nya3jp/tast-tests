// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"image"
	"image/color"
	"time"

	"chromiumos/tast/local/arc"
	arcui "chromiumos/tast/local/arc/ui"
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
		Data:         []string{"shelf_icons_arc_shelf_icon_test.apk"},
		Timeout:      30 * time.Second,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.Booted(),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBooted(),
		}},
	})
}

// ShelfIcons opens 4 windows of an app that will set custom icons and window
// titles. The 4 windows will be grouped in groups of 2. The test then checks
// that the grouping is respected by the shelf, and that the icons and titles
// are displayed correctly when clicking the shelf entries.
func ShelfIcons(ctx context.Context, s *testing.State) {
	const (
		apk = "shelf_icons_arc_shelf_icon_test.apk"
		pkg = "org.chromium.arc.testapp.arcshelficontest"
		cls = "org.chromium.arc.testapp.arcshelficontest.ShelfIconActivity"

		idPrefix    = "org.chromium.arc.testapp.arcshelficontest:id/"
		titleID     = idPrefix + "windowtitle"
		titleSuffix = "_title"
		appTitle    = "ArcShelfIconTest"
		iconOffset  = 8

		colorMaxDiff = 32
	)
	var (
		iconColors     = []string{"white", "red", "green", "blue"}
		windowGroups   = []string{"group1", "group2", "group2", "group1"}
		expGroupTitles = [][]string{{"blue", "white"}, {"green", "red"}}
		expGroupColor  = [][]color.RGBA{{color.RGBA{0, 0, 255, 255}, color.RGBA{255, 255, 255, 255}}, {color.RGBA{0, 255, 0, 255}, color.RGBA{255, 0, 0, 255}}}
	)

	p := s.PreValue().(arc.PreData)
	cr := p.Chrome
	a := p.ARC

	s.Log("Creating Test API connection")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	s.Log("Initializing UI Automator")
	d, err := arcui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	s.Log("Installing app")
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}
	CreateWindow := func(color, group string) {
		// This command creates a new app window.
		if err := a.Command(ctx,
			"am", "start", "-W", "-n", pkg+"/"+cls,
			// These flags (NEW_TASK, MULTIPLE_TASK) ensure that a new window is created.
			"-f", "0x18000000",
			// Specify color, window title and window grouping via extras.
			"-e", "color", color,
			"-e", "title", color+titleSuffix,
			"-e", "org.chromium.arc.shelf_group_id", group,
		).Run(); err != nil {
			s.Fatal("Failed starting app: ", err)
		}
		s.Log("Waiting for " + color + " app to show up")
		if err := d.Object(arcui.ID(titleID), arcui.Text(color+titleSuffix)).WaitForExists(ctx, 30*time.Second); err != nil {
			s.Fatal("Failed to wait for the app shown: ", err)
		}
	}
	for i, color := range iconColors {
		CreateWindow(color, windowGroups[i])
	}
	// In the end, close all windows.
	defer a.Command(ctx, "pm", "clear", pkg).Run()

	// Get the root view to translate coordinates in different resolutions.
	root, err := ui.Root(ctx, tconn)
	if err != nil {
		s.Fatal("Could not find root node: ", err)
	}
	AdjustResolution := func(p coords.Point, img image.Image) coords.Point {
		return coords.NewPoint(p.X*img.Bounds().Dx()/root.Location.Width, p.Y*img.Bounds().Dy()/root.Location.Height)
	}

	params := ui.FindParams{ClassName: "ash/ShelfAppButton", Name: "ArcShelfIconTest"}
	buttons, err := ui.FindAll(ctx, tconn, params)
	if err != nil {
		s.Fatal("Failed to find shelf button: ", err)
	}
	defer buttons.Release(ctx)
	if len(buttons) != 2 {
		s.Fatalf("Unexpected number of ArcShelfIconTest buttons: expected 2, got %d", len(buttons))
	}
	for i, button := range buttons {
		if err := button.LeftClick(ctx); err != nil {
			s.Fatalf("Failed to click button %d: %s", i+1, err)
		}
		if err := testing.Sleep(ctx, 500*time.Millisecond); err != nil {
			s.Fatal("Waiting for finished animation failed: ", err)
		}
		img, err := screenshot.GrabScreenshot(ctx, cr)
		if err != nil {
			s.Fatal("Failed to grab screenshot: ", err)
		}
		c := button.Location.CenterPoint()
		c = AdjustResolution(c, img)
		if !colorcmp.ColorsMatch(img.At(c.X, c.Y), expGroupColor[i][0], colorMaxDiff) {
			s.Error("Button color ", img.At(c.X, c.Y), " was not close enough to ", expGroupColor[i])
		}
		params := ui.FindParams{ClassName: "MenuItemView"}
		items, err := ui.FindAll(ctx, tconn, params)
		if len(items) != 3 {
			s.Fatal("Did not find 3 entries, but ", items)
		}
		for j, item := range items {
			// The first item is the header without icon.
			if j == 0 {
				if item.Name != appTitle {
					s.Errorf("Expected menu title %s, got %s", appTitle, item.Name)
				}
				continue
			}
			expTitle := expGroupTitles[i][j-1] + titleSuffix
			expColor := expGroupColor[i][j-1]
			if item.Name != expTitle {
				s.Errorf("Expected menu title %d, %d to be %s, got %s", i, j, expTitle, item.Name)
			}
			// This is the layout of one menu item, we want the color of O:
			// /-------------------\
			// |  /-\              |
			// |  |O| green_title  |
			// |  \-/              |
			// \-------------------/
			//
			// |--| iconOffset
			//
			point := item.Location.TopLeft()
			point.X += iconOffset + item.Location.Height/2
			point.Y += item.Location.Height / 2
			point = AdjustResolution(point, img)
			if !colorcmp.ColorsMatch(img.At(point.X, point.Y), expColor, colorMaxDiff) {
				s.Errorf("Color %v of icon %d,%d was not close enough to %v", img.At(point.X, point.Y), i, j, expColor)
			}
		}
		if err := button.LeftClick(ctx); err != nil {
			s.Fatal("Could not close context menu: ", err)
		}
	}
}
