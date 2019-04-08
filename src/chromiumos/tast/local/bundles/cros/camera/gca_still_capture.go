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
		SoftwareDeps: []string{"android", "chrome_login", "caps.BuiltinOrVividCamera"},
		Pre:          arc.Booted(),
		Timeout:      4 * time.Minute,
	})
}

func GCAStillCapture(ctx context.Context, s *testing.State) {
	must := func(err error) {
		if err != nil {
			s.Fatal(err) // NOLINT: camera/gca returns loggable errors
		}
	}

	a := s.PreValue().(arc.PreData).ARC
	d, err := gca.SetUpDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed to set up device: ", err)
	}
	defer gca.TearDownDevice(ctx, a, d)

	// Get current timestamp and take a picture.
	ts := time.Now()
	must(gca.ClickShutterButton(ctx, d))

	// Verify that a new image file is created.
	must(gca.VerifyFile(ctx, s.PreValue().(arc.PreData).Chrome, gca.ImagePattern, ts))
}
