// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"image"
	"image/color"
	"math"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SurfaceOrientation,
		Desc:         "Test the various orientations of an ARC activity window surface",
		Contacts:     []string{"srok@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_vm_r"},
		}},
	})
}

// Values are from http://cs/android/system/core/libsystem/include/system/graphics-base-v1.0.h?l=36&rcl=c86e2898d7575b42133132ccf72e4546ea23d3d9
const (
	nativeWindowTransformNormal    = 0
	nativeWindowTransformFlipH     = 1
	nativeWindowTransformFlipV     = 2
	nativeWindowTransformRotate90  = 4
	nativeWindowTransformRotate180 = 3
	nativeWindowTransformRotate270 = 7
)

type expectedQuadrantColors struct {
	TopLeft     color.Color
	TopRight    color.Color
	BottomLeft  color.Color
	BottomRight color.Color
}

type quadrant int

const (
	quadTopLeft = iota
	quadTopRight
	quadBottomLeft
	quadBottomRight
)

// Increasing this will allow more variance in the colors observed from the screenshot
// compared to what is expected. Decreasing will require the colors to be more exact.
const maxRGBValueDiff = 10

func SurfaceOrientation(ctx context.Context, s *testing.State) {

	const (
		apk = "ArcSurfaceOrientationTest.apk"
		pkg = "org.chromium.arc.testapp.surfaceorientation"
		cls = ".MainActivity"
	)

	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--force-tablet-mode=clamshell"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to create ARC instance: ", err)
	}
	defer a.Close(ctx)

	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	red := color.RGBA{255, 0, 0, 255}
	green := color.RGBA{0, 255, 0, 255}
	blue := color.RGBA{0, 0, 255, 255}
	yellow := color.RGBA{255, 255, 0, 255}

	var colorsDidNotMatchErr error
	colorsDidNotMatch := false

	for _, tc := range []struct {
		Name               string
		Transform          int
		ExpectedQuadColors expectedQuadrantColors
	}{
		// TODO: Make sure that these rotations are clockwise like you are now assuming.
		{"NoTransform", nativeWindowTransformNormal, expectedQuadrantColors{TopLeft: red, TopRight: green,
			BottomLeft: blue, BottomRight: yellow}},
		{"FlipHorizontal", nativeWindowTransformFlipH, expectedQuadrantColors{TopLeft: green, TopRight: red,
			BottomLeft: yellow, BottomRight: blue}},
		{"FlipVertical", nativeWindowTransformFlipV, expectedQuadrantColors{TopLeft: blue, TopRight: yellow,
			BottomLeft: red, BottomRight: green}},
		{"Rotate90", nativeWindowTransformRotate90, expectedQuadrantColors{TopLeft: blue, TopRight: red,
			BottomLeft: yellow, BottomRight: green}},
		{"Rotate180", nativeWindowTransformRotate180, expectedQuadrantColors{TopLeft: yellow, TopRight: blue,
			BottomLeft: green, BottomRight: red}},
		{"Rotate270", nativeWindowTransformRotate270, expectedQuadrantColors{TopLeft: green, TopRight: yellow,
			BottomLeft: red, BottomRight: blue}},
		// TODO: Test inverse display transform once you understand what it is
	} {
		if err := act.StartWithArgs(ctx, tconn, []string{}, []string{"--ei", "transform", strconv.Itoa(tc.Transform)}); err != nil {
			s.Fatal("Failed to start activity: ", err)
		}

		if err := ash.WaitForVisible(ctx, tconn, act.PackageName()); err != nil {
			s.Fatal("Failed to wait for activity to be visible: ", err)
		}

		if err := act.SetWindowState(ctx, tconn, arc.WindowStateMaximized); err != nil {
			s.Fatal("Failed to set the activity to Maximized: ", err)
		}

		if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateMaximized); err != nil {
			s.Fatal("Failed to wait for activity to enter Maximized state: ", err)
		}

		img, err := screenshot.GrabScreenshot(ctx, cr)
		if err != nil {
			s.Fatal("Failed to take screen: ", err)
		}

		windowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, act.PackageName())
		if err != nil {
			s.Fatal("Failed to get arc app window info: ", err)
		}

		dispMode, err := ash.PrimaryDisplayMode(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get display mode of the primary display: ", err)
		}

		captionHeight := int(math.Round(float64(windowInfo.CaptionHeight) * dispMode.DeviceScaleFactor))

		bounds, err := act.WindowBounds(ctx)
		if err != nil {
			s.Fatal("Could not get activity window bounds: ", err)
		}
		// The rectangle encapsulating the visible area that will be colored in the activity
		visibleBounds := image.Rect(bounds.Left, bounds.Top+captionHeight, bounds.Width, bounds.Height)

		for _, quadInfo := range []struct {
			Name          string
			Quad          quadrant
			ExpectedColor color.Color
		}{
			{"TopLeft", quadTopLeft, tc.ExpectedQuadColors.TopLeft},
			{"TopRight", quadTopRight, tc.ExpectedQuadColors.TopRight},
			{"BottomLeft", quadBottomLeft, tc.ExpectedQuadColors.BottomLeft},
			{"BottomRight", quadBottomRight, tc.ExpectedQuadColors.BottomRight},
		} {
			observedColor := img.At(getCenterXYOfQuadrant(quadInfo.Quad, visibleBounds))
			if !colorcmp.ColorsMatch(observedColor, quadInfo.ExpectedColor, maxRGBValueDiff) {
				// Allow all (orientation, quadrant) combinations to run this pixel color check and do not fail immediately
				colorsDidNotMatchErr = errors.Wrapf(colorsDidNotMatchErr, "Test case with reorientation %q had wrong color in %s quadrant, colors observed=%v expected=%v",
					tc.Name, quadInfo.Name, observedColor, quadInfo.ExpectedColor)
				colorsDidNotMatch = true
			}
		}

		if err := act.Stop(ctx, tconn); err != nil {
			s.Fatal("Failed to stop activity: ", err)
		}
	}

	if colorsDidNotMatch {
		s.Fatal("Pixel color match failed: ", colorsDidNotMatchErr)
	}
}

func getCenterXYOfQuadrant(quadrant quadrant, rect image.Rectangle) (int, int) {
	w := rect.Max.X - rect.Min.X
	h := rect.Max.Y - rect.Min.Y
	switch quadrant {
	case quadTopLeft:
		return rect.Min.X + w/4, rect.Min.Y + h/4
	case quadTopRight:
		return rect.Min.X + w*3/4, rect.Min.Y + h/4
	case quadBottomLeft:
		return rect.Min.X + w/4, rect.Min.Y + h*3/4
	default /* quadBottomRight */ :
		return rect.Min.X + w*3/4, rect.Min.Y + h*3/4
	}
}
