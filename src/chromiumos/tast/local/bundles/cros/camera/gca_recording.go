// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/camera/gca"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GCARecording,
		Desc:         "Tests video recording with GoogleCameraArc (GCA)",
		Contacts:     []string{"lnishan@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Pre:          arc.Booted(),
		Timeout:      4 * time.Minute,
	})
}

func GCARecording(ctx context.Context, s *testing.State) {
	d, err := gca.SetUpDevice(ctx, s)
	if err != nil {
		gca.TearDownDevice(ctx, s, d)
		s.Fatal("Failed to set up device: ", err)
	}
	defer gca.TearDownDevice(ctx, s, d)

	// Switch to video mode.
	gca.SwitchMode(ctx, s, d, gca.VideoMode)

	// Start recording.
	gca.ClickShutterButton(ctx, s, d)

	// Record for 3 seconds.
	testing.Sleep(ctx, 3*time.Second)

	// Get current timestamp and stop recording by clicking on the shutter button again.
	ts := time.Now()
	gca.ClickShutterButton(ctx, s, d)

	// Verify that a new video file is created.
	gca.VerifyFile(ctx, s, gca.VideoFormat, ts)
}
