// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package encode provides common code to run Chrome binary tests for video encoding.
package encode

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/media/encoding"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// TestOptionsNew is the options for runNewAccelVideoTest.
type TestOptionsNew struct {
	WebMName string
	Profile  videotype.CodecProfile
}

// TestData returns the files used in video.EncodeAccelNew(Perf), the webm file and the json file returned by encode.YUVJSONFileNameFor().
func TestData(webmFileName string) []string {
	return []string{webmFileName, YUVJSONFileNameFor(webmFileName)}
}

// YUVJSONFileNameFor returns the json file name used in video.EncodeAccelNew with |webmMFileName|.
// For example, if |webMFileName| is bear-320x192.vp9.webm, then bear-320x192.yuv.json is returned.
func YUVJSONFileNameFor(webMFileName string) string {
	const webMSuffix = ".vp9.webm"
	if !strings.HasSuffix(webMFileName, webMSuffix) {
		return "error.json"
	}
	yuvName := strings.TrimSuffix(webMFileName, webMSuffix) + ".yuv"
	return yuvName + ".json"
}

func codecProfileToEncodeCodecOption(profile videotype.CodecProfile) (string, error) {
	switch profile {
	case videotype.H264Prof:
		return "h264baseline", nil
	case videotype.VP8Prof:
		return "vp8", nil
	case videotype.VP9Prof:
		return "vp9", nil
	default:
		return "", errors.Errorf("unknown codec profile: %v", profile)
	}
}

func prepareExecuteTest(ctx context.Context, profile videotype.CodecProfile) (string, error) {
	codec, err := codecProfileToEncodeCodecOption(profile)
	if err != nil {
		return "", errors.Wrap(err, "failed to get codec option: ")
	}
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		return "", errors.Wrap(err, "failed to stop ui: ")
	}
	return codec, nil
}

func prepareYUVFiles(ctx context.Context, s *testing.State, webMName string) (string, string, error) {
	yuvPath, err := encoding.PrepareYUV(ctx, s.DataPath(webMName), videotype.I420, coords.NewSize(0, 0) /* placeholder size */)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to prepare YUVFile")
	}

	yuvJSONFileName := YUVJSONFileNameFor(webMName)
	yuvJSONPath := yuvPath + ".json"
	if err := fsutil.CopyFile(s.DataPath(yuvJSONFileName), yuvJSONPath); err != nil {
		os.Remove(yuvPath)
		return "", "", errors.Wrapf(err, "failed to copy json file: %v %v", s.DataPath(yuvJSONFileName), yuvJSONPath)

	}
	return yuvPath, yuvJSONPath, nil
}

