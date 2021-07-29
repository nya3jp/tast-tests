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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/media/encoding"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// Duration of the interval during which CPU usage will be measured in the performance test.
const measureInterval = 20 * time.Second

// TestOptions is the options for runAccelVideoTest.
type TestOptions struct {
	webMName string
	profile  videotype.CodecProfile

	// The number of spatial layers of the produced bitstream.
	// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes* about spatial layers.
	spatialLayers int

	// The number of temporal layers of the produced bitstream.
	// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes* about temporal layers.
	temporalLayers int

	// NV12 test cases runs (e.g. FlushAtEndOfStream_NV12DmabufScaling) if and only if this is true.
	// TODO(hiroh): Remove this and run both I420 and NV12 test cases in the regular video.EncodeAccel tests
	// once the dashboard is green.
	verifyNV12Input bool

	// Encode bitrate.
	bitrate int
}

// MakeTestOptions creates TestOptions from webMName and profile.
// spatialLayers and temporalLayers are set to 1 and verifyNV12Input is set to false.
func MakeTestOptions(webMName string, profile videotype.CodecProfile) TestOptions {
	return TestOptions{
		webMName:        webMName,
		profile:         profile,
		spatialLayers:   1,
		temporalLayers:  1,
		verifyNV12Input: false,
	}
}

// MakeBitrateTestOptions creates TestOptions from webMName and codec.
// spatialLayers and temporalLayers are set to 1 and verifyNV12Input is set to false.
// Sets bitrate for testing quality changes.
func MakeBitrateTestOptions(webMName string, profile videotype.CodecProfile, bitrate int) TestOptions {
	return TestOptions{
		webMName:        webMName,
		profile:         profile,
		spatialLayers:   1,
		temporalLayers:  1,
		verifyNV12Input: false,
		bitrate:         bitrate,
	}
}

// MakeNV12TestOptions creates TestOptions from webMName and profile.
// spatialLayers and temporalLayers are set to 1 and verifyNV12Input is set to true.
func MakeNV12TestOptions(webMName string, profile videotype.CodecProfile) TestOptions {
	return TestOptions{
		webMName:        webMName,
		profile:         profile,
		spatialLayers:   1,
		temporalLayers:  1,
		verifyNV12Input: true,
	}
}

// MakeTestOptionsWithSVCLayers creates TestOptions from webMName, profile, spatialLayers and temporalLayers.
// verifyNV12Input is set to false.
func MakeTestOptionsWithSVCLayers(webMName string, profile videotype.CodecProfile, spatialLayers, temporalLayers int) TestOptions {
	return TestOptions{
		webMName:        webMName,
		profile:         profile,
		spatialLayers:   spatialLayers,
		temporalLayers:  temporalLayers,
		verifyNV12Input: true,
	}
}

// TestData returns the files used in video.EncodeAccel(Perf), the webm file and the json file returned by encode.YUVJSONFileNameFor().
func TestData(webmFileName string) []string {
	return []string{webmFileName, YUVJSONFileNameFor(webmFileName)}
}

// YUVJSONFileNameFor returns the json file name used in video.EncodeAccel with |webmMFileName|.
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
	case videotype.H264BaselineProf:
		return "h264baseline", nil
	case videotype.H264MainProf:
		return "h264main", nil
	case videotype.H264HighProf:
		return "h264high", nil
	case videotype.VP8Prof:
		return "vp8", nil
	case videotype.VP9Prof:
		return "vp9", nil
	default:
		return "", errors.Errorf("unknown codec profile: %v", profile)
	}
}

