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
		Func:         HAL3Frame,
		Desc:         "Verifies camera frame function with HAL3 interface",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android", "arc_camera3", caps.BuiltinCamera},
		// Default timeout (i.e. 2 minutes) is not enough for some devices to
		// exercise all resolutions on all cameras. Currently the device that
		// needs longest timeout is Nocturne, which supports many resolutions
		// up to 3264x2448.
		Timeout: 10 * time.Minute,
	})
}

func HAL3Frame(ctx context.Context, s *testing.State) {
	hal3.RunTest(ctx, s, hal3.TestConfig{GtestFilter: "Camera3FrameTest/*"})
}
