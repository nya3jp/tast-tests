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
		Func:         HAL3Frame,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies camera frame function with HAL3 interface",
		Contacts:     []string{"hywu@chromium.org", "shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"arc", "arc_camera3", "chrome", caps.BuiltinCamera},
		Pre:          chrome.LoggedIn(),
		// Default timeout (i.e. 2 minutes) is not enough for some devices to
		// exercise all resolutions on all cameras. Currently the device that
		// needs longest timeout is Krane, which supports many resolutions
		// up to 3264x2448 as well private/YUV reprocessing.
		Timeout: 15 * time.Minute,
	})
}

func HAL3Frame(ctx context.Context, s *testing.State) {
	if err := hal3.RunTest(ctx, hal3.FrameTestConfig()); err != nil {
		s.Error("Test failed: ", err)
	}
}
