// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/webrtc/peerconnection"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
)

// rtcPerfTest is used to describe the config used to run each test case.
type rtcPerfTest struct {
	enableHWAccel bool   // Instruct to use hardware or software decoding.
	profile       string // Codec to try, e.g. VP8, VP9.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         RTCPeerConnectionPerf,
		Desc:         "Measures WebRTC decode performance in terms of CPU usage and decode time with and without hardware acceleration",
		Contacts:     []string{"mcasas@chromium.org", "chromeos-gfx-video@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Data:         append(webrtc.DataFiles(), peerconnection.LoopbackFile),
		// TODO(crbug.com/1029548): Add more variations here, e.g. vp8.
		Params: []testing.Param{{
			Name:              "h264_hw",
			Val:               rtcPerfTest{enableHWAccel: true, profile: "H264"},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			// TODO(b/145961243): Use pre.ChromeVideoWithFakeWebcam().
			Pre: pre.ChromeVideoWithFakeWebcamAndH264AMDEncoder(),
		}, {
			Name:              "h264_sw",
			Val:               rtcPerfTest{enableHWAccel: false, profile: "H264"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideoWithFakeWebcamAndSWDecoding(),
		}, {
			Name:              "vp8_hw",
			Val:               rtcPerfTest{enableHWAccel: true, profile: "VP8"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name: "vp8_sw",
			Val:  rtcPerfTest{enableHWAccel: false, profile: "VP8"},
			Pre:  pre.ChromeVideoWithFakeWebcamAndSWDecoding(),
		}, {
			Name: "vp9_hw",
			Val:  rtcPerfTest{enableHWAccel: true, profile: "VP9"},
			// TODO(crbug.com/811912): Remove "vaapi" and use pre.ChromeVideoWithFakeWebcam()
			// once the feature is enabled by default on VA-API devices.
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9, "vaapi"},
			Pre:               pre.ChromeVideoWithFakeWebcamAndVP9VaapiEncoder(),
		}, {
			Name: "vp9_sw",
			Val:  rtcPerfTest{enableHWAccel: false, profile: "VP9"},
			Pre:  pre.ChromeVideoWithFakeWebcamAndSWDecoding(),
		}},
		// Default timeout (i.e. 2 minutes) is not enough.
		Timeout: 10 * time.Minute,
	})
}

// RTCPeerConnectionPerf opens a WebRTC loopback page that loops a given capture stream to measure decode time and CPU usage.
func RTCPeerConnectionPerf(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(rtcPerfTest)
	peerconnection.RunDecodePerf(ctx, s, s.PreValue().(*chrome.Chrome), testOpt.profile, testOpt.enableHWAccel)
}