// RunAccelVideoTest runs all tests in video_encode_accelerator_tests.
func RunAccelVideoTest(ctxForDefer context.Context, s *testing.State, opts TestOptions) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	ctx, cancel := ctxutil.Shorten(ctxForDefer, 10*time.Second)
	defer cancel()

	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Error("Failed to stop ui: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	yuvPath, err := encoding.PrepareYUV(ctx, s.DataPath(opts.webMName),
		videotype.I420, coords.NewSize(0, 0) /* placeholder size */)
	if err != nil {
		s.Fatal("Failed to create a yuv file: ", err)
	}
	defer os.Remove(yuvPath)

	yuvJSONPath, err := encoding.PrepareYUVJSON(ctx, yuvPath,
		s.DataPath(YUVJSONFileNameFor(opts.webMName)))
	if err != nil {
		s.Fatal("Failed to create a yuv json file: ", err)
	}
	defer os.Remove(yuvJSONPath)

	codec, err := codecProfileToEncodeCodecOption(opts.profile)
	if err != nil {
		s.Fatal("Failed to get codec option: ", err)
	}
	testArgs := []string{logging.ChromeVmoduleFlag(),
		fmt.Sprintf("--codec=%s", codec),
		yuvPath,
		yuvJSONPath,
	}

	if opts.spatialLayers > 1 {
		testArgs = append(testArgs, fmt.Sprintf("--num_spatial_layers=%d", opts.spatialLayers))
	}
	if opts.temporalLayers > 1 {
		testArgs = append(testArgs, fmt.Sprintf("--num_temporal_layers=%d", opts.temporalLayers))
	}

	gtestFilter := gtest.Filter("-*NV12Dmabuf*")
	if opts.verifyNV12Input {
		gtestFilter = gtest.Filter("*NV12Dmabuf*")
	}

	exec := filepath.Join(chrome.BinTestDir, "video_encode_accelerator_tests")
	logfile := filepath.Join(s.OutDir(), fmt.Sprintf("output_%s_%d.txt", filepath.Base(exec), time.Now().Unix()))
	t := gtest.New(exec, gtest.Logfile(logfile),
		gtestFilter,
		gtest.ExtraArgs(testArgs...),
		gtest.UID(int(sysutil.ChronosUID)))

	command, _ := t.Args()
	if command != nil {
		testing.ContextLogf(ctx, "Running %s", shutil.EscapeSlice(command))
	}

	if report, err := t.Run(ctx); err != nil {
		if report != nil {
			for _, name := range report.FailedTestNames() {
				s.Error(name, " failed")
			}
		}
		s.Fatalf("Failed to run %v: %v", exec, err)
	}
}

