// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package decoding

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/gtest"
	mediacpu "chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

// DecoderType represents the different video decoder types.
type DecoderType int

const (
	// VDA is the video decoder type based on the VideoDecodeAccelerator
	// interface. These are set to be deprecrated.
	VDA DecoderType = iota
	// VD is the video decoder type based on the VideoDecoder interface. These
	// will replace the current VDAs.
	VD
	// VDVDA refers to an adapter between the arc.mojom VideoDecodeAccelerator
	// and the VideoDecoder-based video decode accelerator. This entry is used
	// to test interaction with older interface that expected the VDA interface.
	VDVDA
)

// ValidatorType represents the validator types used in video_decode_accelerator_tests.
type ValidatorType int

const (
	// MD5 is to validate the correctness of decoded frames by comparing with
	// md5hash of expected frames.
	MD5 ValidatorType = iota
	// SSIM is to validate the correctness of decoded frames by computing SSIM
	// values with expected frames.
	SSIM
)

// TestCaseBitmask defines which performance test cases to run: capped, uncapped
// single, or uncapped with several concurrent decoder instances in parallel.
type TestCaseBitmask int

const (
	// CappedFlag identifies the case when a single decoder instance decodes a
	// video sequence from start to finish at its actual frame rate. Rendering is
	// simulated and late frames are dropped.
	CappedFlag TestCaseBitmask = 1 << iota
	// UncappedFlag identifies the case when a single decoder instance decodes a
	// video sequence as fast as possible. This provides an estimate of the
	// decoder's max performance (e.g. the maximum FPS).
	UncappedFlag
	// UncappedConcurrentFlag identifies the case when the specified test video is
	// decoded by multiple concurrent decoders as fast as possible.
	UncappedConcurrentFlag
)

// TestParams allows adjusting some of the test arguments passed in.
type TestParams struct {
	DecoderType            DecoderType
	LinearOutput           bool
	DisableGlobalVaapiLock bool
	TestCases              TestCaseBitmask
}

func generateCmdArgs(outDir, filename string, parameters TestParams) []string {
	args := []string{
		filename,
		filename + ".json",
		"--output_folder=" + outDir,
	}
	if parameters.DecoderType == VDVDA {
		args = append(args, "--use_vd_vda")
	} else if parameters.DecoderType == VDA {
		args = append(args, "--use-legacy")
	}
	if parameters.LinearOutput {
		args = append(args, "--linear_output")
	}
	if parameters.DisableGlobalVaapiLock {
		args = append(args, "--disable_vaapi_lock")
	}
	return args
}

// runAccelVideoTestCmd runs execCmd with args and also applying filter.
// Returns the values returned by gtest.Run() as-is.
func runAccelVideoTestCmd(ctx context.Context, execCmd, filter, logfilepath string, args []string) (*gtest.Report, error) {
	if report, err := gtest.New(
		filepath.Join(chrome.BinTestDir, execCmd),
		gtest.Logfile(logfilepath),
		gtest.ExtraArgs(args...),
		gtest.Filter(filter),
		gtest.UID(int(sysutil.ChronosUID)),
	).Run(ctx); err != nil {
		return report, err
	}
	return nil, nil
}

// RunAccelVideoTest runs video_decode_accelerator_tests with the specified
// video file. TestParams specifies:
// 1. Whether to run the tests against the VDA or VD based video decoder
//    implementations.
// 2. If the output of the decoder is a linear buffer (this is false by
//    default).
// 3. If the global VA-API lock should be disabled.
func RunAccelVideoTest(ctx context.Context, outDir, filename string, parameters TestParams) error {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		return errors.Wrap(err, "failed to set values for verbose logging")
	}
	defer vl.Close()

	// Reserve time to restart the ui job at the end of the test.
	// Only a single process can have access to the GPU, so we are required
	// to call "stop ui" at the start of the test. This will shut down the
	// chrome process and allow us to claim ownership of the GPU.
	shortCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	args := generateCmdArgs(outDir, filename, parameters)
	args = append(args, logging.ChromeVmoduleFlag())

	const exec = "video_decode_accelerator_tests"
	if report, err := runAccelVideoTestCmd(shortCtx,
		exec, "", filepath.Join(outDir, exec+".log"), args); err != nil {
		msg := fmt.Sprintf("failed to run %v with video %s", exec, filename)
		if report != nil {
			for _, name := range report.FailedTestNames() {
				msg = fmt.Sprintf("%s, %s: failed", msg, name)
			}
		}
		return errors.Wrap(err, msg)
	}
	return nil
}

// RunAccelVideoTestWithTestVectors runs
// video_decode_accelerator_tests --gtest_filter=VideoDecoderTest.FlushAtEndOfStream
// --validator_type=validatorType with the specified video files using the
// direct VideoDecoder. It expects such execution to succeed unless mustFail is
// set.
func RunAccelVideoTestWithTestVectors(ctx context.Context, outDir string, testVectors []string, validatorType ValidatorType, mustFail bool) error {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		return errors.Wrap(err, "failed to set values for verbose logging")
	}
	defer vl.Close()

	// Reserve time to restart the ui job at the end of the test.
	// Only a single process can have access to the GPU, so we are required
	// to call "stop ui" at the start of the test. This will shut down the
	// chrome process and allow us to claim ownership of the GPU.
	shortCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	const exec = "video_decode_accelerator_tests"
	var filenamesToReport []string
	for _, file := range testVectors {
		args := generateCmdArgs(outDir, file, TestParams{DecoderType: VD, LinearOutput: false})
		args = append(args, logging.ChromeVmoduleFlag())
		if validatorType == SSIM {
			args = append(args, "--validator_type=ssim")
		} else if validatorType == MD5 {
			args = append(args, "--validator_type=md5")
		}
		filename := filepath.Base(file)

		hasFailed := false
		if _, err = runAccelVideoTestCmd(shortCtx,
			exec, "VideoDecoderTest.FlushAtEndOfStream",
			filepath.Join(outDir, exec+"_"+filename+".log"), args); err != nil {
			hasFailed = true

			if mustFail {
				testing.ContextLog(ctx, "Test vector failed (expected): ", filename)
			} else {
				testing.ContextLog(ctx, "Test vector failed (unexpected): ", filename)
			}
		} else {
			if mustFail {
				testing.ContextLog(ctx, "Test vector passed (unexpected): ", filename)
			} else {
				testing.ContextLog(ctx, "Test vector passed (expected): ", filename)
			}
		}

		if hasFailed != mustFail {
			filenamesToReport = append(filenamesToReport, filename)
		}
	}

	if filenamesToReport != nil {
		return errors.Errorf("failed to validate: %v", filenamesToReport)
	}
	return nil
}