// RunNewAccelVideoTest runs all tests in video_encode_accelerator_tests.
func RunNewAccelVideoTest(ctxForDefer context.Context, s *testing.State, opts TestOptionsNew) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	ctx, cancel := ctxutil.Shorten(ctxForDefer, 10*time.Second)
	defer cancel()

	codec, err := prepareExecuteTest(ctx, opts.Profile)
	if err != nil {
		s.Fatal("Failed in preparing the test: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	yuvPath, yuvJSONPath, err := prepareYUVFiles(ctx, s, opts.WebMName)
	if err != nil {
		s.Fatal("Failed to prepare YUV files: ", err)
	}
	defer os.Remove(yuvPath)
	defer os.Remove(yuvJSONPath)

	testArgs := []string{logging.ChromeVmoduleFlag(),
		fmt.Sprintf("--codec=%s", codec),
		yuvPath,
		yuvJSONPath,
	}

	exec := filepath.Join(chrome.BinTestDir, "video_encode_accelerator_tests")
	logfile := filepath.Join(s.OutDir(), fmt.Sprintf("output_%s_%d.txt", filepath.Base(exec), time.Now().Unix()))
	t := gtest.New(exec, gtest.Logfile(logfile),
		gtest.ExtraArgs(testArgs...),
		gtest.UID(int(sysutil.ChronosUID)))

	if report, err := t.Run(ctx); err != nil {
		if report != nil {
			for _, name := range report.FailedTestNames() {
				s.Error(name, " failed")
			}
		}
		s.Fatalf("Failed to run %v: %v", exec, err)
	}
}

// RunNewAccelVideoPerfTest runs video_encode_accelerator_perf_tests with the specified
// video file.
// - Uncapped performance: the specified test video is encoded for 300 frames.
// This provides an estimate of the decoder's max performance (e.g. the maximum FPS).
// - Quality performance:
func RunNewAccelVideoPerfTest(ctxForDefer context.Context, s *testing.State, opts TestOptionsNew) {
	const (
		// Name of the uncapped performance test.
		uncappedTestname = "MeasureUncappedPerformance"
		// Name of the capped performance test.
		cappedTestname = "MeasureCappedPerformance"
		// Name of the bitstream quality test.
		qualityTestname = "MeasureProducedBitstreamQuality"
		// Duration of the interval during which CPU usage will be measured.
		measureDuration = 20 * time.Second
		// The binary performance test
		exec = "video_encode_accelerator_perf_tests"
	)

	ctx, cancel := ctxutil.Shorten(ctxForDefer, 10*time.Second)
	defer cancel()
	// Setup benchmark mode.
	cleanUpBenchmark, err := cpu.SetUpBenchmark(ctx)
	if err != nil {
		s.Fatal("Failed to set up benchmark mode: ", err)
	}
	defer cleanUpBenchmark(ctx)

	codec, err := prepareExecuteTest(ctx, opts.Profile)
	if err != nil {
		s.Fatal("Failed in preparing the test: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	yuvPath, yuvJSONPath, err := prepareYUVFiles(ctx, s, opts.WebMName)
	if err != nil {
		s.Fatal("Failed to prepare YUV files: ", err)
	}
	defer os.Remove(yuvPath)
	defer os.Remove(yuvJSONPath)

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed waiting for CPU to become idle: ", err)
	}

	testArgs := []string{
		fmt.Sprintf("--codec=%s", codec),
		fmt.Sprintf("--output_folder=%s", s.OutDir()),
		yuvPath,
		yuvJSONPath,
	}
	if report, err := gtest.New(
		filepath.Join(chrome.BinTestDir, exec),
		gtest.Logfile(filepath.Join(s.OutDir(), exec+".1.log")),
		gtest.Filter(fmt.Sprintf("*%s:*%s", uncappedTestname, qualityTestname)),
		gtest.ExtraArgs(testArgs...),
		gtest.UID(int(sysutil.ChronosUID)),
	).Run(ctx); err != nil {
		if report != nil {
			for _, name := range report.FailedTestNames() {
				s.Error(name, " failed")
			}
		}
		s.Fatalf("Failed to run %v: %v", exec, err)
	}

	p := perf.NewValues()
	uncappedJSON := filepath.Join(s.OutDir(), "VideoEncoderTest", uncappedTestname+".json")
	if _, err := os.Stat(uncappedJSON); os.IsNotExist(err) {
		s.Fatal("Failed to find uncapped performance metrics file: ", err)
	}
	qualityJSON := filepath.Join(s.OutDir(), "VideoEncoderTest", qualityTestname+".json")
	if _, err := os.Stat(qualityJSON); os.IsNotExist(err) {
		s.Fatal("Failed to find quality performance metrics file: ", err)
	}

	if err := encoding.ParseUncappedPerfMetrics(uncappedJSON, p); err != nil {
		s.Fatal("Failed to parse uncapped performance metrics: ", err)
	}
	if err := encoding.ParseQualityPerfMetrics(qualityJSON, p); err != nil {
		s.Fatal("Failed to parse quality performance metrics: ", err)
	}

	// Test 2: Measure CPU usage and power consumption while running capped
	// performance test only.
	measurements, err := cpu.MeasureProcessUsage(ctx, measureDuration, cpu.KillProcess, gtest.New(
		filepath.Join(chrome.BinTestDir, exec),
		gtest.Logfile(filepath.Join(s.OutDir(), exec+".2.log")),
		gtest.Filter("*"+cappedTestname),
		// Repeat enough times to run for full measurement duration. We don't
		// use -1 here as this can result in huge log files (b/138822793).
		gtest.Repeat(1000),
		gtest.ExtraArgs(testArgs...),
		gtest.UID(int(sysutil.ChronosUID)),
	))
	if err != nil {
		s.Fatalf("Failed to measure CPU usage %v: %v", exec, err)
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

	if err := p.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save performance metrics: ", err)
	}
}
