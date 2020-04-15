// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/c2e2etest"
	"chromiumos/tast/local/bundles/cros/arc/video"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VideoDecodeAccel,
		Desc:         "Verifies ARC hardware decode acceleration by running the c2_e2e_test APK",
		Contacts:     []string{"akahuang@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{c2e2etest.X86ApkName, c2e2etest.ArmApkName},
		SoftwareDeps: []string{"chrome"},
		// TODO(yusukes): Change the timeout back to 4 min when we revert arc.go's BootTimeout to 120s.
		Timeout: 5 * time.Minute,
		Params: []testing.Param{{
			Name:              "h264_hw",
			Val:               "test-25fps.h264",
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "android_p"},
			ExtraData:         []string{"test-25fps.h264", "test-25fps.h264.json"},
			Pre:               arc.BootedWithVideoLogging(),
		}, {
			Name:              "h264_hw_vm",
			Val:               "test-25fps.h264",
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "android_vm"},
			ExtraData:         []string{"test-25fps.h264", "test-25fps.h264.json"},
			Pre:               arc.VMBootedWithVideoLogging(),
		}, {
			Name:              "vp8_hw",
			Val:               "test-25fps.vp8",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, "android_p"},
			ExtraData:         []string{"test-25fps.vp8", "test-25fps.vp8.json"},
			Pre:               arc.BootedWithVideoLogging(),
		}, {
			Name:              "vp8_hw_vm",
			Val:               "test-25fps.vp8",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, "android_vm"},
			ExtraData:         []string{"test-25fps.vp8", "test-25fps.vp8.json"},
			Pre:               arc.VMBootedWithVideoLogging(),
		}, {
			Name:              "vp9_hw",
			Val:               "test-25fps.vp9",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "android_p"},
			ExtraData:         []string{"test-25fps.vp9", "test-25fps.vp9.json"},
			Pre:               arc.BootedWithVideoLogging(),
		}, {
			Name:              "vp9_hw_vm",
			Val:               "test-25fps.vp9",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "android_vm"},
			ExtraData:         []string{"test-25fps.vp9", "test-25fps.vp9.json"},
			Pre:               arc.VMBootedWithVideoLogging(),
		}},
	})
}

func VideoDecodeAccel(ctx context.Context, s *testing.State) {
	video.RunAllARCVideoTests(ctx, s, s.Param().(string))
}
