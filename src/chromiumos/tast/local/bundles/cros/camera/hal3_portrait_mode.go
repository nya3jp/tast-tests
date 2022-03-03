// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
		Func:         HAL3PortraitMode,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies camera portrait mode function with HAL3 interface",
		Contacts:     []string{"hywu@chromium.org", "chromeos-camera-eng@google.com"},
		SoftwareDeps: []string{"arc", "arc_camera3", "chrome", caps.BuiltinCamera},
		Data:         []string{portraitModeTestFile},
		Pre:          chrome.LoggedIn(),
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			Name:      "",
			Val:       false, // generatePerfLog
			ExtraAttr: []string{"group:mainline", "informational", "group:camera-libcamera"},
		}, {
			Name:      "perf",
			Val:       true, // generatePerfLog
			ExtraAttr: []string{"group:crosbolt", "crosbolt_perbuild"},
		}},
	})
}

const portraitModeTestFile = "portrait_4096x3072.jpg"

func HAL3PortraitMode(ctx context.Context, s *testing.State) {
	if err := hal3.RunTest(ctx, hal3.PortraitModeTestConfig(s.Param().(bool), s.DataPath(portraitModeTestFile))); err != nil {
		s.Error("Test failed: ", err)
	}
}
