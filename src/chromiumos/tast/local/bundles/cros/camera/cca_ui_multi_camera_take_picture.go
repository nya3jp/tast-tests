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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIMultiCameraTakePicture,
		Desc:         "Opens CCA and take picture using multi-cameras",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:mainline", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Vars:         []string{"iterations"}, // Number of iterations to test.
		Pre:          chrome.LoggedIn(),
		Params: []testing.Param{{
			Name:      "user_facing",
			ExtraAttr: []string{"informational"},
			Val:       cca.FacingFront,
		}, {
			Name:      "env_facing",
			ExtraAttr: []string{"informational"},
			Val:       cca.FacingBack,
		}, {
			Name:      "both_facing",
			ExtraAttr: []string{"informational"},
			Val:       cca.Facing("both"),
		}},
	})
}

// Open CCA and capture image(s) using front camera or back camera or both cameras.
func CCAUIMultiCameraTakePicture(ctx context.Context, s *testing.State) {
	const defaultIterations = 1
	iterations := intVar(s, "iterations", defaultIterations)
	s.Logf("No. of iteration for camera validation: %d", iterations)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()

	cr := s.PreValue().(*chrome.Chrome)
	tb, err := testutil.NewTestBridge(ctx, cr, testutil.UseRealCamera)
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	defer tb.TearDown(cleanupCtx)

	if err := cca.ClearSavedDir(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir(), tb)
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer func(ctx context.Context) {
		if err := app.Close(ctx); err != nil {
			s.Error("Failed to close app: ", err)
		}
	}(cleanupCtx)

	numCameras, err := app.GetNumOfCameras(ctx)
	if err != nil {
		s.Fatal("Failed to get number of cameras: ", err)
	}

	s.Log("No. of cameras: ", numCameras)

	var facingParam = s.Param().(cca.Facing)
	var bothFacing = cca.Facing("both")

	// Check whether back camera is available or not.
	if (facingParam == cca.FacingBack || facingParam == bothFacing) && numCameras == 1 {
		s.Fatalf("%v camera doesn't exist", facingParam)
	}

	// Check whether correct camera is switched.
	checkFacing := func() bool {
		facing, err := app.GetFacing(ctx)
		if err != nil {
			s.Fatal("Failed to get facing: ", err)
		}

		return facing == facingParam
	}

	// Verify the camera facing for user facing and env facing params only.
	if facingParam != bothFacing && !checkFacing() {
		if err := app.SwitchCamera(ctx); err != nil {
			s.Fatal("Failed to switch camera: ", err)
		}
		if !checkFacing() {
			s.Fatalf("Failed to get default camera facing as %v", facingParam)
		}
	}

	takePicture := func() {
		if _, err := app.TakeSinglePhoto(ctx, cca.TimerOff); err != nil {
			s.Fatal("Failed to capture picture: ", err)
		}
	}

	for i := 0; i < iterations; i++ {
		takePicture()
		if facingParam == bothFacing {
			// Switch camera and capture image using other camera.
			s.Log("Switch camera")
			if err := app.SwitchCamera(ctx); err != nil {
				s.Fatal("Failed to switch camera: ", err)
			}
			takePicture()
		}
	}
}
