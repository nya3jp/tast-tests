// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIMultiCameraTakePicture,
		Desc:         "Opens CCA and verifies the multi-camera photo taking related use cases related use cases",
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

// Open CCA and capture image(s) using front camera or back camera
func CCAUIMultiCameraTakePicture(ctx context.Context, s *testing.State) {
	const defaultIterations = 1
	iterations := intVar(s, "iterations", defaultIterations)
	s.Logf("No of iteration for camera validation: %d", iterations)

	cr := s.PreValue().(*chrome.Chrome)
	tb, err := testutil.NewTestBridge(ctx, cr, testutil.UseRealCamera)
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	defer tb.TearDown(ctx)

	if err := cca.ClearSavedDirs(ctx, cr); err != nil {
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
	}(ctx)

	numCameras, err := app.GetNumOfCameras(ctx)
	if err != nil {
		s.Fatal("Can't get number of cameras: ", err)
	}

	s.Log("No. of cameras: ", numCameras)

	var facingParam = s.Param().(cca.Facing)
	var bothFacing = cca.Facing("both")

	// Check whether back camera is available or not
	if (facingParam == cca.FacingBack || facingParam == bothFacing) && numCameras == 1 {
		s.Fatalf("%v camera not available", facingParam)
	}

	// Check whether correct camera is switched
	checkFacing := func() bool {
		facing, err := app.GetFacing(ctx)
		if err != nil {
			s.Fatal("Get facing failed: ", err)
		}

		if facing == facingParam {
			return true
		}
		return false
	}

	// Verify the camera facing for user facing and env facing params only
	if facingParam != bothFacing && !checkFacing() {
		s.Log("Switch camera")
		if err := app.SwitchCamera(ctx); err != nil {
			s.Fatal("Switch camera failed: ", err)
		}
		if !checkFacing() {
			s.Fatalf("Not on %v facing camera", facingParam)
		}
	}

	takePicture := func() {
		_, err := app.TakeSinglePhoto(ctx, cca.TimerOff)
		if err != nil {
			s.Fatal("Unable to capture pictures: %v", err)
		}
	}

	for i := 0; i < iterations; i++ {
		takePicture()
		if facingParam == bothFacing {
			// Switch camera and capture image using env camera
			s.Log("Switch camera")
			if err := app.SwitchCamera(ctx); err != nil {
				s.Fatal("Switch camera failed: ", err)
			}
			takePicture()
		}
	}
}
