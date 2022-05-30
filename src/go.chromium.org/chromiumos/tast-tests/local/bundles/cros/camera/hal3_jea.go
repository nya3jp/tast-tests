// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"go.chromium.org/chromiumos/tast-tests/common/media/caps"
	"go.chromium.org/chromiumos/tast-tests/local/bundles/cros/camera/hal3"
	"go.chromium.org/chromiumos/tast-tests/local/chrome"
	"go.chromium.org/chromiumos/tast/testing"
	"go.chromium.org/chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HAL3JEA,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies JPEG encode accelerator works in USB HALv3",
		Contacts:     []string{"hywu@chromium.org", "shik@chromium.org", "chromeos-camera-eng@google.com"},
		SoftwareDeps: []string{"arc", "arc_camera3", "chrome", caps.HWEncodeJPEG},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		Pre:          chrome.LoggedIn(),
		Timeout:      4 * time.Minute,
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
		if err := hal3.RunTest(ctx, hal3.JEAUSBTestConfig()); err != nil {
			s.Error("Test failed: ", err)
		}
	} else {
		if err := hal3.RunTest(ctx, hal3.JEATestConfig()); err != nil {
			s.Error("Test failed: ", err)
		}
	}
}
