// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const libyuvUnitTestBin = "libyuv_perftest"

func init() {
	testing.AddTest(&testing.Test{
		Func: PlatformLibYUVPerftest,
		Desc: "Runs libyuv unit tests as perf tests",
		Contacts: []string{
			"pmolinalopez@google.com",
			"chromeos-gfx-video@google.com",
		},
		Attr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		Params: []testing.Param{{
			Name: "yuy2tonv12",
			Val:  "LibYUVConvertTest.YUY2ToNV12_Opt",
		}, {
			Name: "nv12scale",
			Val:  "LibYUVScaleTest.NV12ScaleDownBy2_Bilinear",
		}, {
			Name:              "mm21tonv12",
			Val:               "LibYUVConvertTest.MM21ToNV12_Opt",
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsV4L2StatelessVideoDecoding(), hwdep.SkipOnPlatform("bob", "gru", "kevin")),
		}, {
			Name:              "mm21toi420",
			Val:               "LibYUVConvertTest.MM21ToI420_Opt",
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsV4L2StatelessVideoDecoding(), hwdep.SkipOnPlatform("bob", "gru", "kevin")),
		}},
	})
}

// PlatformLibYUVPerftest runs a libyuv unit test with and without assembly
// optimization to compare performance.
func PlatformLibYUVPerftest(ctx context.Context, s *testing.State) {
	env := []string{"LIBYUV_REPEAT=1000",
		"LIBYUV_WIDTH=640",
		"LIBYUV_HEIGHT=360"}

	logFileOpt := filepath.Join(s.OutDir(), libyuvUnitTestBin+"_opt.log")
	if err := runLIBYUVUnittest(ctx, logFileOpt, s.Param().(string), env); err != nil {
		s.Fatal("Failed to run test binary: ", err)
	}

	timeOpt, err := extractTime(logFileOpt)
	if err != nil {
		s.Fatalf("Failed to extract time from log file %s: %v", logFileOpt, err)
	}

	p := perf.NewValues()
	p.Set(perf.Metric{
		Name:      "runtime_optimized",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(timeOpt))

	// Disable assembly optimization.
	env = append(env, "LIBYUV_DISABLE_ASM=1")

	logFileNoOpt := filepath.Join(s.OutDir(), libyuvUnitTestBin+"_noOpt.log")
	if err = runLIBYUVUnittest(ctx, logFileNoOpt, s.Param().(string), env); err != nil {
		s.Fatal("Failed to run test binary: ", err)
	}

	timeNoOpt, err := extractTime(logFileNoOpt)
	if err != nil {
		s.Fatalf("Failed to extract time from log file %s: %v", logFileNoOpt, err)
	}

	p.Set(perf.Metric{
		Name:      "runtime_no_optimized",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(timeNoOpt))

	if timeNoOpt != 0 {
		timeDecrease := timeNoOpt - timeOpt
		improvement := float64(timeDecrease) / float64(timeNoOpt) * 100.0

		p.Set(perf.Metric{
			Name:      "time_improvement",
			Unit:      "percent",
			Direction: perf.BiggerIsBetter,
		}, improvement)
	}

	if err := p.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save perf results: ", err)
	}
}

// runLIBYUVUnittest runs a libyuv unit test with environment env.
func runLIBYUVUnittest(ctx context.Context, logFile, testName string, env []string) error {
	f, err := os.Create(logFile)
	if err != nil {
		return errors.Wrapf(err, "failed to create log file %s", logFile)
	}
	defer f.Close()

	cmd := testexec.CommandContext(ctx, libyuvUnitTestBin, fmt.Sprintf("--gtest_filter=%v", testName))
	cmd.Stdout = f
	cmd.Stderr = f
	cmd.Env = env
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to run: %v", libyuvUnitTestBin)
	}
	return nil
}

// extractTime parses logFile using r and returns a single int64 matching
// the time in milliseconds.
func extractTime(logFile string) (value int64, err error) {
	regExpTime := regexp.MustCompile(`\n\[==========\] .+\. \((\d+) ms total\)\n`)

	b, err := ioutil.ReadFile(logFile)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to read file %s", logFile)
	}

	matches := regExpTime.FindAllStringSubmatch(string(b), -1)
	if len(matches) != 1 {
		return 0, errors.Errorf("found %d matches in %q; want 1", len(matches), b)
	}

	matchString := matches[0][1]
	if value, err = strconv.ParseInt(matchString, 10, 64); err != nil {
		return 0, errors.Wrapf(err, "failed to parse value %q", matchString)
	}
	return value, err
}
