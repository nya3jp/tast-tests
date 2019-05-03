// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/camera/gca"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GCAMultiCamera,
		Desc:         "Tests multi-camera (camera switching) function of GoogleCameraArc (GCA)",
		Contacts:     []string{"lnishan@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login", caps.BuiltinOrVividCamera},
		Pre:          arc.Booted(),
		Timeout:      4 * time.Minute,
	})
}

// GCAMultiCamera tests multi-camera (camera switching) function of GoogleCameraArc (GCA).
// This test would switch the aoo the next camera multiple times and verify that the default camera facing stays the same after restarting the app.
func GCAMultiCamera(ctx context.Context, s *testing.State) {
	gca.RunTest(ctx, s, func(ctx context.Context, a *arc.ARC, d *ui.Device) {
		// numIterations is the number of times we want to switch the camera.
		const numIterations = 6

		// Get the default camera for verification later.
		// TODO(lnishan): Get the input package to expose and generalize the functions for determining the device mode.
		defaultFacing, err := gca.GetCameraFacing(ctx, d)
		if err != nil {
			s.Fatal("Cannot get the default camera facing: ", err)
		}
		s.Log("Default camera facing is: ", defaultFacing)

		s.Logf("Testing camera switching (%d iterations)", numIterations)
		// Switch to next camera for numIterations times.
		for i := 0; i < numIterations; i++ {
			if err := gca.SwitchCamera(ctx, d); err != nil {
				s.Fatal("Failed to switch camera: ", err)
			}
		}

		s.Log("Restarting app")
		if err := gca.RestartApp(ctx, a, d); err != nil {
			s.Fatal("Failed to restart GCA: ", err)
		}

		s.Log("Verifying the default camera remains the same")
		facing, err := gca.GetCameraFacing(ctx, d)
		if facing != defaultFacing {
			s.Fatalf("GCA didn't open a camera with the default camera facing (defaultFacing: %s, GCA opened: %s)", defaultFacing, facing)
		}
	})
}
