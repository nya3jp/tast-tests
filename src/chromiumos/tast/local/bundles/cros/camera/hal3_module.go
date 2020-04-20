// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/local/bundles/cros/camera/hal3"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     HAL3Module,
		Desc:     "Verifies camera module function with HAL3 interface",
		Contacts: []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		// TODO(shik): Once cros_camera_test supports an external camera,
		// replace caps.BuiltinCamera with caps.BuiltinOrVividCamera.
		// Same for other HAL3* tests.
		SoftwareDeps: []string{"arc", "arc_camera3", caps.BuiltinCamera},
	})
}

func HAL3Module(ctx context.Context, s *testing.State) {
	if err := hal3.RunTest(ctx, hal3.ModuleTestConfig(s.OutDir())); err != nil {
		s.Error("Test failed: ", err)
	}
}
