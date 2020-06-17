// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/webrtc/mediarecorder"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/testing"
)

// mediaRecorderPerfTest is used to describe the config used to run each test case.
type mediaRecorderPerfTest struct {
	enableHWAccel bool   // Instruct to use hardware or software encoding.
	profile       string // Codec to try, e.g. VP8, VP9.
}

func init() {
	testing.AddTest(&testing.Test{
		Func: MediaRecorderPerf,
		Desc: "Captures performance data about MediaRecorder for both SW and HW",
		Contacts: []string{
			"mcasas@chromium.org", // Test author.
			"chromeos-gfx-video@google.com",
			"chromeos-video-eng@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"loopback_media_recorder.html"},
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name:              "h264_sw",
			Val:               mediaRecorderPerfTest{enableHWAccel: false, profile: "H264"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideoWithFakeWebcamAndSWEncoding(),
		}, {
			Name: "vp8_sw",
			Val:  mediaRecorderPerfTest{enableHWAccel: false, profile: "VP8"},
			Pre:  pre.ChromeVideoWithFakeWebcamAndSWEncoding(),
		}, {
			Name: "vp9_sw",
			Val:  mediaRecorderPerfTest{enableHWAccel: false, profile: "VP9"},
			Pre:  pre.ChromeVideoWithFakeWebcamAndSWEncoding(),
		}, {
			Name:              "h264_hw",
			Val:               mediaRecorderPerfTest{enableHWAccel: true, profile: "H264"},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name:              "vp8_hw",
			Val:               mediaRecorderPerfTest{enableHWAccel: true, profile: "VP8"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name:              "vp9_hw",
			Val:               mediaRecorderPerfTest{enableHWAccel: true, profile: "VP9"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			// TODO(crbug.com/811912): Use pre.ChromeVideoWithFakeWebcam() when VP9 encoder is enabled by default.
			Pre: pre.ChromeVideoWithFakeWebcamAndVP9VaapiEncoder(),
		}},
	})
}

// MediaRecorderPerf captures the perf data of MediaRecorder for HW and SW
// cases with a given codec and uploads to server.
func MediaRecorderPerf(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(mediaRecorderPerfTest)
	if err := mediarecorder.MeasurePerf(ctx, s.PreValue().(*chrome.Chrome), s.DataFileSystem(), s.OutDir(), testOpt.profile, testOpt.enableHWAccel); err != nil {
		s.Error("Failed to measure performance: ", err)
	}
}
