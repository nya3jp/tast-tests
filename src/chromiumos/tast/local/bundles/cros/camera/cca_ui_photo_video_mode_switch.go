// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"context"
	"time"
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
		Timeout:      1 * time.Hour,
		Pre:          chrome.LoggedIn(),
	})
}

// Open CCA and switch between photo viewfinder and video viewfinder.
func CCAUIPhotoVideoModeSwitch(ctx context.Context, s *testing.State) {
	// By default the camera should switch between photo and video viewfinder one time
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

	// Switch between photo viewfinder and video viewfinder for given iterations.
	for i := 1; i <= iterations; i++ {
		if err := app.SwitchMode(ctx, cca.Video); err != nil {
			s.Fatalf("Failed to switch to Video viewfinder in iteration %d : %v", i, err)
		}
		if err := app.SwitchMode(ctx, cca.Photo); err != nil {
			s.Fatalf("Failed to switch to Photo viewfinder in iteration %d : %v", i, err)
		}
	}
}
