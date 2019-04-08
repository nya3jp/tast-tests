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
		Func:         GCARecording,
		Desc:         "Tests video recording with GoogleCameraArc (GCA)",
		Contacts:     []string{"lnishan@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login", caps.BuiltinOrVividCamera},
		Pre:          arc.Booted(),
		Timeout:      4 * time.Minute,
	})
}

func GCARecording(ctx context.Context, s *testing.State) {
	gca.RunTest(ctx, s, func(ctx context.Context, d *ui.Device) {
		// Switch to video mode.
		if err := gca.SwitchMode(ctx, d, gca.VideoMode); err != nil {
			s.Fatal("Failed to switch to video mode: ", err)
		}

		// Start recording.
		if err := gca.ClickShutterButton(ctx, d); err != nil {
			s.Fatal("Failed to start video recording: ", err)
		}

		// Record for 3 seconds.
		if err := testing.Sleep(ctx, 3*time.Second); err != nil {
			s.Fatal("Timed out on recording video: ", err)
		}

		// Get current timestamp and stop recording by clicking on the shutter button again.
		ts := time.Now()
		if err := gca.ClickShutterButton(ctx, d); err != nil {
			s.Fatal("Failed to stop video recording: ", err)
		}

		// Verify that a new video file is created.
		if err := gca.VerifyFile(ctx, s.PreValue().(arc.PreData).Chrome, gca.VideoPattern, ts); err != nil {
			s.Fatal("Failed to find matching output video file: ", err)
		}
	})
}
