// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
 // "fmt"

//	"github.com/pixelbender/go-matroska/matroska"

	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/bundles/cros/video/mediarecorder"
	"chromiumos/tast/local/perf"
//	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"

)

const (
	streamFile = "crowd720_25frames.y4m"
	fps = 30
	codec = "h264"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:			MediaRecorderPerfH264,
		Desc:			"Captures performance data about MediaRecorder with H264",
		Contacts: []string{"shenghao@chromium.org", "chromeos-video-eng@google.com"},
		Attr:			[]string{"informational"},
		// "chrome_internal" is needed because H.264 is a proprietary codec.
		SoftwareDeps: []string{caps.HWEncodeH264, "chrome_login", "chrome_internal"},
		Data:					[]string{streamFile, "loopback_media_recorder.html"},
	})
}

// MediaRecorderPerfH264 captures the perf data of MediaRecorder with H264 codec and uploads to server.
func MediaRecorderPerfH264(ctx context.Context, s *testing.State) {
	//doc, err := matroska.Decode("")

	// Measure the performance of SW encode.
	
	chromeArgs := []string{
		logging.ChromeVmoduleFlag(),
		"--use-fake-ui-for-media-stream",
		//"--use-file-for-fake-video-capture=" + streamFile,
		//fmt.Sprintf("--use-fake-device-for-media-stream='fps=%q'", fps),
		"--use-fake-device-for-media-stream",
		"--disable-accelerated-video-encode",
	}
	processingTimeSW := 0
	cpuUsageSW := float64(0)
	if err := mediarecorder.MeasurePerf(ctx, s, chromeArgs, codec, &processingTimeSW, &cpuUsageSW); err != nil {
		s.Error("Failed to measure SW performance: %v", err)
	}

	if err := mediarecorder.ReportPerf("sw_frame_processing_time", "millisecond", s.OutDir(), float64(processingTimeSW), perf.SmallerIsBetter); err != nil {
		s.Error("Failed to save SW processing time metric: %v", err)
	}
	if err := mediarecorder.ReportPerf("sw_cpu_usage", "percent", s.OutDir(), cpuUsageSW, perf.SmallerIsBetter); err != nil {
		s.Error("Failed to save SW cpu usage metric: %v", err)
	}


	// Measure the performance of HW encode.
	chromeArgs = []string{
		logging.ChromeVmoduleFlag(),
		"--use-fake-ui-for-media-stream",
		//"--use-file-for-fake-video-capture=" + streamFile,
		//fmt.Sprintf("--use-fake-device-for-media-stream='fps=%q'", fps),
		"--use-fake-device-for-media-stream",
	}
	processingTimeHW := 0
	cpuUsageHW := float64(0)
	if err := mediarecorder.MeasurePerf(ctx, s, chromeArgs, codec, &processingTimeHW, &cpuUsageHW); err != nil {
		s.Error("Failed to measure HW performance: %v", err)
	}

	if err := mediarecorder.ReportPerf("hw_frame_processing_time", "millisecond", s.OutDir(), float64(processingTimeHW), perf.SmallerIsBetter); err != nil {
		s.Error("Failed to save HW processing time metric: %v", err)
	}
	if err := mediarecorder.ReportPerf("hw_cpu_usage", "percent", s.OutDir(), cpuUsageHW, perf.SmallerIsBetter); err != nil {
		s.Error("Failed to save HW cpu usage metric: %v", err)
	}
}
