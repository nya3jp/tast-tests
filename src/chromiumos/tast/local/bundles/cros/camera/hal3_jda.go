// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/bundles/cros/camera/hal3"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HAL3JDA,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies JPEG decode accelerator works in USB HALv3",
		Contacts:     []string{"hywu@chromium.org", "shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-postsubmit", "group:camera-libcamera"},
		SoftwareDeps: []string{"arc", "arc_camera3", "chrome", caps.HWDecodeJPEG, caps.BuiltinUSBCamera},
		Pre:          chrome.LoggedIn(),
		Timeout:      4 * time.Minute,
	})
}

func HAL3JDA(ctx context.Context, s *testing.State) {
	if err := hal3.RunTest(ctx, hal3.JDATestConfig()); err != nil {
		s.Error("Test failed: ", err)
	}
}
