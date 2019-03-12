// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/local/bundles/cros/camera/hal3"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HAL3JEA,
		Desc:         "Verifies JPEG encode accelerator works in USB HALv3",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "arc_camera3", caps.USBCamera, caps.HWEncodeJPEG},
	})
}

func HAL3JEA(ctx context.Context, s *testing.State) {
	hal3.RunTest(ctx, s, hal3.TestConfig{
		CameraHALs:     []string{"usb"},
		GtestFilter:    "*/Camera3SimpleStillCaptureTest.TakePictureTest/0",
		ForceJPEGHWEnc: true,
	})
}