// RunAccelVideoPerfTest runs video_decode_accelerator_perf_tests with the
// specified video file. TestParams specifies:
// 1. Which Chrome stack implementation to run against (e.g. VDA, VD, etc.)
// 2. Whether the output of the decoder should be a linear buffer (false by
//    default), or the platform natural storage.
// 3. If the global VA-API lock should be disabled (false by default).
// 4. Which test cases to run, e.g. capped, uncapped, both etc.
// The test binary is run twice. The first time the test is run in isolation,
// creating its own output JSON file. The second time it's run cyclically to
// measure system wide metrics (e.g. CPU usage, power consumption).
func RunAccelVideoPerfTest(ctx context.Context, outDir, filename string, parameters TestParams) error {
	const (
		// Binary name.
		exec = "video_decode_accelerator_perf_tests"
		// Duration of the interval during which CPU usage will be measured.
		measureDuration = 20 * time.Second
		// Time reserved for cleanup.
		cleanupTime = 10 * time.Second
	)

	// Setup benchmark mode.
	cleanUpBenchmark, err := mediacpu.SetUpBenchmark(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to set up benchmark mode")
	}
	defer cleanUpBenchmark(ctx)

	// Reserve time to restart the ui job and perform cleanup at the end of the test.
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return errors.Wrap(err, "failed waiting for CPU to become idle")
	}

	var tests = []struct {
		testCase     TestCaseBitmask
		gTestName    string
		metricPrefix string
		parseFunc    func(string, *perf.Values, string) error
	}{
		{CappedFlag, "MeasureCappedPerformance", "capped_single_decoder", parseCappedPerfMetrics},
		{UncappedFlag, "MeasureUncappedPerformance", "uncapped_single_decoder", parseUncappedPerfMetrics},
		{UncappedConcurrentFlag, "MeasureUncappedPerformance_TenConcurrentDecoders", "uncapped_ten_concurrent_decoders",
			// TODO(b/211783279) Replace this parser with one that can handle multiple captures.
			parseUncappedPerfMetrics},
	}

	p := perf.NewValues()
	for _, test := range tests {
		if parameters.TestCases&test.testCase == 0 {
			continue
		}
		testing.ContextLogf(ctx, "Running %s", test.gTestName)

		args := generateCmdArgs(outDir, filename, parameters)
		if report, err := runAccelVideoTestCmd(ctx, exec,
			fmt.Sprintf("*%s", test.gTestName),
			filepath.Join(outDir, exec+"."+test.gTestName+".log"), args); err != nil {
			msg := fmt.Sprintf("failed to run %v with video %s", exec, filename)
			if report != nil {
				for _, name := range report.FailedTestNames() {
					msg = fmt.Sprintf("%s, %s: failed", msg, name)
				}
			}
			return errors.Wrap(err, msg)
		}
		json := filepath.Join(outDir, "VideoDecoderTest", test.gTestName+".json")
		if _, err := os.Stat(json); os.IsNotExist(err) {
			return errors.Wrap(err, "failed to find performance metrics file")
		}
		if err := test.parseFunc(json, p, test.metricPrefix); err != nil {
			return errors.Wrap(err, "failed to parse performance metrics")
		}

		// Run the same test case on repeat for a while and collect CPU and power
		// usage.
		measurements, err := mediacpu.MeasureProcessUsage(ctx, measureDuration, mediacpu.KillProcess, gtest.New(
			filepath.Join(chrome.BinTestDir, exec),
			gtest.Logfile(filepath.Join(outDir, exec+"."+test.gTestName+".onrepeat.log")),
			gtest.Filter("*"+test.gTestName),
			// Repeat enough times to run for full measurement duration. We don't
			// use -1 here as this can result in huge log files (b/138822793).
			gtest.Repeat(1000),
			gtest.ExtraArgs(args...),
			gtest.UID(int(sysutil.ChronosUID)),
		))
		if err != nil {
			return errors.Wrapf(err, "failed to measure CPU/Power usage %v", exec)
		}
		p.Set(perf.Metric{
			Name:      test.metricPrefix + ".cpu_usage",
			Unit:      "percent",
			Direction: perf.SmallerIsBetter,
		}, measurements["cpu"])

		// Power measurements are not supported on all platforms.
		if power, ok := measurements["power"]; ok {
			p.Set(perf.Metric{
				Name:      test.metricPrefix + ".power_consumption",
				Unit:      "watt",
				Direction: perf.SmallerIsBetter,
			}, power)
		}
	}

	if err := p.Save(outDir); err != nil {
		return errors.Wrap(err, "failed to save performance metrics")
	}
	return nil
}
