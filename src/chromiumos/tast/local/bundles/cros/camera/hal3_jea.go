// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/bundles/cros/camera/hal3"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HAL3JEA,
		Desc:         "Verifies JPEG encode accelerator works in USB HALv3",
		Contacts:     []string{"hywu@chromium.org", "shik@chromium.org", "chromeos-camera-eng@google.com"},
		SoftwareDeps: []string{"arc", "arc_camera3", "chrome", caps.HWEncodeJPEG},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		Pre:          chrome.LoggedIn(),
		Params: []testing.Param{{
			Val:               "usb",
			ExtraSoftwareDeps: []string{caps.BuiltinUSBCamera},
			ExtraAttr:         []string{"group:camera-postsubmit"},
		}, {
			Name:              "mipi",
			Val:               "mipi",
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("gru")),
			ExtraSoftwareDeps: []string{caps.BuiltinMIPICamera},
		}},
	})
}

func HAL3JEA(ctx context.Context, s *testing.State) {
	usbOnly := s.Param().(string) == "usb"
	if usbOnly {
		if err := hal3.RunTest(ctx, hal3.JEAUSBTestConfig(s.OutDir())); err != nil {
			s.Error("Test failed: ", err)
		}
	} else {
		if err := hal3.RunTest(ctx, hal3.JEATestConfig(s.OutDir())); err != nil {
			s.Error("Test failed: ", err)
		}
	}
}
