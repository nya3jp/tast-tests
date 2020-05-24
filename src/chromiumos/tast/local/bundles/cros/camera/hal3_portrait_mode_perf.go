// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
		Func:         HAL3PortraitModePerf,
		Desc:         "Measures camera portrait mode performance",
		Contacts:     []string{"hywu@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"android_p", "arc_camera3", caps.BuiltinCamera},
		Data:         []string{portraitModePerfTestFile},
	})
}

const portraitModePerfTestFile = "portrait_4096x3072.jpg"

func HAL3PortraitModePerf(ctx context.Context, s *testing.State) {
	if err := hal3.RunTest(ctx, hal3.PortraitModePerfTestConfig(s.OutDir(), s.DataPath(portraitModePerfTestFile))); err != nil {
		s.Error("Test failed: ", err)
	}
}
