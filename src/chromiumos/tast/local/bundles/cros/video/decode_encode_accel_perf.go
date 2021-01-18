// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/video/encode"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/media/encoding"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// Enable to cache the extracted raw video to speed up the test.
const deapCacheExtractedVideo = false

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeEncodeAccelPerf,
		Desc:         "Simulates video chat performance by simultaneously decoding and encoding a 30fps 1080p video",
		Contacts:     []string{"dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeVP8, caps.HWEncodeVP8},
		Data: []string{"1080p_30fps_300frames.vp8.ivf", "1080p_30fps_300frames.vp8.ivf.json",
			"crowd-1920x1080.vp9.webm", "crowd-1920x1080.yuv.json"},
		// Default timeout (i.e. 2 minutes) is not enough.
		Timeout: 5 * time.Minute,
	})
}

func DecodeEncodeAccelPerf(ctx context.Context, s *testing.State) {
	const (
		// Time reserved for cleanup.
		cleanupTime = 10 * time.Second
		// Time to wait for CPU to stabilize after launching tests.
		stabilize = 5 * time.Second
		// Duration of the interval during which CPU usage will be measured.
		measureDuration = 30 * time.Second
		// Filename of the video that will be decoded.
		decodeFilename = "1080p_30fps_300frames.vp8.ivf"
	)
	// Properties of the video that will be encoded.
	encodeParams := encode.Crowd1080P

	// Only a single process can have access to the GPU, so we are required to
	// call "stop ui" at the start of the test. This will shut down the chrome
	// process and allow us to claim ownership of the GPU.
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to stop ui: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	// Setup benchmark mode.
	cleanUpBenchmark, err := cpu.SetUpBenchmark(ctx)
	if err != nil {
		s.Fatal("Failed to set up benchmark mode: ", err)
	}
	defer cleanUpBenchmark(ctx)

	// Reserve time to restart the ui job and perform cleanup at the end of the test.
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	// Create a raw YUV video and JSON for it to encode for the video encoder tests.
	yuvPath, yuvJSONPath, err :=
		encoding.PrepareYUVAndJSON(ctx, s.DataPath(encodeParams.Name),
			s.DataPath(encode.YUVJSONFileNameFor(encodeParams.Name)))
	if err != nil {
		s.Fatal("Failed to prepare YUV files: ", err)
	}
	if !deapCacheExtractedVideo {
		defer os.Remove(yuvPath)
		defer os.Remove(yuvJSONPath)
	}

	// Wait for the CPU to become idle.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed waiting for CPU to become idle: ", err)
	}

	// Create gtest that runs the video encoder performance test.
	encodeTest := newGTest("video_encode_accelerator_perf_tests", "*MeasureCappedPerformance", s.OutDir(),
		[]string{
			"--codec=vp8",
			yuvPath,
			yuvJSONPath,
			"--output_folder=" + s.OutDir(),
		})
	// Create gtest that runs the video decoder performance test.
	decodeTest := newGTest("video_decode_accelerator_perf_tests", "*MeasureCappedPerformance", s.OutDir(),
		[]string{
			s.DataPath(decodeFilename),
			s.DataPath(decodeFilename + ".json"),
			"--output_folder=" + s.OutDir(),
		})

	// Measure CPU usage while both the encoder and decoder performance tests are running.
	measurements, err := cpu.MeasureProcessUsage(ctx, measureDuration, cpu.KillProcess, encodeTest, decodeTest)
	if err != nil {
		s.Fatal("Failed to measure CPU usage: ", err)
	}
	s.Logf("CPU usage: %.2f%%", measurements["cpu"])

	// Create and save performance report.
	p := perf.NewValues()
	p.Set(perf.Metric{
		Name:      "cpu_usage",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, measurements["cpu"])

	// Power measurements are not supported on all platforms.
	if power, ok := measurements["power"]; ok {
		p.Set(perf.Metric{
			Name:      "power_consumption",
			Unit:      "watt",
			Direction: perf.SmallerIsBetter,
		}, power)
	}

	if err := p.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save performance report: ", err)
	}
}

// newGTest returns a gtest object that starts the specified test binary with
// the provided arguments.
func newGTest(exec, filter, outDir string, args []string) *gtest.GTest {
	return gtest.New(
		filepath.Join(chrome.BinTestDir, exec),
		gtest.Logfile(filepath.Join(outDir, exec+".log")),
		gtest.Filter(filter),
		// Repeat enough times to run for full measurement duration. We don't
		// use -1 here as this can result in huge log files (b/138822793).
		gtest.Repeat(1000),
		gtest.ExtraArgs(args...),
		gtest.UID(int(sysutil.ChronosUID)),
	)
}
