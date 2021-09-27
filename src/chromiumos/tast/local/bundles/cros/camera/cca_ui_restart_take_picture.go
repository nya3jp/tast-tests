// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/camera/common"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIRestartTakePicture,
		Desc:         "Open CCA, take picture and restart the app multiple times",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:mainline", "group:camera-libcamera", "informational"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Vars:         []string{"iterations"}, // Number of iterations to test.
		Pre:          chrome.LoggedIn(),
	})
}

// CCAUIRestartTakePicture Opens CCA, captures image and restarts the app for given iterations.
func CCAUIRestartTakePicture(ctx context.Context, s *testing.State) {
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

	for i := 0; i < iterations; i++ {
		if _, err := app.TakeSinglePhoto(ctx, cca.TimerOff); err != nil {
			s.Fatalf("Failed to capture picture in %d iteration: %v", i, err)
		}
		if err := app.Restart(ctx, tb); err != nil {
			s.Fatalf("Failed to restart the camera in %d iteration: %v", i, err)
		}
	}
}