// RunAccelVideoPerfTest runs video_encode_accelerator_perf_tests with the specified
// video file.
// - Uncapped performance: the specified test video is encoded for 300 frames by the hardware encoder as fast as possible.
// This provides an estimate of the decoder's max performance (e.g. the maximum FPS).
// - Capped performance: the specified test video is encoded for 300 frames by the hardware encoder at 30fps.
// This is used to measure cpu usage and power consumption in the practical case.
// - Quality performance: the specified test video is encoded for 300 frames and computes the SSIM and PSNR metrics of the encoded stream.
func RunAccelVideoPerfTest(ctxForDefer context.Context, s *testing.State, opts TestOptions) error {
	const (
		// Name of the uncapped performance test.
		uncappedTestname = "MeasureUncappedPerformance"
		// Name of the capped performance test.
		cappedTestname = "MeasureCappedPerformance"
		// Name of the bitstream quality test.
		qualityTestname = "MeasureProducedBitstreamQuality"
		// The binary performance test.
		exec = "video_encode_accelerator_perf_tests"
	)

	ctx, cancel := ctxutil.Shorten(ctxForDefer, 10*time.Second)
	defer cancel()
	// Setup benchmark mode.
	cleanUpBenchmark, err := cpu.SetUpBenchmark(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to set up benchmark mode")
	}
	defer cleanUpBenchmark(ctx)

	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Error("Failed to stop ui: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	yuvPath, err := encoding.PrepareYUV(ctx, s.DataPath(opts.webMName),
		videotype.I420, coords.NewSize(0, 0) /* placeholder size */)
	if err != nil {
		s.Fatal("Failed to create a yuv file: ", err)
	}
	defer os.Remove(yuvPath)

	yuvJSONPath, err := encoding.PrepareYUVJSON(ctx, yuvPath,
		s.DataPath(YUVJSONFileNameFor(opts.webMName)))
	if err != nil {
		s.Fatal("Failed to create a yuv json file: ", err)
	}

	defer os.Remove(yuvJSONPath)

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for CPU to become idle")
	}

	codec, err := codecProfileToEncodeCodecOption(opts.profile)
	if err != nil {
		return errors.Wrap(err, "failed to get codec option")
	}

	// Test 1: Measure maximum FPS and the quality of the encoded bitstream.
	testArgs := []string{
		fmt.Sprintf("--codec=%s", codec),
		fmt.Sprintf("--output_folder=%s", s.OutDir()),
		yuvPath,
		yuvJSONPath,
	}

	if opts.spatialLayers > 1 {
		testArgs = append(testArgs, fmt.Sprintf("--num_spatial_layers=%d", opts.spatialLayers))
	}
	if opts.temporalLayers > 1 {
		testArgs = append(testArgs, fmt.Sprintf("--num_temporal_layers=%d", opts.temporalLayers))
	}
	if opts.bitrate > 0 {
		testArgs = append(testArgs, fmt.Sprintf("--bitrate=%d", opts.bitrate))
	}

	if report, err := gtest.New(
		filepath.Join(chrome.BinTestDir, exec),
		gtest.Logfile(filepath.Join(s.OutDir(), exec+".uncap_and_quality.log")),
		gtest.Filter(fmt.Sprintf("*%s:*%s", uncappedTestname, qualityTestname)),
		gtest.ExtraArgs(testArgs...),
		gtest.UID(int(sysutil.ChronosUID)),
	).Run(ctx); err != nil {
		if report != nil {
			for _, name := range report.FailedTestNames() {
				s.Error(name, " failed")
			}
		}
		return errors.Wrapf(err, "failed to run %v", exec)
	}

	p := perf.NewValues()
	uncappedJSON := filepath.Join(s.OutDir(), "VideoEncoderTest", uncappedTestname+".json")
	if _, err := os.Stat(uncappedJSON); os.IsNotExist(err) {
		return errors.Wrap(err, "failed to find uncapped performance metrics file")
	}
	if err := encoding.ParseUncappedPerfMetrics(uncappedJSON, p); err != nil {
		return errors.Wrap(err, "failed to parse uncapped performance metrics")
	}

	qualityJSONPath := filepath.Join(s.OutDir(), "VideoEncoderTest", qualityTestname)
	if opts.spatialLayers > 1 || opts.temporalLayers > 1 {
		for sid := 1; sid <= opts.spatialLayers; sid++ {
			for tid := 1; tid <= opts.temporalLayers; tid++ {
				scalabilityMode := fmt.Sprintf("L%dT%d", sid, tid)
				if err := addQualityMetrics(qualityJSONPath, scalabilityMode, p); err != nil {
					return err
				}
			}
		}
	} else {
		if err := addQualityMetrics(qualityJSONPath, "", p); err != nil {
			return err
		}
	}

	// Test 2: Measure CPU usage and power consumption while running capped
	// performance test only.
	measurements, err := cpu.MeasureProcessUsage(ctx, measureInterval, cpu.KillProcess, gtest.New(
		filepath.Join(chrome.BinTestDir, exec),
		gtest.Logfile(filepath.Join(s.OutDir(), exec+".cap.log")),
		gtest.Filter("*"+cappedTestname),
		// Repeat enough times to run for full measurement duration. We don't
		// use -1 here as this can result in huge log files (b/138822793).
		gtest.Repeat(1000),
		gtest.ExtraArgs(testArgs...),
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

	if err := p.Save(s.OutDir()); err != nil {
		return errors.Wrap(err, "failed to save performance metrics")
	}

	return nil
}

// addQualityMetrics reads quality metrics from JSON file for scalabilityMode and add them to p.
// Empty scalabilityMode represents the JSON contains the metrics of non SVC encoding.
func addQualityMetrics(qualityJSONPath, scalabilityMode string, p *perf.Values) error {
	qualityJSON := qualityJSONPath
	if scalabilityMode != "" {
		qualityJSON += "." + scalabilityMode
	}
	qualityJSON += ".json"
	if _, err := os.Stat(qualityJSON); os.IsNotExist(err) {
		return errors.Wrap(err, "failed to find quality performance metrics file")
	}
	if err := encoding.ParseQualityPerfMetrics(qualityJSON, scalabilityMode, p); err != nil {
		return errors.Wrap(err, "failed to parse quality performance metrics file")
	}
	return nil
}
