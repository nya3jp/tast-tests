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
		screenOrient := []cca.Orientation{cca.PortraitPrimary, cca.LandscapeSecondary, cca.PortraitSecondary}
		dispRotates := []display.RotationAngle{display.Rotate90, display.Rotate180, display.Rotate270}

		for index, rotation := range dispRotates {
			if err := display.SetDisplayRotationSync(ctx, tconn, dispInfo.ID, rotation); err != nil {
				s.Fatalf("Failed to rotate display %v degree: %v", rotation, err)
			}

			if mode == cca.Photo {
				if _, err := app.TakeSinglePhoto(ctx, cca.TimerOff); err != nil {
					s.Fatal("Failed to capture picture: ", err)
				}
			}
			if mode == cca.Video {
				if _, err := app.RecordVideo(ctx, cca.TimerOff, 1*time.Second); err != nil {
					s.Fatal("Failed to record video: ", err)
				}
			}

			if err := app.SaveScreenshot(ctx); err != nil {
				s.Error("Failed to save a screenshot: ", err)
			}

			orient, err := app.GetScreenOrientation(ctx)
			if err != nil {
				s.Fatal("Failed to get screen orientation: ", err)
			}
			if orient != screenOrient[index] {
				s.Fatalf("Failed to match screen orientation: got %q; want %q", orient, screenOrient[index])
			}
		}
	}
}
