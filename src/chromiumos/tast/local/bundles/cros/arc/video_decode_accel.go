// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
		Func:         VideoDecodeAccel,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Verifies ARC++ hardware decode acceleration by running the c2_e2e_test APK",
		Contacts: []string{
			"akahuang@chromium.org",
			"andrescj@chromium.org", // For the 'oopvd' variants.
			"chromeos-video-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{c2e2etest.X86ApkName, c2e2etest.ArmApkName},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name:              "h264",
			Val:               video.DecodeTestOptions{TestVideo: "test-25fps.h264"},
			Fixture:           "arcBootedWithVideoLogging",
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "android_p"},
			ExtraData:         []string{"test-25fps.h264", "test-25fps.h264.json"},
		}, {
			Name:              "h264_oopvd",
			Val:               video.DecodeTestOptions{TestVideo: "test-25fps.h264"},
			Fixture:           "arcBootedWithVideoLoggingAndOutOfProcessVideoDecoding",
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "android_p"},
			ExtraData:         []string{"test-25fps.h264", "test-25fps.h264.json"},
		}, {
			Name:              "h264_vm",
			Val:               video.DecodeTestOptions{TestVideo: "test-25fps.h264"},
			Fixture:           "arcBootedWithVideoLogging",
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "android_vm"},
			ExtraData:         []string{"test-25fps.h264", "test-25fps.h264.json"},
		}, {
			Name:              "h264_oopvd_vm",
			Val:               video.DecodeTestOptions{TestVideo: "test-25fps.h264"},
			Fixture:           "arcBootedWithVideoLoggingAndOutOfProcessVideoDecoding",
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "android_vm"},
			ExtraData:         []string{"test-25fps.h264", "test-25fps.h264.json"},
		}, {
			Name:              "vp8",
			Val:               video.DecodeTestOptions{TestVideo: "test-25fps.vp8"},
			Fixture:           "arcBootedWithVideoLogging",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, "android_p"},
			ExtraData:         []string{"test-25fps.vp8", "test-25fps.vp8.json"},
		}, {
			Name:              "vp8_oopvd",
			Val:               video.DecodeTestOptions{TestVideo: "test-25fps.vp8"},
			Fixture:           "arcBootedWithVideoLoggingAndOutOfProcessVideoDecoding",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, "android_p"},
			ExtraData:         []string{"test-25fps.vp8", "test-25fps.vp8.json"},
		}, {
			Name:              "vp8_vm",
			Val:               video.DecodeTestOptions{TestVideo: "test-25fps.vp8"},
			Fixture:           "arcBootedWithVideoLogging",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, "android_vm"},
			ExtraData:         []string{"test-25fps.vp8", "test-25fps.vp8.json"},
		}, {
			Name:              "vp8_oopvd_vm",
			Val:               video.DecodeTestOptions{TestVideo: "test-25fps.vp8"},
			Fixture:           "arcBootedWithVideoLoggingAndOutOfProcessVideoDecoding",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, "android_vm"},
			ExtraData:         []string{"test-25fps.vp8", "test-25fps.vp8.json"},
		}, {
			Name:              "vp9",
			Val:               video.DecodeTestOptions{TestVideo: "test-25fps.vp9"},
			Fixture:           "arcBootedWithVideoLogging",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "android_p"},
			ExtraData:         []string{"test-25fps.vp9", "test-25fps.vp9.json"},
		}, {
			Name:              "vp9_oopvd",
			Val:               video.DecodeTestOptions{TestVideo: "test-25fps.vp9"},
			Fixture:           "arcBootedWithVideoLoggingAndOutOfProcessVideoDecoding",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "android_p"},
			ExtraData:         []string{"test-25fps.vp9", "test-25fps.vp9.json"},
		}, {
			Name:              "vp9_vm",
			Val:               video.DecodeTestOptions{TestVideo: "test-25fps.vp9"},
			Fixture:           "arcBootedWithVideoLogging",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "android_vm"},
			ExtraData:         []string{"test-25fps.vp9", "test-25fps.vp9.json"},
		}, {
			Name:              "vp9_oopvd_vm",
			Val:               video.DecodeTestOptions{TestVideo: "test-25fps.vp9"},
			Fixture:           "arcBootedWithVideoLoggingAndOutOfProcessVideoDecoding",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "android_vm"},
			ExtraData:         []string{"test-25fps.vp9", "test-25fps.vp9.json"},
		}},
	})
}

func VideoDecodeAccel(ctx context.Context, s *testing.State) {
	video.RunAllARCVideoTests(ctx, s, s.Param().(video.DecodeTestOptions))
}
