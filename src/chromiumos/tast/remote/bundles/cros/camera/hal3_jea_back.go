// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/remote/bundles/cros/camera/hal3client"
	pb "chromiumos/tast/services/cros/camerabox"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HAL3JEABack,
		Desc:         "Verifies JPEG encode accelerator works in USB HALv3 on back camera of remote DUT",
		Contacts:     []string{"inker@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"android_p", "arc_camera3", caps.BuiltinCamera},
		ServiceDeps:  []string{"tast.cros.camerabox.HAL3Service"},
	})
}

func HAL3JEABack(ctx context.Context, s *testing.State) {
	hal3client.RunTest(ctx, s, pb.HAL3CameraTest_JEA, pb.Facing_FACING_BACK)
}
