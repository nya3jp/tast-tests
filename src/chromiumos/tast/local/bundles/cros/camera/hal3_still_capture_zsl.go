// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
		Func:         HAL3StillCaptureZSL,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies camera still capture with ZSL function with HAL3 interface",
		Contacts:     []string{"hywu@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"arc", "arc_camera3", "chrome", caps.BuiltinCamera},
		Pre:          chrome.LoggedIn(),
		Timeout:      3 * time.Minute,
	})
}

func HAL3StillCaptureZSL(ctx context.Context, s *testing.State) {
	if err := hal3.RunTest(ctx, hal3.StillCaptureZSLTestConfig()); err != nil {
		s.Error("Test failed: ", err)
	}
}
