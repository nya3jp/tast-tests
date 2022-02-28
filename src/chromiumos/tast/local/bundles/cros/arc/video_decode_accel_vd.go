// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/bundles/cros/arc/c2e2etest"
	"chromiumos/tast/local/bundles/cros/arc/video"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VideoDecodeAccelVD,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Verifies ARCVM hardware decode acceleration using a media::VideoDecoder by running the c2_e2e_test APK (see go/arcvm-vd)",
		Contacts:     []string{"chromeos-video-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{c2e2etest.X86ApkName, c2e2etest.ArmApkName},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Fixture:      "arcBootedWithVideoLoggingVD",
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name:              "h264_vm",
			Val:               video.DecodeTestOptions{TestVideo: "test-25fps.h264"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264},
			ExtraData:         []string{"test-25fps.h264", "test-25fps.h264.json"},
		}, {
			Name:              "vp8_vm",
			Val:               video.DecodeTestOptions{TestVideo: "test-25fps.vp8"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			ExtraData:         []string{"test-25fps.vp8", "test-25fps.vp8.json"},
		}, {
			Name:              "vp9_vm",
			Val:               video.DecodeTestOptions{TestVideo: "test-25fps.vp9"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         []string{"test-25fps.vp9", "test-25fps.vp9.json"},
		}},
	})
}

func VideoDecodeAccelVD(ctx context.Context, s *testing.State) {
	video.RunAllARCVideoTests(ctx, s, s.Param().(video.DecodeTestOptions))
}
