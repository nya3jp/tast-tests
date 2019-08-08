// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/binsetup"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/sysutil"
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
		swFilter = "MjpegDecodeAcceleratorTest.PerfSW"
		// GTest filter used to run HW JPEG decode tests.
		hwFilter = "MjpegDecodeAcceleratorTest.PerfJDA/SHMEM"
		// Number of JPEG decodes, needs to be high enough to run for measurement duration.
		perfJPEGDecodeTimes = 10000
		// time reserved for cleanup.
		cleanupTime = 5 * time.Second
	)

	// Move all files required by the JPEG decode test to a temp dir, as
	// testing.State doesn't guarantee all files are located in the same dir.
	tempDir := binsetup.CreateTempDataDir(s, "DecodeAccelJPEGPerf.tast.", jpegPerfTestFiles)
	defer os.RemoveAll(tempDir)

	// Stop the UI job. While this isn't required to run the test binary, it's
	// possible a previous tests left tabs open or an animation is playing,
	// influencing our performance results.
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to stop ui: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	cleanUpBenchmark, err := cpu.SetUpBenchmark(ctx)
	if err != nil {
		s.Fatal("Failed to set up benchmark mode: ", err)
	}
	defer cleanUpBenchmark(ctx)

	// Reserve time for cleanup and restarting the ui job at the end of the test.
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait for CPU to become idle: ", err)
	}

	s.Log("Measuring SW JPEG decode performance")
	cpuUsageSW, decodeLatencySW := runJPEGPerfBenchmark(ctx, s, tempDir,
		measureDuration, perfJPEGDecodeTimes, swFilter)
	s.Log("Measuring HW JPEG decode performance")
	cpuUsageHW, decodeLatencyHW := runJPEGPerfBenchmark(ctx, s, tempDir,
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
	p.Set(perf.Metric{
		Name:      "tast_sw_jpeg_decode_latency",
		Unit:      "milliseconds",
		Direction: perf.SmallerIsBetter,
	}, decodeLatencySW.Seconds()*1000)
	p.Set(perf.Metric{
		Name:      "tast_hw_jpeg_decode_latency",
		Unit:      "milliseconds",
		Direction: perf.SmallerIsBetter,
	}, decodeLatencyHW.Seconds()*1000)
	p.Save(s.OutDir())
}

// runJPEGPerfBenchmark runs the JPEG decode accelerator unittest binary, and
// returns the measured CPU usage percentage and decode latency.
func runJPEGPerfBenchmark(ctx context.Context, s *testing.State, tempDir string,
	measureDuration time.Duration, perfJPEGDecodeTimes int, filter string) (float64, time.Duration) {
	// Measures CPU usage while running the unittest, and waits for the unittest
	// process to finish for the complete logs.
	const exec = "jpeg_decode_accelerator_unittest"
	logPath := fmt.Sprintf("%s/%s.%s.log", s.OutDir(), exec, filter)
	cpuUsage, err := cpu.MeasureProcessCPU(ctx, measureDuration, cpu.WaitProcess,
		[]*gtest.GTest{gtest.New(
			filepath.Join(chrome.BinTestDir, exec),
			gtest.Logfile(logPath),
			gtest.Filter(filter),
			gtest.ExtraArgs(
				"--perf_decode_times="+strconv.Itoa(perfJPEGDecodeTimes),
				"--test_data_path="+tempDir+"/"),
			gtest.UID(int(sysutil.ChronosUID)),
		)})
	if err != nil {
		s.Fatalf("Failed to measure CPU usage %v: %v", exec, err)
	}

	// Parse the log file for the decode latency measured by the unittest.
	decodeLatency, err := parseJPEGDecodeLog(logPath, s.OutDir())
	if err != nil {
		s.Fatal("Failed to parse test log: ", err)
	}

	// Check the total decoding time is longer than the measure duration. If not,
	// the measured CPU usage is inaccurate and we should fail this test.
	if decodeLatency*time.Duration(perfJPEGDecodeTimes) < measureDuration {
		s.Fatal("Decoder did not run long enough for measuring CPU usage")
	}

	return cpuUsage, decodeLatency
}

// parseJPEGDecodeLog parses the log file created by the JPEG decode accelerator
// unitttest and returns the averaged decode latency.
func parseJPEGDecodeLog(logPath, outputDir string) (time.Duration, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return 0.0, errors.Wrap(err, "couldn't open log file")
	}
	defer file.Close()

	// The log format printed from unittest looks like:
	//   [...] 27.5416 s for 10000 iterations (avg: 0.002754 s) ...
	pattern := regexp.MustCompile(`\(avg: (\d+(?:\.\d*)?) s\)`)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		match := pattern.FindStringSubmatch(scanner.Text())
		if len(match) != 2 {
			continue
		}
		decodeLatency, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			return 0.0, errors.Wrapf(err, "failed to parse decode latency: %v", match[1])
		}
		return time.Duration(decodeLatency * float64(time.Second)), nil
	}
	return 0.0, errors.New("couldn't find decode latency in log file")
}
