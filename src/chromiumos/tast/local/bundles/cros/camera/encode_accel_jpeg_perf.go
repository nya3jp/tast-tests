// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EncodeAccelJPEGPerf,
		Desc:         "Measures jpeg_encode_accelerator_unittest performance",
		Contacts:     []string{"shenghao@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", caps.HWEncodeJPEG},
		Data:         []string{"coast_3840x2160_P420.yuv"},
		Timeout:      10 * time.Minute,
	})
}

// EncodeAccelJPEGPerf measures SW/HW JPEG encode performance by running the
// SimpleEncode test in jpeg_encode_accelerator_unittest.
func EncodeAccelJPEGPerf(ctx context.Context, s *testing.State) {
	const (
		// GTest filter used to run JPEG encode tests.
		filter = "JpegEncodeAcceleratorTest.SimpleEncode"
		// Number of JPEG encodes.
		perfJPEGEncodeTimes = 100
		// Name of the test file used.
		testFilename = "coast_3840x2160_P420.yuv"
		// Suffix added to the test filename when running JPEG encode tests.
		testFileSuffix = ":3840x2160"
		// time reserved for cleanup.
		cleanupTime = 10 * time.Second
	)

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
		s.Fatal("Failed waiting for CPU to become idle: ", err)
	}

	// Execute the test binary.
	s.Log("Measuring JPEG encode performance")
	const exec = "jpeg_encode_accelerator_unittest"
	logPath := filepath.Join(s.OutDir(), "test.log")
	if report, err := gtest.New(
		filepath.Join(chrome.BinTestDir, exec),
		gtest.Logfile(filepath.Join(s.OutDir(), exec+".log")),
		gtest.Filter(filter),
		gtest.ExtraArgs(
			"--repeat="+strconv.Itoa(perfJPEGEncodeTimes),
			"--output_log="+logPath,
			"--yuv_filenames="+s.DataPath(testFilename)+testFileSuffix),
		gtest.UID(int(sysutil.ChronosUID)),
	).Run(ctx); err != nil {
		s.Errorf("Failed to run %v: %v", exec, err)
		if report != nil {
			for _, name := range report.FailedTestNames() {
				s.Error(name, " failed")
			}
		}
	}

	// Parse and write performance data.
	if err := parseJPEGEncodeLog(logPath, s.OutDir(), testFilename); err != nil {
		s.Fatal("Failed to parse test log: ", err)
	}
}

// parseJPEGEncodeLog parses and processes the log file created by the JPEG
// encode test. The results are written to results-chart.json, which can be
// parsed by crosbolt.
func parseJPEGEncodeLog(testLogPath, outputDir, testFilename string) error {
	file, err := os.Open(testLogPath)
	if err != nil {
		return errors.Wrap(err, "couldn't open log file")
	}
	defer file.Close()

	var encodeTimesHW, encodeTimesSW []time.Duration
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		tokens := strings.Split(line, ":")
		if len(tokens) != 2 {
			return errors.Errorf("wrong number of tokens in line %q", line)
		}

		usec, err := strconv.ParseUint(strings.TrimSpace(tokens[1]), 10, 32)
		if err != nil {
			return errors.Wrapf(err, "failed to parse time from line %q", line)
		}
		dur := time.Duration(usec) * time.Microsecond

		if name := strings.TrimSpace(tokens[0]); name == "hw_encode_time" {
			encodeTimesHW = append(encodeTimesHW, dur)
		} else if name == "sw_encode_time" {
			encodeTimesSW = append(encodeTimesSW, dur)
		} else {
			return errors.Errorf("unexpected name %q on line %q ", name, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return errors.Wrap(err, "failed to scan test log")
	}

	// TODO(dstaessens@): Remove "tast_" prefix after removing video_JEAPerf in autotest.
	p := perf.NewValues()
	if err := calculatePercentiles(p, encodeTimesSW, "tast_sw_"+testFilename); err != nil {
		return errors.Wrap(err, "failed to calculate software percentiles")
	}
	if err := calculatePercentiles(p, encodeTimesHW, "tast_hw_"+testFilename); err != nil {
		return errors.Wrap(err, "failed to calculate hardware percentiles")
	}
	p.Save(outputDir)

	return nil
}

// calculatePercentiles caculates the 50, 75 and 95th percentile for the
// specified list of encode times, and adds them to the passed perf object.
func calculatePercentiles(p *perf.Values, encodeTimes []time.Duration, metricName string) error {
	if len(encodeTimes) == 0 {
		return errors.New("list of encode times is empty")
	}

	sort.Slice(encodeTimes, func(i, j int) bool { return encodeTimes[i] < encodeTimes[j] })
	percentile50 := len(encodeTimes) / 2
	percentile75 := len(encodeTimes) * 3 / 4
	percentile95 := len(encodeTimes) * 95 / 100

	p.Set(perf.Metric{
		Name:      metricName + ".encode_latency.50_percentile",
		Unit:      "microsecond",
		Direction: perf.SmallerIsBetter,
	}, float64(encodeTimes[percentile50]/time.Microsecond))

	p.Set(perf.Metric{
		Name:      metricName + ".encode_latency.75_percentile",
		Unit:      "microsecond",
		Direction: perf.SmallerIsBetter,
	}, float64(encodeTimes[percentile75]/time.Microsecond))

	p.Set(perf.Metric{
		Name:      metricName + ".encode_latency.95_percentile",
		Unit:      "microsecond",
		Direction: perf.SmallerIsBetter,
	}, float64(encodeTimes[percentile95]/time.Microsecond))

	return nil
}
