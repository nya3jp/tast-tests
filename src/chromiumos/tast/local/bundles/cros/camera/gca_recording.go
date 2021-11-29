// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/camera/gca"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GCARecording,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests video recording with GoogleCameraArc (GCA)",
		Contacts:     []string{"mojahsu@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{gca.Apk},
		Pre:          arc.Booted(),
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

// GCARecording tests video recording with GoogleCameraArc (GCA).
// This test would record a video with the default resolution and verify that a matching output image file is created.
// Note that this test doesn't verify the integrity of the output file.
func GCARecording(ctx context.Context, s *testing.State) {
	gca.RunTest(ctx, s, func(ctx context.Context, a *arc.ARC, d *ui.Device) {
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
