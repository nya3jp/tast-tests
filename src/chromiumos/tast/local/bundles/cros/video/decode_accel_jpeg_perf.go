// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/video/lib/binsetup"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/cpu"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/chrome/bintest"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeAccelJPEGPerf,
		Desc:         "Measures jpeg_decode_accelerator_unittest performance",
		Contacts:     []string{"dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{caps.HWDecodeJPEG},
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
		// Maximum time to wait for CPU to become idle.
		waitIdleCPUTimeout = 30 * time.Second
		// Average usage below which CPU is considered idle.
		idleCPUUsagePercent = 10.0
		// Time to wait for CPU to stabilize after launching test binary.
		stabilizationDuration = 1 * time.Second
		// Duration of the interval during which CPU usage will be measured.
		measurementDuration = 10 * time.Second
		// GTest filter used to run SW JPEG decode tests.
		swFilter = "JpegDecodeAcceleratorTest.PerfSW"
		// GTest filter used to run HW JPEG decode tests.
		hwFilter = "JpegDecodeAcceleratorTest.PerfJDA"
		// Number of JPEG decodes, needs to be high enough to run for measurement duration.
		perfJPEGDecodeTimes = 10000
	)

	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging: ", err)
	}
	defer vl.Close()

	// Move all files required by the JPEG decode test to a temp dir, as
	// testing.State doesn't guarantee all files are located in the same dir.
	tempDir := binsetup.CreateTempDataDir(s, "DecodeAccelJPEGPerf.tast.", jpegPerfTestFiles)
	defer os.RemoveAll(tempDir)

	// Run all non-cleanup operations with a shorter context. This ensures
	// thermal throttling and CPU frequency scaling get re-enabled, even when
	// test execution exceeds the maximum time allowed.
	shortCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// CPU frequency scaling and thermal throttling might influence our test results.
	restoreCPUFrequencyScaling, err := cpu.DisableCPUFrequencyScaling()
	if err != nil {
		s.Fatal("Failed to disable CPU frequency scaling: ", err)
	}
	defer restoreCPUFrequencyScaling()

	restoreThermalThrottling, err := cpu.DisableThermalThrottling(shortCtx)
	if err != nil {
		s.Fatal("Failed to disable thermal throttling: ", err)
	}
	defer restoreThermalThrottling(ctx)

	if err := cpu.WaitForIdle(shortCtx, waitIdleCPUTimeout, idleCPUUsagePercent); err != nil {
		s.Fatal("Failed waiting for CPU to become idle: ", err)
	}

	s.Log("Measuring SW JPEG decode performance")
	cpuUsageSW := runJPEGPerfBenchmark(shortCtx, s, tempDir, stabilizationDuration,
		measurementDuration, perfJPEGDecodeTimes, swFilter)
	s.Log("Measuring HW JPEG decode performance")
	cpuUsageHW := runJPEGPerfBenchmark(shortCtx, s, tempDir, stabilizationDuration,
		measurementDuration, perfJPEGDecodeTimes, hwFilter)

	// TODO(dstaessens@): Remove "tast_" prefix after removing video_JDAPerf in autotest.
	p := &perf.Values{}
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
	stabilizationDuration time.Duration, measurementDuration time.Duration,
	perfJPEGDecodeTimes int, filter string) float64 {
	args := []string{
		logging.ChromeVmoduleFlag(),
		"--perf_decode_times=" + strconv.Itoa(perfJPEGDecodeTimes),
		"--test_data_path=" + tempDir + "/",
		"--gtest_filter=" + filter,
	}

	const testExec = "jpeg_decode_accelerator_unittest"
	cmd, err := bintest.RunAsync(ctx, testExec, args, s.OutDir())
	if err != nil {
		s.Fatalf("Failed to run %v: %v", testExec, err)
	}

	s.Logf("Sleeping %v to wait for CPU usage to stabilize", stabilizationDuration.Round(time.Second))
	select {
	case <-ctx.Done():
		s.Fatal("Failed waiting for CPU usage to stabilize: ", err)
	case <-time.After(stabilizationDuration):
	}

	s.Logf("Sleeping %v to measure CPU usage", measurementDuration.Round(time.Second))
	cpuUsage, err := cpu.MeasureUsage(ctx, measurementDuration)
	if err != nil {
		s.Fatal("Failed to measure CPU usage: ", err)
	}

	// We got our measurements, now kill the process. After killing a process we
	// still need to wait for all resources to get released.
	if err := cmd.Kill(); err != nil {
		s.Fatalf("Failed to kill %v: %v", testExec, err)
	}
	if err := cmd.Wait(); err != nil {
		ws := err.(*exec.ExitError).Sys().(syscall.WaitStatus)
		if !ws.Signaled() || ws.Signal() != syscall.SIGKILL {
			s.Fatalf("Failed to run %v: %v", testExec, err)
		}
	}

	return cpuUsage
}
