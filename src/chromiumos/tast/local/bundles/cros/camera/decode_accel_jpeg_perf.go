// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"os"
	"strconv"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/video/lib/binsetup"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/cpu"
	"chromiumos/tast/local/chrome/bintest"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeAccelJPEGPerf,
		Desc:         "Measures jpeg_decode_accelerator_unittest performance",
		Contacts:     []string{"mojahsu@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeJPEG},
		Data:         jpegPerfTestFiles,
	})
}

// Test files used by the JPEG decode accelerator unittest.
// TODO(dstaessens@) Only the first file is used for performance testing, but
// the other files are also loaded by the jpeg_decode_accelerator_unittest.
// Ideally the performance test should be moved to a separate binary.
var jpegPerfTestFiles = []string{
	"peach_pi-1280x720.jpg",
	"peach_pi-40x23.jpg",
	"peach_pi-41x22.jpg",
	"peach_pi-41x23.jpg",
	"pixel-1280x720.jpg",
}

// DecodeAccelJPEGPerf measures SW/HW jpeg decode performance by running the
// PerfSW and PerfJDA tests in the jpeg_decode_accelerator_unittest.
// TODO(dstaessens@) Currently the performance tests decode JPEGs as fast as
// possible. But this means a performant HW decoder might actually increase
// CPU usage, as the CPU becomes the bottleneck.
func DecodeAccelJPEGPerf(ctx context.Context, s *testing.State) {
	const (
		// Duration of the interval during which CPU usage will be measured.
		measureDuration = 10 * time.Second
		// GTest filter used to run SW JPEG decode tests.
		swFilter = "JpegDecodeAcceleratorTest.PerfSW"
		// GTest filter used to run HW JPEG decode tests.
		hwFilter = "JpegDecodeAcceleratorTest.PerfJDA"
		// Number of JPEG decodes, needs to be high enough to run for measurement duration.
		perfJPEGDecodeTimes = 10000
		// time reserved for cleanup.
		cleanupTime = 5 * time.Second
	)

	// Move all files required by the JPEG decode test to a temp dir, as
	// testing.State doesn't guarantee all files are located in the same dir.
	tempDir := binsetup.CreateTempDataDir(s, "DecodeAccelJPEGPerf.tast.", jpegPerfTestFiles)
	defer os.RemoveAll(tempDir)

	// Reserve time for cleanup and restarting the ui job at the end of the test.
	shortCtx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	// Stop the UI job. While this isn't required to run the test binary, it's
	// possible a previous tests left tabs open or an animation is playing,
	// influencing our performance results.
	if err := upstart.StopJob(shortCtx, "ui"); err != nil {
		s.Error("Failed to stop ui: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	cleanUpBenchmark, err := cpu.SetUpBenchmark(shortCtx)
	if err != nil {
		s.Fatal("Failed to set up benchmark mode: ", err)
	}
	defer cleanUpBenchmark(ctx)

	if err := cpu.WaitUntilIdle(shortCtx); err != nil {
		s.Fatal("Failed waiting for CPU to become idle: ", err)
	}

	s.Log("Measuring SW JPEG decode performance")
	cpuUsageSW := runJPEGPerfBenchmark(shortCtx, s, tempDir,
		measureDuration, perfJPEGDecodeTimes, swFilter)
	s.Log("Measuring HW JPEG decode performance")
	cpuUsageHW := runJPEGPerfBenchmark(shortCtx, s, tempDir,
		measureDuration, perfJPEGDecodeTimes, hwFilter)

	// TODO(dstaessens@): Remove "tast_" prefix after removing video_JDAPerf in autotest.
	p := perf.NewValues()
	p.Set(perf.Metric{
		Name:      "tast_sw_jpeg_decode_cpu",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, cpuUsageSW)
	p.Set(perf.Metric{
		Name:      "tast_hw_jpeg_decode_cpu",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, cpuUsageHW)
	p.Save(s.OutDir())
}

// runBenchmark measures CPU usage while running the the JPEG decode accelerator
// unittest binary.
func runJPEGPerfBenchmark(ctx context.Context, s *testing.State, tempDir string,
	measureDuration time.Duration, perfJPEGDecodeTimes int, filter string) float64 {
	args := []string{
		"--perf_decode_times=" + strconv.Itoa(perfJPEGDecodeTimes),
		"--test_data_path=" + tempDir + "/",
		"--gtest_filter=" + filter,
	}

	const testExec = "jpeg_decode_accelerator_unittest"
	runCmdAsync := func() (*testexec.Cmd, error) {
		return bintest.RunAsync(ctx, testExec, args, nil, s.OutDir())
	}

	cpuUsage, err := cpu.MeasureProcessCPU(ctx, runCmdAsync, measureDuration)
	if err != nil {
		s.Fatalf("Failed to measure CPU usage %v: %v", testExec, err)
	}

	return cpuUsage
}
