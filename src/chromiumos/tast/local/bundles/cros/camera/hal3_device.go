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
		Func:         HAL3Device,
		Desc:         "Verifies camera device function with HAL3 interface",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "arc_camera3", caps.BuiltinCamera},
	})
}

func HAL3Device(ctx context.Context, s *testing.State) {
	hal3.RunTest(ctx, s, hal3.TestConfig{GtestFilter: "Camera3DeviceTest/*"})
}
