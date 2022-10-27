// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/dut"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Suspend,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that the camera stack can handle a suspend/resume",
		Contacts:     []string{"ribalda@google.com", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"arc_camera3", caps.BuiltinCamera},
		Timeout:      2 * time.Minute,
	})
}

func smokeTest(ctx context.Context, dut *dut.DUT) ([]byte, error) {
	return dut.Conn().CommandContext(ctx, "cros_camera_connector_test", "--gtest_filter=ConnectorTest/CaptureTest.OneFrame/NV12_640x480_30fps").CombinedOutput()
}

func Suspend(ctx context.Context, s *testing.State) {

	d := s.DUT()

	if output, err := smokeTest(ctx, d); err != nil {
		s.Log(string(output))
		s.Fatal("Failed to run a smoke test on the camera: ", err)
	}

	if output, err := d.Conn().CommandContext(ctx, "suspend_stress_test", "-c", "1").CombinedOutput(); err != nil {
		s.Log(string(output))
		s.Fatal("Failed to perform suspend stress test: ", err)
	}

	if output, err := smokeTest(ctx, d); err != nil {
		s.Log(string(output))
		s.Fatal("Camera failed after suspend: ", err)
	}

}
