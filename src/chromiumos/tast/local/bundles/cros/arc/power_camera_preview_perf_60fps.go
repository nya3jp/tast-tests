// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/camera"
	"chromiumos/tast/testing"
)

func init() {
	// Note: Some cameras do not support 60 FPS and won't capture more than 30 FPS, regardless of the target FPS.
	testing.AddTest(&testing.Test{
		Func: PowerCameraPreviewPerf60Fps,
		Desc: "Measures the battery drain and camera statistics (e.g., dropped frames) during camera preview at 60 FPS",
		Contacts: []string{
			"springerm@chromium.org",
			"arcvm-eng@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:              "",
			ExtraAttr:         []string{"group:crosbolt", "crosbolt_nightly"},
			ExtraSoftwareDeps: []string{"android"},
			Pre:               arc.Booted(),
		}, {
			Name:              "vm",
			ExtraAttr:         []string{"group:crosbolt", "crosbolt_nightly"},
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBooted(),
		}},
		Timeout: 5 * time.Minute,
	})
}

func PowerCameraPreviewPerf60Fps(ctx context.Context, s *testing.State) {
	camera.PowerCameraPreviewPerf(ctx, s, "60")
}
