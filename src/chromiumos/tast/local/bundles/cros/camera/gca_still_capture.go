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
		Func:         GCAStillCapture,
		Desc:         "Tests still capture with GoogleCameraArc (GCA)",
		Contacts:     []string{"lnishan@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android", "chrome", caps.BuiltinOrVividCamera},
		Data:         []string{gca.Apk},
		Pre:          arc.Booted(),
		Timeout:      4 * time.Minute,
	})
}

// GCAStillCapture tests still capture with GoogleCameraArc (GCA).
// This test would take a picture with the default resolution and verify that a matching output image file is created.
// Note that this test doesn't verify the integrity of the output file.
func GCAStillCapture(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(arc.PreData).Chrome
	gca.RunTest(ctx, s, func(ctx context.Context, a *arc.ARC, d *ui.Device) {
		// Switch to photo mode.
		if err := gca.SwitchMode(ctx, d, gca.PhotoMode); err != nil {
			s.Fatal("Failed to switch to photo mode: ", err)
		}

		s.Log("Taking the first picture")
		// Get current timestamp and take a picture.
		ts := time.Now()
		if err := gca.ClickShutterButton(ctx, d); err != nil {
			s.Fatal("Failed to take a picture: ", err)
		}
		// Verify that a new image file is created.
		if err := gca.VerifyFile(ctx, cr, gca.ImagePattern, ts); err != nil {
			s.Fatal("Failed to verify that a matching output image file is created: ", err)
		}

		s.Log("Taking the second picture with 3-second countdown")
		ts = time.Now()
		if err := gca.SetTimerOption(ctx, d, gca.ThreeSecondTimer); err != nil {
			s.Fatal("Failed to set timer option to 3 seconds: ", err)
		}
		if err := gca.ClickShutterButton(ctx, d); err != nil {
			s.Fatal("Failed to take a picture: ", err)
		}
		if err := gca.VerifyFile(ctx, cr, gca.ImagePattern, ts.Add(3*time.Second)); err != nil {
			s.Fatal("Failed to verify that a matching output image file is created: ", err)
		}
	})
}
