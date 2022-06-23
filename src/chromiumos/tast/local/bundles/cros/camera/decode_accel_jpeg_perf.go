// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/gtest"
	mediacpu "chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeAccelJPEGPerf,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Measures jpeg_decode_accelerator_unittest performance",
		Contacts:     []string{"mojahsu@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeJPEG},
		Data:         []string{decodeAccelJpegPerfTestFile},
		// The default timeout is not long enough for the unittest to finish. Set the
		// timeout to 8m so the decode latency could be up to 20ms:
		//   20 ms * 10000 times * 2 runs (SW,HW) + 1 min (CPU idle time) < 8 min.
		Timeout: 8 * time.Minute,
	})
}

const decodeAccelJpegPerfTestFile = "peach_pi-1280x720.jpg"

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
		hwFilter = "All/MjpegDecodeAcceleratorTest.PerfJDA/DMABUF"
		// Number of JPEG decodes, needs to be high enough to run for measurement duration.
		perfJPEGDecodeTimes = 10000
		// time reserved for cleanup.
		cleanupTime = 5 * time.Second
	)

	testDir := filepath.Dir(s.DataPath(decodeAccelJpegPerfTestFile))

	// Stop the UI job. While this isn't required to run the test binary, it's
	// possible a previous tests left tabs open or an animation is playing,
	// influencing our performance results.
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to stop ui: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	cleanUpBenchmark, err := mediacpu.SetUpBenchmark(ctx)
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
	cpuUsageSW, metricsSW := runJPEGPerfBenchmark(ctx, s, testDir,
		measureDuration, perfJPEGDecodeTimes, swFilter, "sw")
	s.Log("Measuring HW JPEG decode performance")
	cpuUsageHW, metricsHW := runJPEGPerfBenchmark(ctx, s, testDir,
		measureDuration, perfJPEGDecodeTimes, hwFilter, "hw")

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
	for name, value := range metricsSW {
		p.Set(perf.Metric{
			Name:      name,
			Unit:      "milliseconds",
			Direction: perf.SmallerIsBetter,
		}, value)
	}
	for name, value := range metricsHW {
		p.Set(perf.Metric{
			Name:      name,
			Unit:      "milliseconds",
			Direction: perf.SmallerIsBetter,
		}, value)
	}
	p.Save(s.OutDir())
}

// runJPEGPerfBenchmark runs the JPEG decode accelerator unittest binary, and
// returns the measured CPU usage percentage and decode latency.
func runJPEGPerfBenchmark(ctx context.Context, s *testing.State, testDir string,
	measureDuration time.Duration, perfJPEGDecodeTimes int, filter, id string) (float64, map[string]float64) {
	// Measures CPU usage while running the unittest, and waits for the unittest
	// process to finish for the complete logs.
	const exec = "jpeg_decode_accelerator_unittest"
	logPath := fmt.Sprintf("%s/%s.%s.log", s.OutDir(), exec, id)
	outPath := fmt.Sprintf("%s/perf_output.%s.json", s.OutDir(), id)
	startTime := time.Now()
	measurements, err := measureProcessUsage(ctx, measureDuration,
		gtest.New(
			filepath.Join(chrome.BinTestDir, exec),
			gtest.Logfile(logPath),
			gtest.Filter(filter),
			gtest.ExtraArgs(
				"--perf_decode_times="+strconv.Itoa(perfJPEGDecodeTimes),
				"--perf_output_path="+outPath,
				"--test_data_path="+testDir+"/",
				"--jpeg_filenames="+decodeAccelJpegPerfTestFile),
			gtest.UID(int(sysutil.ChronosUID)),
		))
	if err != nil {
		s.Fatalf("Failed to measure CPU usage %v: %v", exec, err)
	}
	cpuUsage := measurements["cpu"]
	duration := time.Since(startTime)

	// Check the total decoding time is longer than the measure duration. If not,
	// the measured CPU usage is inaccurate and we should fail this test.
	if duration < measureDuration {
		s.Fatal("Decoder did not run long enough for measuring CPU usage")
	}

	// Parse the log file for the decode latency measured by the unittest.
	out, err := ioutil.ReadFile(outPath)
	if err != nil {
		s.Fatal("Failed to read output file: ", err)
	}
	var metrics map[string]float64
	if err := json.Unmarshal(out, &metrics); err != nil {
		s.Fatal("Failed to parse output file: ", err)
	}

	return cpuUsage, metrics
}

// measureProcessUsage starts one or more gtest processes and measures CPU usage and power consumption asynchronously
// for the given duration. A map is returned containing CPU usage (percentage in [0-100] range) with key "cpu" and power
// consumption (Watts) with key "power" if supported.
func measureProcessUsage(ctx context.Context, duration time.Duration, ts ...*gtest.GTest) (measurements map[string]float64, retErr error) {
	const (
		stabilizeTime = 1 * time.Second // time to wait for CPU to stabilize after launching proc.
		cleanupTime   = 5 * time.Second // time reserved for cleanup after measuring.
	)

	for _, t := range ts {
		// Start the process asynchronously by calling the provided startup function.
		cmd, err := t.Start(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to run binary")
		}

		// Clean up the process upon exiting the function.
		defer func() {
			// Wait for the process to terminate.
			if err := cmd.Wait(); err != nil {
				retErr = err
				testing.ContextLog(ctx, "Failed waiting for the command to exit: ", retErr)
			}
			return

			// If the exit option is 'KillProcess' we will send a 'SIGKILL' signal
			// to the process after collecting performance metrics.
			if err := cmd.Kill(); err != nil {
				retErr = err
				testing.ContextLog(ctx, "Failed to kill process: ", retErr)
				return
			}

			// After sending a 'SIGKILL' signal to the process we need to wait
			// for the process to terminate. If Wait() doesn't return any error,
			// we know the process already terminated before we explicitly killed
			// it and the measured performance metrics are invalid.
			err = cmd.Wait()
			if err == nil {
				retErr = errors.New("process did not run for entire measurement duration")
				testing.ContextLog(ctx, retErr)
				return
			}

			// Check whether the process was terminated with a 'SIGKILL' signal.
			ws, ok := testexec.GetWaitStatus(err)
			if !ok {
				retErr = errors.Wrap(err, "failed to get wait status")
				testing.ContextLog(ctx, retErr)
			} else if !ws.Signaled() || ws.Signal() != syscall.SIGKILL {
				retErr = errors.Wrap(err, "process did not terminate with SIGKILL signal")
				testing.ContextLog(ctx, retErr)
			}
		}()
	}

	// Use a shorter context to leave time for cleanup upon failure.
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	if err := testing.Sleep(ctx, stabilizeTime); err != nil {
		return nil, errors.Wrap(err, "failed waiting for CPU usage to stabilize")
	}

	testing.ContextLog(ctx, "Measuring CPU usage and power consumption for ", duration.Round(time.Second))
	return mediacpu.MeasureUsage(ctx, duration)
}
