// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/bundles/cros/webrtc/mediarecorder"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// mediaRecorderPerfTest is used to describe the config used to run each test case.
type mediaRecorderPerfTest struct {
	enableHWAccel bool   // Instruct to use hardware or software encoding.
	profile       string // Codec to try, e.g. VP8, VP9.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         MediaRecorderPerf,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Captures performance data about MediaRecorder for both SW and HW",
		Contacts: []string{
			"mcasas@chromium.org", // Test author.
			"hiroh@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"loopback_media_recorder.html"},
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name:              "h264_sw",
			Val:               mediaRecorderPerfTest{enableHWAccel: false, profile: "H264"},
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			Fixture:           "chromeVideoWithFakeWebcamAndSWEncoding",
		}, {
			Name:    "vp8_sw",
			Val:     mediaRecorderPerfTest{enableHWAccel: false, profile: "VP8"},
			Fixture: "chromeVideoWithFakeWebcamAndSWEncoding",
		}, {
			Name:    "vp9_sw",
			Val:     mediaRecorderPerfTest{enableHWAccel: false, profile: "VP9"},
			Fixture: "chromeVideoWithFakeWebcamAndSWEncoding",
		}, {
			Name:              "h264_hw",
			Val:               mediaRecorderPerfTest{enableHWAccel: true, profile: "H264"},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "vp8_hw",
			Val:               mediaRecorderPerfTest{enableHWAccel: true, profile: "VP8"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "vp9_hw",
			Val:               mediaRecorderPerfTest{enableHWAccel: true, profile: "VP9"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcam",
		}},
	})
}

// MediaRecorderPerf captures the perf data of MediaRecorder for HW and SW
// cases with a given codec and uploads to server.
func MediaRecorderPerf(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(mediaRecorderPerfTest)
	if err := mediarecorder.MeasurePerf(ctx, s.FixtValue().(*chrome.Chrome), s.DataFileSystem(), s.OutDir(), testOpt.profile, testOpt.enableHWAccel); err != nil {
		s.Error("Failed to measure performance: ", err)
	}
}
