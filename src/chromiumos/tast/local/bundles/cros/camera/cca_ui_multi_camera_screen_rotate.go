// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIMultiCameraScreenRotate,
		Desc:         "Opens CCA, rotate the display using display APIs and take picture or record video in every orientation using multi-cameras",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:mainline", "group:camera-libcamera", "informational"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Fixture:      "ccaLaunched",
		Params: []testing.Param{{
			Name: "photo",
			Val:  cca.Photo,
		}, {
			Name: "video",
			Val:  cca.Video,
		}},
	})
}

// CCAUIMultiCameraScreenRotate Open CCA, rotate the display to either take
// picture or record video using all available cameras.
func CCAUIMultiCameraScreenRotate(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()

	cr := s.FixtValue().(cca.FixtureData).Chrome
	app := s.FixtValue().(cca.FixtureData).App()
	s.FixtValue().(cca.FixtureData).SetDebugParams(cca.DebugParams{SaveScreenshotWhenFail: true})
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	numCameras, err := app.GetNumOfCameras(ctx)
	if err != nil {
		s.Fatal("Failed to get number of cameras: ", err)
	}
	s.Log("No. of cameras: ", numCameras)

	mode := s.Param().(cca.Mode)

	if err := app.SwitchMode(ctx, mode); err != nil {
		s.Fatalf("Failed to switch to %v viewfinder: %v, ", mode, err)
	}

	// Get display info.
	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get internal display info: ", err)
	}
	// Revert back to initial screen orientation.
	defer func(ctx context.Context) {
		s.Log("Setting back to initial orientation")
		if err := display.SetDisplayRotationSync(ctx, tconn, dispInfo.ID, display.Rotate0); err != nil {
			s.Fatal("Failed to rotate display 0 degree: ", err)
		}
	}(cleanupCtx)

	for camera := 0; camera < numCameras; camera++ {
		if camera == 1 {
			if err := app.SwitchCamera(ctx); err != nil {
				s.Fatal("Switch camera failed: ", err)
			}
		}
		facing, err := app.GetFacing(ctx)
		if err != nil {
			s.Fatal("Failed to get facing: ", err)
		}
		s.Logf("Starting test with %v facing camera", facing)

		for _, tc := range []struct {
			name         string
			screenOrient cca.Orientation
			dispRotate   display.RotationAngle
		}{
			{"testRotate90", cca.PortraitPrimary, display.Rotate90},
			{"testRotate180", cca.LandscapeSecondary, display.Rotate180},
			{"testRotate270", cca.PortraitSecondary, display.Rotate270},
			{"testRotate360", cca.LandscapePrimary, display.Rotate0},
		} {
			s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
				if err := display.SetDisplayRotationSync(ctx, tconn, dispInfo.ID, tc.dispRotate); err != nil {
					s.Fatalf("Failed to rotate display %v degree: %v", tc.dispRotate, err)
				}
				if mode == cca.Photo {
					if _, err := app.TakeSinglePhoto(ctx, cca.TimerOff); err != nil {
						s.Fatalf("Failed to capture picture in %v subtest: %v", tc.name, err)
					}
				}
				if mode == cca.Video {
					if _, err := app.RecordVideo(ctx, cca.TimerOff, 1*time.Second); err != nil {
						s.Fatalf("Failed to record video in %v subtest: %v", tc.name, err)
					}
				}

				if err := app.SaveScreenshot(ctx); err != nil {
					s.Errorf("Failed to save a screenshot in %v subtest: %v", tc.name, err)
				}

				orient, err := app.GetScreenOrientation(ctx)
				if err != nil {
					s.Fatalf("Failed to get screen orientation in %v subtest: %v", tc.name, err)
				}
				if orient != tc.screenOrient {
					s.Fatalf("Failed to match screen orientation: got %q; want %q", orient, tc.screenOrient)
				}
			})
		}
	}
}
