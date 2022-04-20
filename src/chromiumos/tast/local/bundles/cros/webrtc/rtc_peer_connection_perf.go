// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/bundles/cros/webrtc/peerconnection"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RTCPeerConnectionPerf,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Measures WebRTC decode performance in terms of CPU usage and decode time with and without hardware acceleration",
		Contacts: []string{
			"hiroh@chromium.org",
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Data:         append(peerconnection.DataFiles(), peerconnection.LoopbackFile),
		// TODO(crbug.com/1029548): Add more variations here, e.g. vp8.
		Params: []testing.Param{{
			Name:    "av1_sw",
			Val:     peerconnection.MakeSWTestOptions("AV1"),
			Fixture: "chromeVideoWithFakeWebcamAndNoHwAcceleration",
		}, {
			Name:              "h264_hw",
			Val:               peerconnection.MakeTestOptions("H264"),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, caps.HWEncodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "h264_sw",
			Val:               peerconnection.MakeSWTestOptions("H264"),
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			Fixture:           "chromeVideoWithFakeWebcamAndNoHwAcceleration",
		}, {
			Name:              "vp8_hw",
			Val:               peerconnection.MakeTestOptions("VP8"),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, caps.HWEncodeVP8},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:    "vp8_sw",
			Val:     peerconnection.MakeSWTestOptions("VP8"),
			Fixture: "chromeVideoWithFakeWebcamAndNoHwAcceleration",
		}, {
			Name:              "vp9_hw",
			Val:               peerconnection.MakeTestOptions("VP9"),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:    "vp9_sw",
			Val:     peerconnection.MakeSWTestOptions("VP9"),
			Fixture: "chromeVideoWithFakeWebcamAndNoHwAcceleration",
		}, {
			// This is a 2 temporal layers test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name:              "vp9_hw_svc_l1t2",
			Val:               peerconnection.MakeTestOptionsWithSVC("VP9", "L1T2"),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabled",
		}, {
			// This is a 3 temporal layers test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name:              "vp9_hw_svc_l1t3",
			Val:               peerconnection.MakeTestOptionsWithSVC("VP9", "L1T3"),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabled",
		}, {
			// This is a 3 spatial layers, 3 temporal layers (each) k-SVC test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name:              "vp9_hw_svc_l3t3_key",
			Val:               peerconnection.MakeTestOptionsWithSVC("VP9", "L3T3_KEY"),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWEncodeVP9},
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsVP9KSVCHWEncoding()),
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabled",
		}, {
			Name:              "vp8_hw_multi_vp9_3x3",
			Val:               peerconnection.MakeTestOptionsWithVideoGrid("VP8", 3, "tulip2-320x180.vp9.webm"),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWDecodeVP8, caps.HWEncodeVP8},
			ExtraData:         []string{"tulip2-320x180.vp9.webm"},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "vp8_hw_multi_vp9_4x4",
			Val:               peerconnection.MakeTestOptionsWithVideoGrid("VP8", 4, "tulip2-320x180.vp9.webm"),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWDecodeVP8, caps.HWEncodeVP8},
			ExtraData:         []string{"tulip2-320x180.vp9.webm"},
			Fixture:           "chromeVideoWithFakeWebcam",
			// Trogdor doesn't have enough hardware contexts to pass this test.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("trogdor")),
		}, {
			Name:              "vp9_hw_multi_vp9_3x3",
			Val:               peerconnection.MakeTestOptionsWithVideoGrid("VP9", 3, "tulip2-320x180.vp9.webm"),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWEncodeVP9},
			ExtraData:         []string{"tulip2-320x180.vp9.webm"},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "vp8_hw_multi_vp9_3x3_global_vaapi_lock_disabled",
			Val:               peerconnection.MakeTestOptionsWithVideoGrid("VP8", 3, "tulip2-320x180.vp9.webm"),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWDecodeVP8, caps.HWEncodeVP8, "thread_safe_libva_backend"},
			ExtraData:         []string{"tulip2-320x180.vp9.webm"},
			Fixture:           "chromeVideoWithFakeWebcamAndGlobalVaapiLockDisabled",
		}, {
			Name:              "vp8_hw_multi_vp9_4x4_global_vaapi_lock_disabled",
			Val:               peerconnection.MakeTestOptionsWithVideoGrid("VP8", 4, "tulip2-320x180.vp9.webm"),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWDecodeVP8, caps.HWEncodeVP8, "thread_safe_libva_backend"},
			ExtraData:         []string{"tulip2-320x180.vp9.webm"},
			Fixture:           "chromeVideoWithFakeWebcamAndGlobalVaapiLockDisabled",
		}, {
			Name:              "vp9_hw_multi_vp9_3x3_global_vaapi_lock_disabled",
			Val:               peerconnection.MakeTestOptionsWithVideoGrid("VP9", 3, "tulip2-320x180.vp9.webm"),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWEncodeVP9, "thread_safe_libva_backend"},
			ExtraData:         []string{"tulip2-320x180.vp9.webm"},
			Fixture:           "chromeVideoWithFakeWebcamAndGlobalVaapiLockDisabled",
		}},
		// Default timeout (i.e. 2 minutes) is not enough.
		Timeout: 10 * time.Minute,
	})
}

// RTCPeerConnectionPerf opens a WebRTC loopback page that loops a given capture stream to measure decode time and CPU usage.
func RTCPeerConnectionPerf(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(peerconnection.RTCTestOptions)
	if err := peerconnection.RunDecodePerf(ctx, s.FixtValue().(*chrome.Chrome), s.DataFileSystem(), s.OutDir(), testOpt); err != nil {
		s.Error("Failed to measure performance: ", err)
	}
}
