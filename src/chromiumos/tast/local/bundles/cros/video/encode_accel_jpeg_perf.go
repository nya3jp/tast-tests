// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

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
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/cpu"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/chrome/bintest"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EncodeAccelJPEGPerf,
		Desc:         "Measures jpeg_encode_accelerator_unittest performance",
		Contacts:     []string{"dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{caps.HWEncodeJPEG},
		Data:         []string{"lake_4160x3120_P420.yuv"},
		Timeout:      3 * time.Minute,
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
		testFileName = "lake_4160x3120_P420.yuv"
		// Suffix added to the test filename when running JPEG encode tests.
		testFileSuffix = ":4160x3120"
	)

	// CPU frequency scaling and thermal throttling might influence our test results.
	shortCtx, cleanupBenchmark, err := cpu.SetUpBenchmark(ctx)
	if err != nil {
		s.Fatal("Failed to set up benchmark mode: ", err)
	}
	defer cleanupBenchmark()

	// Run all non-cleanup operations with a shorter context. This ensures
	// thermal throttling and CPU frequency scaling get re-enabled, even when
	// test execution exceeds the maximum time allowed.
	shortCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Execute the test binary.
	s.Log("Measuring JPEG encode performance")
	testLogPath := filepath.Join(s.OutDir(), "test.log")
	args := []string{
		logging.ChromeVmoduleFlag(),
		"--repeat=" + strconv.Itoa(perfJPEGEncodeTimes),
		"--output_log=" + testLogPath,
		"--gtest_filter=" + filter,
		"--yuv_filenames=" + s.DataPath(testFileName) + testFileSuffix,
	}
	const exec = "jpeg_encode_accelerator_unittest"
	if _, err := bintest.Run(shortCtx, exec, args, s.OutDir()); err != nil {
		s.Fatalf("Failed to run %v: %v", exec, err)
	}

	// Parse and write test results.
	if err := parseJPEGEncodeLog(testLogPath, s.OutDir(), testFileName); err != nil {
		s.Fatal("Failed to parse test log: ", err)
	}
}

// parseJPEGEncodeLog parses and processes the log file created by the JPEG
// encode test and outputs the results to results-chart.json.
func parseJPEGEncodeLog(testLogPath, outputDir, testFilename string) error {
	file, err := os.Open(testLogPath)
	if err != nil {
		return errors.Wrap(err, "couldn't open log file")
	}
	defer file.Close()

	var encodeTimesHW, encodeTimesSW []int
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		tokens := strings.Split(line, ":")
		if len(tokens) != 2 {
			return errors.Errorf("wrong number of tokens on line %q", line)
		}
		time, err := strconv.ParseUint(strings.TrimSpace(tokens[1]), 10, 32)
		if err != nil {
			return errors.Wrapf(err, "failed to parse time from line %q", line)
		}

		if name := strings.TrimSpace(tokens[0]); name == "hw_encode_time" {
			encodeTimesHW = append(encodeTimesHW, int(time))
		} else if name == "sw_encode_time" {
			encodeTimesSW = append(encodeTimesSW, int(time))
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
func calculatePercentiles(p *perf.Values, encodeTimes []int, metricName string) error {
	if len(encodeTimes) == 0 {
		return errors.New("list of encode times is empty")
	}

	sort.Ints(encodeTimes)
	percentile50 := len(encodeTimes) / 2
	percentile75 := len(encodeTimes) * 3 / 4
	percentile95 := len(encodeTimes) * 95 / 100

	p.Set(perf.Metric{
		Name:      metricName + ".encode_latency.50_percentile",
		Unit:      "millisecond",
		Direction: perf.SmallerIsBetter,
	}, float64(encodeTimes[percentile50]))

	p.Set(perf.Metric{
		Name:      metricName + ".encode_latency.75_percentile",
		Unit:      "millisecond",
		Direction: perf.SmallerIsBetter,
	}, float64(encodeTimes[percentile75]))

	p.Set(perf.Metric{
		Name:      metricName + ".encode_latency.95_percentile",
		Unit:      "millisecond",
		Direction: perf.SmallerIsBetter,
	}, float64(encodeTimes[percentile95]))

	return nil
}
