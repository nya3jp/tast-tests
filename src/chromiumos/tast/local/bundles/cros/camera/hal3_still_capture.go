// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/camera/hal3"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HAL3StillCapture,
		Desc:         "Verifies camera still capture function with HAL3 interface",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"arc", "arc_camera3", caps.BuiltinCamera},
		// Krane needs 4 minutes and 30 seconds for whole dark environment(covering the camera lens).
		// We also need rooms for preparation time.
		Timeout: 6 * time.Minute,
	})
}

func HAL3StillCapture(ctx context.Context, s *testing.State) {
	if err := hal3.RunTest(ctx, hal3.StillCaptureTestConfig(s.OutDir())); err != nil {
		s.Error("Test failed: ", err)
	}
}
