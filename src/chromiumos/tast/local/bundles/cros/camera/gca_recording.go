// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/camera/gca"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GCARecording,
		Desc:         "Tests still capture with GoogleCameraArc (GCA)",
		Contacts:     []string{"lnishan@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Pre:          arc.Booted(),
		Timeout:      4 * time.Minute,
	})
}

func GCARecording(ctx context.Context, s *testing.State) {
	gca.RunTest(ctx, s, func(ctx context.Context, s *testing.State, d *ui.Device) {
		// Switch to video mode
		gca.SwitchMode(ctx, s, d, gca.VideoMode)

		// Start recording
		gca.ClickShutterButton(ctx, s, d)

		// Record for 3 seconds
		time.Sleep(3 * time.Second) // // NOLINT: We actually need to wait for 3 seconds for recording.

		// Stop recording by clicking on the shutter button again
		gca.ClickShutterButton(ctx, s, d)
	})
}
