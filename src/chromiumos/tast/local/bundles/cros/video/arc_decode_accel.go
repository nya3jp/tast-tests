// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/video/c2e2etest"
	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCDecodeAccel,
		Desc:         "Verifies ARC++ hardware decode acceleration by running the c2_e2e_test APK",
		Contacts:     []string{"akahuang@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{c2e2etest.X86ApkName, c2e2etest.ArmApkName},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:              "h264",
			Val:               "test-25fps.h264",
			ExtraSoftwareDeps: []string{"android_p", caps.HWDecodeH264},
			ExtraData:         []string{"test-25fps.h264", "test-25fps.h264.json"},
			Pre:               arc.BootedWithVideoLogging(),
		}, {
			Name:              "h264_vm",
			Val:               "test-25fps.h264",
			ExtraSoftwareDeps: []string{"android_vm", caps.HWDecodeH264},
			ExtraData:         []string{"test-25fps.h264", "test-25fps.h264.json"},
			Pre:               arc.VMBootedWithVideoLogging(),
		}, {
			Name:              "vp8",
			Val:               "test-25fps.vp8",
			ExtraSoftwareDeps: []string{"android_p", caps.HWDecodeVP8},
			ExtraData:         []string{"test-25fps.vp8", "test-25fps.vp8.json"},
			Pre:               arc.BootedWithVideoLogging(),
		}, {
			Name:              "vp8_vm",
			Val:               "test-25fps.vp8",
			ExtraSoftwareDeps: []string{"android_vm", caps.HWDecodeVP8},
			ExtraData:         []string{"test-25fps.vp8", "test-25fps.vp8.json"},
			Pre:               arc.VMBootedWithVideoLogging(),
		}, {
			Name:              "vp9",
			Val:               "test-25fps.vp9",
			ExtraSoftwareDeps: []string{"android_p", caps.HWDecodeVP9},
			ExtraData:         []string{"test-25fps.vp9", "test-25fps.vp9.json"},
			Pre:               arc.BootedWithVideoLogging(),
		}, {
			Name:              "vp9_vm",
			Val:               "test-25fps.vp9",
			ExtraSoftwareDeps: []string{"android_vm", caps.HWDecodeVP9},
			ExtraData:         []string{"test-25fps.vp9", "test-25fps.vp9.json"},
			Pre:               arc.VMBootedWithVideoLogging(),
		}},
	})
}

func ARCDecodeAccel(ctx context.Context, s *testing.State) {
	decode.RunAllARCVideoTests(ctx, s, s.Param().(string))
}
