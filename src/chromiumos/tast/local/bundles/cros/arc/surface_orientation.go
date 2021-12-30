// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

const (
	arcSurfaceOrientationTestApkFilename = "ArcSurfaceOrientationTest.apk"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SurfaceOrientation,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Test the various orientations of an ARC activity window surface",
		Contacts:     []string{"srok@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Fixture:      "arcBooted",
		Data:         []string{arcSurfaceOrientationTestApkFilename},
		Timeout:      5 * time.Minute,
	})
}

// Values coming from:
// https://cs.android.com/android/platform/superproject/+/HEAD:frameworks/native/libs/nativewindow/include/android/native_window.h;drc=50e37fefe51554d300c28b496c942c0bab299fee
const (
	nativeWindowTransformNormal    = 0
	nativeWindowTransformFlipH     = 1
	nativeWindowTransformFlipV     = 2
	nativeWindowTransformRotate90  = 4
	nativeWindowTransformRotate180 = nativeWindowTransformFlipH | nativeWindowTransformFlipV
	nativeWindowTransformRotate270 = nativeWindowTransformRotate180 | nativeWindowTransformRotate90
	// These two values are not explicitly listed in the native window API enums, but are a logical
	// combination of transforms to complete all the possible simple transforms
	nativeWindowTransformFlipHRotate90 = nativeWindowTransformFlipH | nativeWindowTransformRotate90
	nativeWindowTransformFlipVRotate90 = nativeWindowTransformFlipV | nativeWindowTransformRotate90
)

type expectedQuadrantColors struct {
	topLeft     color.Color
	topRight    color.Color
	bottomLeft  color.Color
	bottomRight color.Color
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
const maxRGBValueDiff = 30

func SurfaceOrientation(ctx context.Context, s *testing.State) {

	const (
		pkg = "org.chromium.arc.testapp.surfaceorientation"
		cls = ".MainActivity"
	)

	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC
	d := s.FixtValue().(*arc.PreData).UIDevice

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	if err := a.Install(ctx, s.DataPath(arcSurfaceOrientationTestApkFilename)); err != nil {
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

	// TODO(b/203800119): Add testcases which use multiple transformations in serial.

	for _, tc := range []struct {
		name               string
		transform          int
		expectedQuadColors expectedQuadrantColors
	}{
		{"NoTransform", nativeWindowTransformNormal, expectedQuadrantColors{topLeft: red, topRight: green,
			bottomLeft: blue, bottomRight: yellow}},
		{"FlipHorizontal", nativeWindowTransformFlipH, expectedQuadrantColors{topLeft: green, topRight: red,
			bottomLeft: yellow, bottomRight: blue}},
		{"FlipVertical", nativeWindowTransformFlipV, expectedQuadrantColors{topLeft: blue, topRight: yellow,
			bottomLeft: red, bottomRight: green}},
		{"Rotate90", nativeWindowTransformRotate90, expectedQuadrantColors{topLeft: blue, topRight: red,
			bottomLeft: yellow, bottomRight: green}},
		{"Rotate180", nativeWindowTransformRotate180, expectedQuadrantColors{topLeft: yellow, topRight: blue,
			bottomLeft: green, bottomRight: red}},
		{"Rotate270", nativeWindowTransformRotate270, expectedQuadrantColors{topLeft: green, topRight: yellow,
			bottomLeft: red, bottomRight: blue}},
		{"FlipHorizontalRotate90", nativeWindowTransformFlipHRotate90, expectedQuadrantColors{topLeft: yellow, topRight: green,
			bottomLeft: blue, bottomRight: red}},
		{"FlipVerticalRotate90", nativeWindowTransformFlipVRotate90, expectedQuadrantColors{topLeft: red, topRight: blue,
			bottomLeft: green, bottomRight: yellow}},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			if err := act.Start(ctx, tconn,
				arc.WithWindowingMode(arc.WindowingModeFullscreen),
				arc.WithExtraInt("transform", tc.transform),
			); err != nil {
				s.Fatal("Failed to start activity: ", err)
			}
			defer act.Stop(ctx, tconn)

			if err := ash.WaitForVisible(ctx, tconn, act.PackageName()); err != nil {
				s.Fatal("Failed to wait for activity to be visible: ", err)
			}

			if err := d.WaitForIdle(ctx, time.Second); err != nil {
				s.Fatal("Failed to wait for idle: ", err)
			}

			img, err := screenshot.GrabScreenshot(ctx, cr)
			if err != nil {
				s.Fatal("Failed to take screenshot: ", err)
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
				s.Fatal("Failed to get activity window bounds: ", err)
			}
			// The rectangle encapsulating the visible area that will be colored in the activity
			visibleBounds := image.Rect(bounds.Left, bounds.Top+captionHeight, bounds.Width, bounds.Height)

			if err := act.Stop(ctx, tconn); err != nil {
				s.Fatal("Failed to stop activity: ", err)
			}

			var tcErrString strings.Builder
			tcErrString.WriteString("Test case with transformation " + tc.name)
			tcTransformColorsDidNotMatch := false
			for _, quadInfo := range []struct {
				name          string
				quad          quadrant
				expectedColor color.Color
			}{
				{"TopLeft", quadTopLeft, tc.expectedQuadColors.topLeft},
				{"TopRight", quadTopRight, tc.expectedQuadColors.topRight},
				{"BottomLeft", quadBottomLeft, tc.expectedQuadColors.bottomLeft},
				{"BottomRight", quadBottomRight, tc.expectedQuadColors.bottomRight},
			} {
				observedColor := img.At(getCenterXYOfQuadrant(quadInfo.quad, visibleBounds))
				if !colorcmp.ColorsMatch(observedColor, quadInfo.expectedColor, maxRGBValueDiff) {
					tcErrString.WriteString(fmt.Sprintf(" had wrong color in %s quadrant, colors observed=%v expected=%v; ",
						quadInfo.name, observedColor, quadInfo.expectedColor))
					tcTransformColorsDidNotMatch = true
				}
			}

			if tcTransformColorsDidNotMatch {
				colorsDidNotMatch = true
				colorsDidNotMatchErr = errors.Wrap(colorsDidNotMatchErr, tcErrString.String())

				// Save screenshot
				screenshotFileName := fmt.Sprintf("%s_screenshot_fail.png", tc.name)
				outPath, err := saveScreenshot(img, s.OutDir(), screenshotFileName)
				if err != nil {
					s.Error("Failed to save screenshot: ", err)
				} else {
					s.Logf("Screenshot saved to %s", outPath)
				}
			}
		})
	}

	if colorsDidNotMatch {
		s.Fatal("Pixel color match failed: ", colorsDidNotMatchErr)
	}
}

func saveScreenshot(img image.Image, outDir, fileName string) (string, error) {
	path := filepath.Join(outDir, fileName)
	fd, err := os.Create(path)
	if err != nil {
		return "", errors.Wrap(err, "failed to create screenshot")
	}
	defer fd.Close()
	if err := png.Encode(fd, img); err != nil {
		return "", errors.Wrap(err, "failed to encode screenshot to png format")
	}
	return path, nil
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
