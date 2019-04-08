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
		Func:         GCAStillCapture,
		Desc:         "Tests still capture with GoogleCameraArc (GCA)",
		Contacts:     []string{"lnishan@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Pre:          arc.Booted(),
		Timeout:      4 * time.Minute,
	})
}

func GCAStillCapture(ctx context.Context, s *testing.State) {
	gca.RunTest(ctx, s, func(ctx context.Context, s *testing.State, d *ui.Device) {
		// GCA always starts in photo mode.
		// Clicking on the shutter button initiates a still capture.
		gca.ClickShutterButton(ctx, s, d)
	})
}
