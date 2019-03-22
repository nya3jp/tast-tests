// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/local/bundles/cros/camera/hal3"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HAL3Perf,
		Desc:         "Measures camera HAL3 performance",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"android", "arc_camera3", caps.BuiltinCamera},
	})
}

func HAL3Perf(ctx context.Context, s *testing.State) {
	hal3.RunTest(ctx, s, hal3.TestConfig{
		GtestFilter:     "Camera3StillCaptureTest/Camera3SimpleStillCaptureTest.PerformanceTest/*",
		GeneratePerfLog: true,
	})
}
