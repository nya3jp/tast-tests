// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/local/bundles/cros/camera/getusermedia"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GetUserMediaPerf2,
		Desc:         "Captures process performance data during Camera preview",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{caps.BuiltinCamera, "chrome"},
		Pre:          pre.ChromeCameraPerf(),
		Data:         []string{"camera_preview.html"},
	})
}

func GetUserMediaPerf2(ctx context.Context, s *testing.State) {
	// Run tests for 20 seconds per resolution.
	getusermedia.RunGetUserMediaPerf2(ctx, s, s.PreValue().(*chrome.Chrome))
}
