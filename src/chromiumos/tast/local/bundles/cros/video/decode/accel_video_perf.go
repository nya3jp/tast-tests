// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package decode

import (
	"context"
	"encoding/json"
	"os"
	"syscall"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/video/lib/cpu"
	"chromiumos/tast/local/chrome/bintest"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// RunAccelVideoPerfTest runs video_decode_accelerator_perf_tests with the
// specified video file.
func RunAccelVideoPerfTest(ctx context.Context, s *testing.State, filename string) {
	const (
		// GTest filter used to run SW JPEG decode tests.
		cappedTestname = "MeasureCappedPerformance"
		// GTest filter used to run HW JPEG decode tests.
		uncappedTestname = "MeasureUncappedPerformance"
		// Time to wait for CPU to stabilize after launching test binary.
		stabilizationDuration = 1 * time.Second
		// Duration of the interval during which CPU usage will be measured.
		measurementDuration = 60 * time.Second
	)

	shortCtx, cleanupBenchmark, err := cpu.SetUpBenchmark(ctx)
	if err != nil {
		s.Fatal("Failed to set up benchmark mode: ", err)
	}
	defer cleanupBenchmark()

	// Test 1: Uncapped performance
	// TODO(dstaessens@) Investigate first frame delivery time being higher than
	// when running binary manually.
	uncappedArgs := []string{
		"--gtest_filter=*" + uncappedTestname,
		"--output_folder=" + s.OutDir(),
		s.DataPath(filename),
		s.DataPath(filename + ".json"),
	}

	const exec = "video_decode_accelerator_perf_tests"
	if ts, err := bintest.Run(shortCtx, exec, uncappedArgs, s.OutDir()); err != nil {
		s.Errorf("Failed to run %v with video %s: %v", exec, filename, err)
		for _, t := range ts {
			s.Error(t, " failed")
		}
	}

	p := perf.NewValues()
	if err := parseUncappedPerformanceMetrics(s.OutDir()+"/"+uncappedTestname+".json", p); err != nil {
		s.Fatal("Failed to parse uncapped performance metrics: ", err)
	}

	// Test 2: Capped performance
	cappedArgs := []string{
		"--gtest_filter=*" + cappedTestname,
		"--output_folder=" + s.OutDir(),
		s.DataPath(filename),
		s.DataPath(filename + ".json"),
	}

	if ts, err := bintest.Run(shortCtx, exec, cappedArgs, s.OutDir()); err != nil {
		s.Errorf("Failed to run %v with video %s: %v", exec, filename, err)
		for _, t := range ts {
			s.Error(t, " failed")
		}
	}

	if err := parseCappedPerformanceMetrics(s.OutDir()+"/"+cappedTestname+".json", p); err != nil {
		s.Fatal("Failed to parse capped performance metrics: ", err)
	}

	// Test3: CPU usage while running capped performance test
	// TODO(dstaessens@) Investigate collecting CPU usage in the previous test.
	cappedArgs = append(cappedArgs, "--gtest_repeat=-1")
	cmd, err := bintest.RunAsync(ctx, exec, cappedArgs, nil, s.OutDir())
	if err != nil {
		s.Fatalf("Failed to run %v: %v", exec, err)
	}

	s.Logf("Sleeping %v to wait for CPU usage to stabilize", stabilizationDuration.Round(time.Second))
	if err := testing.Sleep(ctx, stabilizationDuration); err != nil {
		s.Fatal("Failed waiting for CPU usage to stabilize: ", err)
	}

	s.Logf("Sleeping %v to measure CPU usage", measurementDuration.Round(time.Second))
	cpuUsage, err := cpu.MeasureUsage(ctx, measurementDuration)
	if err != nil {
		s.Fatal("Failed to measure CPU usage: ", err)
	}

	// We got our measurements, now kill the process. After killing a process we
	// still need to wait for all resources to get released.
	if err := cmd.Kill(); err != nil {
		s.Fatalf("Failed to kill %v: %v", exec, err)
	}
	if err := cmd.Wait(); err != nil {
		ws, _ := testexec.GetWaitStatus(err)
		if !ws.Signaled() || ws.Signal() != syscall.SIGKILL {
			s.Fatalf("Failed to run %v: %v", exec, err)
		}
	}

	p.Set(perf.Metric{
		Name:      "tast_cpu_usage",
		Unit:      "ratio",
		Direction: perf.SmallerIsBetter,
	}, cpuUsage)

	p.Save(s.OutDir())
}

func parseUncappedPerformanceMetrics(metricsFileName string, p *perf.Values) error {
	f, err := os.Open(metricsFileName)
	if err != nil {
		return errors.Wrapf(err, "failed to open %s", metricsFileName)
	}
	defer f.Close()

	type metricsData struct {
		FrameDeliveryTimePercentile25 float64   `json:"FrameDeliveryTimePercentile25"`
		FrameDeliveryTimePercentile50 float64   `json:"FrameDeliveryTimePercentile50"`
		FrameDeliveryTimePercentile75 float64   `json:"FrameDeliveryTimePercentile75"`
		FrameDeliveryTimes            []float64 `json:"FrameDeliveryTimes"`
	}

	var metrics metricsData
	if err := json.NewDecoder(f).Decode(&metrics); err != nil {
		return errors.Wrapf(err, "failed decoding %s", metricsFileName)
	}

	// TODO(dstaessens@): Remove "tast_" prefix after removing video_VDAPerf in autotest.
	p.Set(perf.Metric{
		Name:      "delivery_time.first",
		Unit:      "milliseconds",
		Direction: perf.SmallerIsBetter,
	}, metrics.FrameDeliveryTimes[0])
	p.Set(perf.Metric{
		Name:      "tast_delivery_time.percentile_0.25",
		Unit:      "milliseconds",
		Direction: perf.SmallerIsBetter,
	}, metrics.FrameDeliveryTimePercentile25)
	p.Set(perf.Metric{
		Name:      "tast_delivery_time.percentile_0.50",
		Unit:      "milliseconds",
		Direction: perf.SmallerIsBetter,
	}, metrics.FrameDeliveryTimePercentile50)
	p.Set(perf.Metric{
		Name:      "tast_delivery_time.percentile_0.75",
		Unit:      "milliseconds",
		Direction: perf.SmallerIsBetter,
	}, metrics.FrameDeliveryTimePercentile75)

	return nil
}

func parseCappedPerformanceMetrics(metricsFileName string, p *perf.Values) error {
	f, err := os.Open(metricsFileName)
	if err != nil {
		return errors.Wrapf(err, "failed to open %s", metricsFileName)
	}
	defer f.Close()

	type metricsData struct {
		DroppedFrameRate            float64 `json:"DroppedFrameRate"`
		FrameDecodeTimePercentile50 float64 `json:"FrameDecodeTimePercentile50"`
	}

	var metrics metricsData
	if err := json.NewDecoder(f).Decode(&metrics); err != nil {
		return errors.Wrapf(err, "failed decoding %s", metricsFileName)
	}

	// TODO(dstaessens@): Remove "tast_" prefix after removing video_VDAPerf in autotest.
	p.Set(perf.Metric{
		Name:      "frame_drop_rate",
		Unit:      "ratio",
		Direction: perf.SmallerIsBetter,
	}, metrics.DroppedFrameRate)
	p.Set(perf.Metric{
		Name:      "decode_time.percentile_0.50",
		Unit:      "milliseconds",
		Direction: perf.SmallerIsBetter,
	}, metrics.FrameDecodeTimePercentile50)

	return nil
}
