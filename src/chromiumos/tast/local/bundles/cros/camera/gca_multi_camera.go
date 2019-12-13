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
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GCAMultiCamera,
		Desc:         "Tests multi-camera (camera switching) function of GoogleCameraArc (GCA)",
		Contacts:     []string{"lnishan@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android", "chrome", caps.BuiltinOrVividCamera},
		Data:         []string{gca.Apk},
		Pre:          arc.Booted(),
		Timeout:      4 * time.Minute,
	})
}

// GCAMultiCamera tests multi-camera (camera switching) function of GoogleCameraArc (GCA).
// This test switches GCA to the next camera multiple times and verifies that the default camera facing stays the same after restarting the app.
// Note that this test is intended for devices with more than one camera. On single-camera devices, the test would always pass since there's no other camera to switch to.
// Ideally we'd like to skip the test, but it's technically infeasible to add an autotest-capability for it. We cannot get the number of cameras statically due to several complications with camera/device configurations.
func GCAMultiCamera(ctx context.Context, s *testing.State) {
	gca.RunTest(ctx, s, func(ctx context.Context, a *arc.ARC, d *ui.Device) {
		// Get the default camera for verification later.
		// TODO(lnishan): Get the input package to expose and generalize the functions for determining the device mode.
		defaultFacing, err := gca.GetFacing(ctx, d)
		if err != nil {
			s.Fatal("Cannot get the default facing of camera: ", err)
		}
		s.Log("Default camera facing is: ", defaultFacing)

		// numIterations is the number of times we want to switch the camera.
		// We chose 7 because this is a reasonably adequate number to verify camera switching works correctly.
		// 7 as a prime number also makes sense because it's not a multiple of the number of cameras on a device and thus the final camera we're on won't be the one we opened on startup.
		const numIterations = 7
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

		s.Log("Verifying the default camera facing remains the same")
		if facing, err := gca.GetFacing(ctx, d); err != nil {
			s.Fatal("Cannot get the direction of camera after restarting the app: ", err)
		} else if facing != defaultFacing {
			s.Fatalf("GCA didn't open a camera with the default facing (default: %s, GCA opened: %s)", defaultFacing, facing)
		}
	})
}
