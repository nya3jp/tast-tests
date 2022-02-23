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
	"chromiumos/tast/local/upstart"
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

// TestParams allows adjusting some of the test arguments passed in.
type TestParams struct {
	DecoderType            DecoderType
	LinearOutput           bool
	DisableGlobalVaapiLock bool
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
	upstart.StopJob(shortCtx, "ui")
	defer upstart.EnsureJobRunning(ctx, "ui")

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

// RunAccelVideoTestWithTestVectors runs video_decode_accelerator_tests --gtest_filter=VideoDecoderTest.FlushAtEndOfStream
// --validator_type=validatorType with the specified video files using the direct VideoDecoder.
func RunAccelVideoTestWithTestVectors(ctx context.Context, outDir string, testVectors []string, validatorType ValidatorType) error {
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
	upstart.StopJob(shortCtx, "ui")
	defer upstart.EnsureJobRunning(ctx, "ui")
	const exec = "video_decode_accelerator_tests"
	var failedFilenames []string
	for _, file := range testVectors {
		args := generateCmdArgs(outDir, file, TestParams{DecoderType: VD, LinearOutput: false})
		args = append(args, logging.ChromeVmoduleFlag())
		if validatorType == SSIM {
			args = append(args, "--validator_type=ssim")
		} else if validatorType == MD5 {
			args = append(args, "--validator_type=md5")
		}
		filename := filepath.Base(file)
		if _, err = runAccelVideoTestCmd(shortCtx,
			exec, "VideoDecoderTest.FlushAtEndOfStream",
			filepath.Join(outDir, exec+"_"+filename+".log"), args); err != nil {
			failedFilenames = append(failedFilenames, filename)
			testing.ContextLog(ctx, "Test vector failed: ", filename)
		} else {
			testing.ContextLog(ctx, "Test vector passed: ", filename)
		}
	}
	if failedFilenames != nil {
		return errors.Errorf("failed to validate the decoding of %v", failedFilenames)
	}
	return nil
}

// RunAccelVideoPerfTest runs video_decode_accelerator_perf_tests with the
// specified video file. TestParams specifies:
// 1. Whether to run the tests against the VDA or VD based video decoder
//    implementations.
// 2. If the output of the decoder is a linear buffer (this is false by
//    default).
// 3. If the global VA-API lock should be disabled.
// Both capped and uncapped performance is measured.
// - Uncapped performance: the specified test video is decoded from start to
// finish as fast as possible. This provides an estimate of the decoder's max
// performance (e.g. the maximum FPS).
// - Capped decoder performance: uses a more realistic environment by decoding
// the test video from start to finish at its actual frame rate. Rendering is
// simulated and late frames are dropped.
// The test binary is run twice. Once to measure both capped and uncapped
// performance, once to measure CPU usage and power consumption while running
// the capped performance test.
// Multiple concurrent performance: the specified test video is decoded with multiple concurrent decoders as fast as possible.
func RunAccelVideoPerfTest(ctx context.Context, outDir, filename string, parameters TestParams) error {
	const (
		// Name of the capped performance test.
		cappedTestname = "MeasureCappedPerformance"
		// Name of the uncapped performance test.
		uncappedTestname = "MeasureUncappedPerformance"
		// Name of the uncapped, multiple concurrent test.
		multipleConcurrentTestname = "MeasureUncappedPerformance_TenConcurrentDecoders"
		// Duration of the interval during which CPU usage will be measured.
		measureDuration = 20 * time.Second
		// Time reserved for cleanup.
		cleanupTime = 10 * time.Second
	)

	// Only a single process can have access to the GPU, so we are required to
	// call "stop ui" at the start of the test. This will shut down the chrome
	// process and allow us to claim ownership of the GPU.
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		return errors.Wrap(err, "failed to stop ui")
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

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

	// Test 1: Measure capped and uncapped performance.
	args := generateCmdArgs(outDir, filename, parameters)

	const exec = "video_decode_accelerator_perf_tests"
	if report, err := runAccelVideoTestCmd(ctx, exec,
		fmt.Sprintf("*%s:*%s:*%s", cappedTestname, uncappedTestname, multipleConcurrentTestname),
		filepath.Join(outDir, exec+".1.log"), args); err != nil {
		msg := fmt.Sprintf("failed to run %v with video %s", exec, filename)
		if report != nil {
			for _, name := range report.FailedTestNames() {
				msg = fmt.Sprintf("%s, %s: failed", msg, name)
			}
		}
		return errors.Wrap(err, msg)
	}

	p := perf.NewValues()
	uncappedJSON := filepath.Join(outDir, "VideoDecoderTest", uncappedTestname+".json")
	if _, err := os.Stat(uncappedJSON); os.IsNotExist(err) {
		return errors.Wrap(err, "failed to find uncapped performance metrics file")
	}

	cappedJSON := filepath.Join(outDir, "VideoDecoderTest", cappedTestname+".json")
	if _, err := os.Stat(cappedJSON); os.IsNotExist(err) {
		return errors.Wrap(err, "failed to find capped performance metrics file")
	}

	multipleConcurrentJSON := filepath.Join(outDir, "VideoDecoderTest", multipleConcurrentTestname+".json")
	if _, err := os.Stat(multipleConcurrentJSON); os.IsNotExist(err) {
		return errors.Wrap(err, "failed to find ten concurrent decoders performance metrics file")
	}

	if err := parseUncappedPerfMetrics(uncappedJSON, p, "single_decoder"); err != nil {
		return errors.Wrap(err, "failed to parse uncapped performance metrics")
	}
	if err := parseCappedPerfMetrics(cappedJSON, p); err != nil {
		return errors.Wrap(err, "failed to parse capped performance metrics")
	}
	// TODO(b/211783279) Replace this parser with one that can handle multiple captures.
	// Use the uncapped parser since only one performance evaluator is current being used in the test.
	if err := parseUncappedPerfMetrics(multipleConcurrentJSON, p, "ten_concurrent_decoders"); err != nil {
		return errors.Wrap(err, "failed to parse multiple concurrent decoders performance metrics")
	}
	// Test 2: Measure CPU usage and power consumption while running capped
	// performance test only.
	// TODO(dstaessens) Investigate collecting CPU usage during previous test.
	measurements, err := mediacpu.MeasureProcessUsage(ctx, measureDuration, mediacpu.KillProcess, gtest.New(
		filepath.Join(chrome.BinTestDir, exec),
		gtest.Logfile(filepath.Join(outDir, exec+".2.log")),
		gtest.Filter("*"+cappedTestname),
		// Repeat enough times to run for full measurement duration. We don't
		// use -1 here as this can result in huge log files (b/138822793).
		gtest.Repeat(1000),
		gtest.ExtraArgs(args...),
		gtest.UID(int(sysutil.ChronosUID)),
	))
	if err != nil {
		return errors.Wrapf(err, "failed to measure CPU usage %v", exec)
	}

	p.Set(perf.Metric{
		Name:      "cpu_usage",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, measurements["cpu"])

	// Power measurements are not supported on all platforms.
	if power, ok := measurements["power"]; ok {
		p.Set(perf.Metric{
			Name:      "power_consumption",
			Unit:      "watt",
			Direction: perf.SmallerIsBetter,
		}, power)
	}

	if err := p.Save(outDir); err != nil {
		return errors.Wrap(err, "failed to save performance metrics")
	}
	return nil
}
