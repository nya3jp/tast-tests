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
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

const arcSurfaceOrientationTestApkFilename = "ArcSurfaceOrientationTest.apk"

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

const windowingModeFullscreen = 1

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
		Name               string
		Transform          int
		ExpectedQuadColors expectedQuadrantColors
	}{
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
		{"FlipHorizontalRotate90", nativeWindowTransformFlipHRotate90, expectedQuadrantColors{TopLeft: yellow, TopRight: green,
			BottomLeft: blue, BottomRight: red}},
		{"FlipVerticalRotate90", nativeWindowTransformFlipVRotate90, expectedQuadrantColors{TopLeft: red, TopRight: blue,
			BottomLeft: green, BottomRight: yellow}},
	} {
		s.Run(ctx, tc.Name, func(ctx context.Context, s *testing.State) {
			// Fullscreen windowing mode so that screenshot scanning is easier
			prefixes := []string{"--windowingMode", strconv.Itoa(windowingModeFullscreen),
				"--ei", "transform", strconv.Itoa(tc.Transform)}

			if err := act.StartWithArgs(ctx, tconn, prefixes, []string{}); err != nil {
				s.Fatal("Failed to start activity: ", err)
			}
			defer act.Stop(ctx, tconn)

			if err := ash.WaitForVisible(ctx, tconn, act.PackageName()); err != nil {
				s.Fatal("Failed to wait for activity to be visible: ", err)
			}

			if err := d.WaitForIdle(ctx, time.Second); err != nil {
				s.Fatal("Failed to wait for idle: ", err)
			}

			testing.Poll(ctx, func(ctx context.Context) error {
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
				tcErrString.WriteString("Test case with transformation " + tc.Name)
				tcTransformColorsDidNotMatch := false
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
						tcErrString.WriteString(fmt.Sprintf(" had wrong color in %s quadrant, colors observed=%v expected=%v; ",
							quadInfo.Name, observedColor, quadInfo.ExpectedColor))
						tcTransformColorsDidNotMatch = true
					}
				}

				if tcTransformColorsDidNotMatch {
					colorsDidNotMatch = true
					colorsDidNotMatchErr = errors.Wrap(colorsDidNotMatchErr, tcErrString.String())

					// Save screenshot
					screenshotFileName := fmt.Sprintf("%s_screeshot_fail.png", tc.Name)
					outPath, err := saveScreenshot(img, s.OutDir(), screenshotFileName)
					if err != nil {
						s.Error("Failed to save screenshot: ", err)
					} else {
						s.Logf("Screenshot saved to %s", outPath)
					}
					return colorsDidNotMatchErr
				}

				return nil
			}, &testing.PollOptions{Timeout: 5 * time.Second})
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
