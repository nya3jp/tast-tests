// Copyright 2020 The Chromium OS Authors. All rights reserved.
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

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF019DecodeAccelJPEGPerf,
		Desc:         "Measures jpeg_decode_accelerator_unittest performance",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeJPEG},
		Data:         []string{decodeAccelJpegPerfTestFile},
		// The default timeout is not long enough for the unittest to finish. Set the
		// timeout to 8m so the decode latency could be up to 20ms:
		//   20 ms * 10000 times * 2 runs (SW,HW) + 1 min (CPU idle time) < 8 min.
		Timeout: 8 * time.Minute,
		Pre:     chrome.LoginReuse(),
	})
}

const decodeAccelJpegPerfTestFile = "peach_pi-1280x720.jpg"

// MTBF019DecodeAccelJPEGPerf measures SW/HW jpeg decode performance by running the
// PerfSW and PerfJDA tests in the jpeg_decode_accelerator_unittest.
// TODO(dstaessens@) Currently the performance tests decode JPEGs as fast as
// possible. But this means a performant HW decoder might actually increase
// CPU usage, as the CPU becomes the bottleneck.
func MTBF019DecodeAccelJPEGPerf(ctx context.Context, s *testing.State) {
	const (
		// Duration of the interval during which CPU usage will be measured.
		measureDuration = 10 * time.Second
		// GTest filter used to run SW JPEG decode tests.
		swFilter = "MjpegDecodeAcceleratorTest.PerfSW"
		// GTest filter used to run HW JPEG decode tests.
		hwFilter = "All/MjpegDecodeAcceleratorTest.PerfJDA/DMABUF"
		// Number of JPEG decodes, needs to be high enough to run for measurement duration.
		perfJPEGDecodeTimes = 10000
		// time reserved for cleanup.
		cleanupTime = 5 * time.Second
	)

	testDir := filepath.Dir(s.DataPath(decodeAccelJpegPerfTestFile))

	cleanUpBenchmark, err := cpu.SetUpBenchmark(ctx)
	if err != nil {
		s.Fatal("Failed to set up benchmark mode: ", err)
	}
	defer cleanUpBenchmark(ctx)

	// Reserve time for cleanup and restarting the ui job at the end of the test.
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.VideoCPUIdle, err))
	}

	s.Log("Measuring SW JPEG decode performance")
	cpuUsageSW, decodeLatencySW := runJPEGPerfBenchmark(ctx, s, testDir,
		measureDuration, perfJPEGDecodeTimes, swFilter)
	s.Log("Measuring HW JPEG decode performance")
	cpuUsageHW, decodeLatencyHW := runJPEGPerfBenchmark(ctx, s, testDir,
		measureDuration, perfJPEGDecodeTimes, hwFilter)

	p := perf.NewValues()
	p.Set(perf.Metric{
		Name:      "sw_jpeg_decode_cpu",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, cpuUsageSW)
	p.Set(perf.Metric{
		Name:      "hw_jpeg_decode_cpu",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, cpuUsageHW)
	p.Set(perf.Metric{
		Name:      "sw_jpeg_decode_latency",
		Unit:      "milliseconds",
		Direction: perf.SmallerIsBetter,
	}, decodeLatencySW.Seconds()*1000)
	p.Set(perf.Metric{
		Name:      "hw_jpeg_decode_latency",
		Unit:      "milliseconds",
		Direction: perf.SmallerIsBetter,
	}, decodeLatencyHW.Seconds()*1000)
	p.Save(s.OutDir())
}

// runJPEGPerfBenchmark runs the JPEG decode accelerator unittest binary, and
// returns the measured CPU usage percentage and decode latency.
func runJPEGPerfBenchmark(ctx context.Context, s *testing.State, testDir string,
	measureDuration time.Duration, perfJPEGDecodeTimes int, filter string) (float64, time.Duration) {
	// Measures CPU usage while running the unittest, and waits for the unittest
	// process to finish for the complete logs.
	const exec = "jpeg_decode_accelerator_unittest"
	logPath := fmt.Sprintf("%s/%s.%s.log", s.OutDir(), exec, filter)
	measurements, err := cpu.MeasureProcessUsage(ctx, measureDuration, cpu.WaitProcess,
		gtest.New(
			filepath.Join(chrome.BinTestDir, exec),
			gtest.Logfile(logPath),
			gtest.Filter(filter),
			gtest.ExtraArgs(
				"--perf_decode_times="+strconv.Itoa(perfJPEGDecodeTimes),
				"--test_data_path="+testDir+"/",
				"--jpeg_filenames="+decodeAccelJpegPerfTestFile),
			gtest.UID(int(sysutil.ChronosUID)),
		))
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.VideoCPUMeasure, err, exec))
	}
	cpuUsage := measurements["cpu"]

	// Parse the log file for the decode latency measured by the unittest.
	decodeLatency, err := parseJPEGDecodeLog(logPath, s.OutDir())
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.VideoParseLog, err))
	}

	// Check the total decoding time is longer than the measure duration. If not,
	// the measured CPU usage is inaccurate and we should fail this test.
	if decodeLatency*time.Duration(perfJPEGDecodeTimes) < measureDuration {
		s.Fatal(mtbferrors.New(mtbferrors.VideoDecodeRun, err, exec))
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
