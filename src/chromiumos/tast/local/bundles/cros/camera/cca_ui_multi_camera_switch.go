// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/camera/common"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIMultiCameraSwitch,
		Desc:         "Opens CCA and switches between front and back cameras for given iterations",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:mainline", "group:camera-libcamera", "informational"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Vars:         []string{"iterations"}, // Number of iterations to test.
		Pre:          chrome.LoggedIn(),
	})
}

// CCAUIMultiCameraSwitch Opens CCA and switches between front and back cameras for given iterations.
func CCAUIMultiCameraSwitch(ctx context.Context, s *testing.State) {
	const defaultIterations = 1
	iterations := common.IntVar(s, "iterations", defaultIterations)
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

	if numCameras > 1 {
		prevFacing, err := app.GetFacing(ctx)
		if err != nil {
			s.Fatal("Failed to get facing: ", err)
		}
		for i := 0; i < iterations; i++ {
			if err := app.SwitchCamera(ctx); err != nil {
				s.Fatalf("Failed to switch camera in iteration %d: %v", i, err)
			}
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				facing, err := app.GetFacing(ctx)
				if err != nil {
					return testing.PollBreak(errors.Wrap(err, "failed to get camera facing"))
				}
				if prevFacing == facing {
					return errors.Errorf("failed to switch camera in iteration %d: camera is in %q facing before and after switching", i, facing)
				}
				prevFacing = facing
				return nil
			}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
				s.Fatal("Failed to wait for camera switching: ", err)
			}
		}
	} else {
		s.Error("DUT doesn't support dual cameras")
	}
}
