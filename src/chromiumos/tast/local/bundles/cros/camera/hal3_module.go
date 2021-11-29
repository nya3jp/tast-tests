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
		Func:         HAL3Module,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies camera module function with HAL3 interface",
		Contacts:     []string{"hywu@chromium.org", "shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera", "group:camera-postsubmit"},
		// TODO(shik): Once cros_camera_test supports an external camera,
		// replace caps.BuiltinCamera with caps.BuiltinOrVividCamera.
		// Same for other HAL3* tests.
		SoftwareDeps: []string{"arc", "arc_camera3", "chrome", caps.BuiltinCamera},
		Pre:          chrome.LoggedIn(),
		Timeout:      4 * time.Minute,
	})
}

func HAL3Module(ctx context.Context, s *testing.State) {
	if err := hal3.RunTest(ctx, hal3.ModuleTestConfig()); err != nil {
		s.Error("Test failed: ", err)
	}
}
