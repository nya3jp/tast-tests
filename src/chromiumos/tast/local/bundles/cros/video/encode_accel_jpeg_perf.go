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
		SoftwareDeps: []string{"chrome", caps.HWEncodeJPEG},
		Data:         []string{"lake_4160x3120_P420.yuv"},
		Timeout:      3 * time.Minute,
	})
}

// EncodeAccelJPEGPerf measures SW/HW JPEG encode performance by running the
// SimpleEncode test in jpeg_encode_accelerator_unittest.
func EncodeAccelJPEGPerf(ctx context.Context, s *testing.State) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	const (
		// GTest filter used to run JPEG encode tests.
		filter = "JpegEncodeAcceleratorTest.SimpleEncode"
		// Number of JPEG encodes.
		perfJPEGEncodeTimes = 100
		// Name of the test file used.
		testFilename = "lake_4160x3120_P420.yuv"
		// Suffix added to the test filename when running JPEG encode tests.
		testFileSuffix = ":4160x3120"
	)

	shortCtx, cleanupBenchmark, err := cpu.SetUpBenchmark(ctx)
	if err != nil {
		s.Fatal("Failed to set up benchmark mode: ", err)
	}
	defer cleanupBenchmark()

	// Execute the test binary.
	s.Log("Measuring JPEG encode performance")
	testLogPath := filepath.Join(s.OutDir(), "test.log")
	args := []string{
		logging.ChromeVmoduleFlag(),
		"--repeat=" + strconv.Itoa(perfJPEGEncodeTimes),
		"--output_log=" + testLogPath,
		"--gtest_filter=" + filter,
		"--yuv_filenames=" + s.DataPath(testFilename) + testFileSuffix,
	}
	const exec = "jpeg_encode_accelerator_unittest"
	if ts, err := bintest.Run(shortCtx, exec, args, s.OutDir()); err != nil {
		s.Errorf("Failed to run %v: %v", exec, err)
		for _, t := range ts {
			s.Error(t, " failed")
		}
	}

	// Parse and write performance data.
	if err := parseJPEGEncodeLog(testLogPath, s.OutDir(), testFilename); err != nil {
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

		var dur time.Duration
		if usec, err := strconv.ParseUint(strings.TrimSpace(tokens[1]), 10, 32); err != nil {
			return errors.Wrapf(err, "failed to parse time from line %q", line)
		} else {
			dur = time.Duration(usec)*time.Microsecond
		}

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
