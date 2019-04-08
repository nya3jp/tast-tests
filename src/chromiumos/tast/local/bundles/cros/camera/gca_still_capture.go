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
		Func:         GCAStillCapture,
		Desc:         "Tests still capture with GoogleCameraArc (GCA)",
		Contacts:     []string{"lnishan@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login", caps.BuiltinOrVividCamera},
		Pre:          arc.Booted(),
		Timeout:      4 * time.Minute,
	})
}

func GCAStillCapture(ctx context.Context, s *testing.State) {
	gca.RunTest(ctx, s, func(ctx context.Context, d *ui.Device) {
		// Switch to photo mode.
		if err := gca.SwitchMode(ctx, d, gca.PhotoMode); err != nil {
			s.Fatal("Failed to switch to photo mode: ", err)
		}

		// Get current timestamp and take a picture.
		ts := time.Now()
		if err := gca.ClickShutterButton(ctx, d); err != nil {
			s.Fatal("Failed to take a picture: ", err)
		}

		// Verify that a new image file is created.
		if err := gca.VerifyFile(ctx, s.PreValue().(arc.PreData).Chrome, gca.ImagePattern, ts); err != nil {
			s.Fatal("Failed to verify that a matching output image file is created: ", err)
		}
	})
}
