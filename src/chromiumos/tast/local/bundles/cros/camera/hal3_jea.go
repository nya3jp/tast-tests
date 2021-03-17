// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/local/bundles/cros/camera/hal3"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HAL3JEA,
		Desc:         "Verifies JPEG encode accelerator works in USB HALv3",
		Contacts:     []string{"hywu@chromium.org", "shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-postsubmit", "group:camera-libcamera"},
		SoftwareDeps: []string{"arc", "arc_camera3", "chrome", caps.HWEncodeJPEG, caps.BuiltinUSBCamera},
		Pre:          chrome.LoggedIn(),
	})
}

func HAL3JEA(ctx context.Context, s *testing.State) {
	if err := hal3.RunTest(ctx, hal3.JEATestConfig(s.OutDir())); err != nil {
		s.Error("Test failed: ", err)
	}
}
