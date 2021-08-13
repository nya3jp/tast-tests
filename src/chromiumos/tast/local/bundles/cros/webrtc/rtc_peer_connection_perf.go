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
		Desc:         "Measures WebRTC decode performance in terms of CPU usage and decode time with and without hardware acceleration",
		Contacts:     []string{"mcasas@chromium.org", "chromeos-gfx-video@google.com"},
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Data:         append(peerconnection.DataFiles(), peerconnection.LoopbackFile),
		// TODO(crbug.com/1029548): Add more variations here, e.g. vp8.
		Params: []testing.Param{{
			Name:              "h264_hw",
			Val:               peerconnection.MakeTestOptions("H264"),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, caps.HWEncodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "h264_sw",
			Val:               peerconnection.MakeSWTestOptions("H264"),
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			Fixture:           "chromeVideoWithFakeWebcamAndSWDecoding",
		}, {
			Name:              "vp8_hw",
			Val:               peerconnection.MakeTestOptions("VP8"),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, caps.HWEncodeVP8},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:    "vp8_sw",
			Val:     peerconnection.MakeSWTestOptions("VP8"),
			Fixture: "chromeVideoWithFakeWebcamAndSWDecoding",
		}, {
			Name:              "vp9_hw",
			Val:               peerconnection.MakeTestOptions("VP9"),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:    "vp9_sw",
			Val:     peerconnection.MakeSWTestOptions("VP9"),
			Fixture: "chromeVideoWithFakeWebcamAndSWDecoding",
		}, {
			// This is a 3 temporal layers test.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name:              "vp9_hw_force_l1t3",
			Val:               peerconnection.MakeTestOptions("VP9"),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcamAndForceVP9ThreeTemporalLayers",
		}, {
			// This is 3 spatial layers, 3 temporal layers (each) k-SVC.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name:              "vp9_hw_force_l3t3_key",
			Val:               peerconnection.MakeTestOptions("VP9"),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWEncodeVP9},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform("volteer", "dedede")),
			Fixture:           "chromeVideoWithFakeWebcamAndForceVP9SVC3SL3TLKey",
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
