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
	d, err := gca.SetUpDevice(ctx, s)
	if err != nil {
		gca.TearDownDevice(ctx, s, d)
		s.Fatal("Failed to set up device: ", err)
	}
	defer gca.TearDownDevice(ctx, s, d)

	// Get current timestamp and take a picture.
	ts := time.Now()
	gca.ClickShutterButton(ctx, s, d)

	// Verify that a new image file is created.
	gca.VerifyFile(ctx, s, gca.ImageFormat, ts)
}
