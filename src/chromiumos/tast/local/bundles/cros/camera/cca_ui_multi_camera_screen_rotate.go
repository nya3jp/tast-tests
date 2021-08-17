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
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
)

type testParam struct {
	facing cca.Facing
	mode   cca.Mode
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIMultiCameraScreenRotate,
		Desc:         "Opens CCA, rotate the display using display APIs and take picture or record video in every orientation using multi-cameras",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:mainline", "group:camera-libcamera", "informational"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoggedIn(),
		Params: []testing.Param{{
			Name: "user_facing_photo",
			Val: testParam{
				facing: cca.FacingFront,
				mode:   cca.Photo,
			},
		}, {
			Name: "user_facing_video",
			Val: testParam{
				facing: cca.FacingFront,
				mode:   cca.Video,
			},
		}, {
			Name: "env_facing_photo",
			Val: testParam{
				facing: cca.FacingBack,
				mode:   cca.Photo,
			},
		}, {
			Name: "env_facing_video",
			Val: testParam{
				facing: cca.FacingBack,
				mode:   cca.Video,
			},
		}},
	})
}

// Open CCA, rotate the display to either take picture or record video using front camera or back camera.
func CCAUIMultiCameraScreenRotate(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()

	cr := s.PreValue().(*chrome.Chrome)
	tb, err := testutil.NewTestBridge(ctx, cr, testutil.UseRealCamera)
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	defer tb.TearDown(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer tconn.Close()

	if err := cca.ClearSavedDir(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir(), tb)
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer func(ctx context.Context) {
		if err := app.Close(ctx); err != nil {
			s.Error("Failed to close camera: ", err)
		}
	}(cleanupCtx)

	numCameras, err := app.GetNumOfCameras(ctx)
	if err != nil {
		s.Fatal("Failed to get number of cameras: ", err)
	}
	s.Log("No. of cameras: ", numCameras)

	param := s.Param().(testParam)
	// Check whether back camera is available or not.
	if param.facing == cca.FacingBack && numCameras == 1 {
		s.Fatalf("Failed to test as %v camera doesn't exist", param.facing)
	}

	// Check whether correct camera is switched.
	checkFacing := func() bool {
		facing, err := app.GetFacing(ctx)
		if err != nil {
			s.Fatal("Failed to get facing: ", err)
		}
		return facing == param.facing
	}

	// Verify the camera facing for user facing and env facing params.
	if !checkFacing() {
		if err := app.SwitchCamera(ctx); err != nil {
			s.Fatal("Failed to switch camera: ", err)
		}
		if !checkFacing() {
			s.Fatalf("Failed to get default camera facing as %v", param.facing)
		}
	}

	if err := app.SwitchMode(ctx, param.mode); err != nil {
		s.Fatalf("Failed to switch to %v viewfinder, ", param.mode, err)
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
			s.Fatal("Failed to rotate display 0 degree", err)
		}
	}(cleanupCtx)

	screenOrient := []cca.Orientation{cca.PortraitPrimary, cca.LandscapeSecondary, cca.PortraitSecondary}
	dispRotates := []display.RotationAngle{display.Rotate90, display.Rotate180, display.Rotate270}

	for index, rotation := range dispRotates {
		if err := display.SetDisplayRotationSync(ctx, tconn, dispInfo.ID, rotation); err != nil {
			s.Fatalf("Failed to rotate display %v degree", rotation, err)
		}

		if param.mode == cca.Photo {
			if _, err := app.TakeSinglePhoto(ctx, cca.TimerOff); err != nil {
				s.Fatal("Failed to capture picture: ", err)
			}
		}
		if param.mode == cca.Video {
			if _, err := app.RecordVideo(ctx, cca.TimerOff, 10*time.Second); err != nil {
				s.Fatal("Failed to record video: ", err)
			}
		}

		if err := app.SaveScreenshot(ctx); err != nil {
			s.Error("Failed to save a screenshot: ", err)
		}

		orient, err := app.GetScreenOrientation(ctx)
		if err != nil {
			s.Fatal("Failed to get screen orientation", err)
		}
		if orient != screenOrient[index] {
			s.Fatalf("Failed to match screen orientation: got %q; want %q", orient, screenOrient[index])
		}
	}
}
