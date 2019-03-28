// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"fmt"

	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/bundles/cros/video/mediarecorder"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

const (
	streamFile = "crowd720_25frames.y4m"
	fps        = 30
	codec      = "h264"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MediaRecorderPerfH264,
		Desc:     "Captures performance data about MediaRecorder for SW and HW with H264",
		Contacts: []string{"shenghao@chromium.org", "chromeos-video-eng@google.com"},
		Attr:     []string{"informational"},
		// "chrome_internal" is needed because H.264 is a proprietary codec.
		SoftwareDeps: []string{caps.HWEncodeH264, "chrome_login", "chrome_internal"},
		Data:         []string{streamFile, "loopback_media_recorder.html"},
	})
}

// MediaRecorderPerfH264 captures the perf data of MediaRecorder for HW and SW cases with H264 codec and uploads to server.
func MediaRecorderPerfH264(ctx context.Context, s *testing.State) {
	// Measure the performance of SW encode.
	fpsArg := fmt.Sprintf("\"fps=%d\"", fps)
	chromeArgs := []string{
		logging.ChromeVmoduleFlag(),
		"--use-fake-ui-for-media-stream",
		"--use-file-for-fake-video-capture=" + s.DataPath(streamFile),
		"--use-fake-device-for-media-stream=" + fpsArg,
		"--disable-accelerated-video-encode",
	}
	processingTimeSW := 0
	cpuUsageSW := float64(0)
	if err := mediarecorder.MeasurePerf(ctx, s.DataFileSystem(), s.OutDir(), chromeArgs, codec, &processingTimeSW, &cpuUsageSW); err != nil {
		s.Error("Failed to measure SW performance: ", err)
	}

	if err := mediarecorder.ReportPerf("sw_frame_processing_time", "millisecond", s.OutDir(), float64(processingTimeSW), perf.SmallerIsBetter); err != nil {
		s.Error("Failed to save SW processing time metric: ", err)
	}
	if err := mediarecorder.ReportPerf("sw_cpu_usage", "percent", s.OutDir(), cpuUsageSW, perf.SmallerIsBetter); err != nil {
		s.Error("Failed to save SW cpu usage metric: ", err)
	}

	// Measure the performance of HW encode.
	chromeArgs = []string{
		logging.ChromeVmoduleFlag(),
		"--use-fake-ui-for-media-stream",
		"--use-file-for-fake-video-capture=" + s.DataPath(streamFile),
		"--use-fake-device-for-media-stream=" + fpsArg,
	}
	processingTimeHW := 0
	cpuUsageHW := float64(0)
	if err := mediarecorder.MeasurePerf(ctx, s.DataFileSystem(), s.OutDir(), chromeArgs, codec, &processingTimeHW, &cpuUsageHW); err != nil {
		s.Error("Failed to measure HW performance: ", err)
	}

	if err := mediarecorder.ReportPerf("hw_frame_processing_time", "millisecond", s.OutDir(), float64(processingTimeHW), perf.SmallerIsBetter); err != nil {
		s.Error("Failed to save HW processing time metric: ", err)
	}
	if err := mediarecorder.ReportPerf("hw_cpu_usage", "percent", s.OutDir(), cpuUsageHW, perf.SmallerIsBetter); err != nil {
		s.Error("Failed to save HW cpu usage metric: ", err)
	}
	testing.ContextLogf(ctx, "processingTimeSW=%d cpuUsageSW=%f processingTimeHW=%d cpuUsageHW=%f", processingTimeSW, cpuUsageSW, processingTimeHW, cpuUsageHW)
}
