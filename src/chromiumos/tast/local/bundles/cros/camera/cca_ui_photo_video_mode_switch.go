// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"context"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIPhotoVideoModeSwitch,
		Desc:         "Opens CCA and switches between photo and video viewfinder",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Vars:         []string{"iterations"}, // Number of iterations to test.
		Pre:          chrome.LoggedIn(),
	})
}

// Open CCA and switch between photo viewfinder and video viewfinder
func CCAUIPhotoVideoModeSwitch(ctx context.Context, s *testing.State) {
	const defaultIterations = 1
	iterations := intVar(s, "iterations", defaultIterations)
	s.Logf("No. of iteration for camera validation: %d", iterations)

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

	// Switch between photo viewfinder and video viewfinder for given iterations
	for i := 1; i <= iterations; i++ {
		s.Log("Switching to video mode")
		if err := app.SwitchMode(ctx, cca.Video); err != nil {
			s.Fatalf("Failed to switch to Video viewfinder in iteration %d : %v", i, err)
		}
		s.Log("Switching to photo mode")
		if err := app.SwitchMode(ctx, cca.Photo); err != nil {
			s.Fatalf("Failed to switch to Photo viewfinder in iteration %d : %v", i, err)
		}
	}
}
