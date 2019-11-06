// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCDecodeAccel,
		Desc:         "Verifies ARC++ hardware decode acceleration by running the c2_e2e_test APK",
		Contacts:     []string{"johnylin@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{"c2_e2e_test.apk", "c2_e2e_test_arm.apk"},
		SoftwareDeps: []string{"android", "chrome"},
		Pre:          arc.Booted(),
		Params: []testing.Param{{
			Name:              "h264",
			Val:               "test-25fps.h264",
			ExtraSoftwareDeps: []string{caps.HWDecodeH264},
			ExtraData:         []string{"test-25fps.h264", "test-25fps.h264.json"},
		}, {
			Name:              "vp8",
			Val:               "test-25fps.vp8",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			ExtraData:         []string{"test-25fps.vp8", "test-25fps.vp8.json"},
		}, {
			Name:              "vp9",
			Val:               "test-25fps.vp9",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         []string{"test-25fps.vp9", "test-25fps.vp9.json"},
		}},
	})
}

func ARCDecodeAccel(ctx context.Context, s *testing.State) {
	decode.RunAllARCVideoTests(ctx, s, s.Param().(string))
}
