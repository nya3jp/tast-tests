// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
//
// This file provides code for video.ARCDecodeAccel* tests.

package decode

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const pkg = "org.chromium.c2.test"
const arcFilePath = "/sdcard/Download/"
const apkName = "c2_e2e_test.apk"
const activityName = "E2eTestActivity"

const perfMeasurementDurationSec = 30

// PerfTestRuntimeSec is the runtime for a single performance test case
const PerfTestRuntimeSec = (perfMeasurementDurationSec * 2) + 60

// decodeMetadata stores parsed metadata from test video JSON files, which are external files located in
// gs://chromiumos-test-assets-public/tast/cros/video/, e.g. test-25fps.h264.json.
type decodeMetadata struct {
	Profile            string   `json:"profile"`
	Width              int      `json:"width"`
	Height             int      `json:"height"`
	FrameRate          int      `json:"frame_rate"`
	NumFrames          int      `json:"num_frames"`
	NumFragments       int      `json:"num_fragments"`
	MD5Checksums       []string `json:"md5_checksums"`
	ThumbnailChecksums []string `json:"thumbnail_checksums"`
}

// toStreamDataArg returns a string that can be used for an argument to the c2_e2e_test APK.
// dataPath is the absolute path of the video file.
func (d *decodeMetadata) toStreamDataArg(dataPath string) (string, error) {
	pEnum, found := videoCodecEnumValues[d.Profile]
	if !found {
		return "", errors.Errorf("cannot find enum value for profile %v", d.Profile)
	}

	// Set MinFPSNoRender and MinFPSWithRender to 0 for disabling FPS check because we would like
	// TestFPS to be always passed and store FPS value into perf metric.
	sdArg := fmt.Sprintf("--test_video_data=%s:%d:%d:%d:%d:0:0:%d:%d",
		dataPath, d.Width, d.Height, d.NumFrames, d.NumFragments, pEnum, d.FrameRate)
	return sdArg, nil
}

// arcTestConfig stores test configuration to run c2_e2e_test APK.
type arcTestConfig struct {
	// testVideo stores the test video's name.
	testVideo string
	// requireMD5File indicates whether to prepare MD5 file for test.
	requireMD5File bool
	// testFilter specifies test pattern the test can run.
	// If unspecified, c2_e2e_test runs all tests.
	testFilter string
	// measurePerf indicates whether perf results should be measured.
	measurePerf bool
	// logPrefix
	logPrefix string
}

// toArgsList converts arcTestConfig to a list of argument strings.
// md is the decodeMetadata parsed from JSON file.
func (t *arcTestConfig) toArgsList(md decodeMetadata) (string, error) {
	// decoder test only.
	dataPath := filepath.Join(arcFilePath, t.testVideo)
	sdArg, err := md.toStreamDataArg(dataPath)
	if err != nil {
		return "", err
	}
	return sdArg, nil
}

// writeLinesToFile writes lines to filepath line by line.
func writeLinesToFile(lines []string, filepath string) error {
	return ioutil.WriteFile(filepath, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

// videoCodecEnumValues maps profile string to its enum value.
// These values must match integers in VideoCodecProfile in https://cs.chromium.org/chromium/src/media/base/video_codecs.h
var videoCodecEnumValues = map[string]int{
	"H264PROFILE_MAIN":    1,
	"VP8PROFILE_ANY":      11,
	"VP9PROFILE_PROFILE0": 12,
}

func waitForFinish(ctx context.Context, s *testing.State) error {
	a := s.PreValue().(arc.PreData).ARC

	s.Log("Waiting for activity to finish")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		tasks, err := a.DumpsysActivityActivities(ctx)
		if err != nil {
			return testing.PollBreak(err)
		}
		for _, task := range tasks {
			if task.PkgName == pkg {
				return errors.New("match")
			}
		}
		return nil
	}, nil); err != nil {
		s.Fatal("Failed to wait for activity: ", err)
	}
	return nil
}

func runARCVideoTestSetup(ctx context.Context, s *testing.State, testVideo string, requireMD5File bool) decodeMetadata {
	shortCtx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	a := s.PreValue().(arc.PreData).ARC

	videoPath := s.DataPath(testVideo)
	pushFiles := []string{videoPath}

	// Parse JSON metadata.
	// TODO(johnylin) Adapt ARC decoder test to use the json file directly.
	jf, err := os.Open(videoPath + ".json")
	if err != nil {
		s.Fatal("Failed to open JSON file: ", err)
	}
	defer jf.Close()

	s.Log("Installing APK ", apkName)
	if err := a.Install(ctx, s.DataPath(apkName)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	s.Log("Granting storage permissionss")
	permissions := [2]string{
		"android.permission.READ_EXTERNAL_STORAGE",
		"android.permission.WRITE_EXTERNAL_STORAGE"}
	for _, perm := range permissions {
		if err := a.Command(ctx, "pm", "grant", pkg, perm).Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed granting storage permission: ", err)
		}
	}

	var md decodeMetadata
	if err := json.NewDecoder(jf).Decode(&md); err != nil {
		s.Fatal("Failed to parse metadata from JSON file: ", err)
	}

	if requireMD5File {
		// Prepare frames MD5 file.
		frameMD5Path := videoPath + ".frames.md5"
		s.Logf("Preparing frames MD5 file %v from JSON metadata", frameMD5Path)
		if err := writeLinesToFile(md.MD5Checksums, frameMD5Path); err != nil {
			s.Fatalf("Failed to prepare frames MD5 file %s: %v", frameMD5Path, err)
		}
		defer os.Remove(frameMD5Path)

		pushFiles = append(pushFiles, frameMD5Path)
	}

	// Push files to ARC container.
	for _, pushFile := range pushFiles {
		err := a.PushFile(shortCtx, pushFile, arcFilePath)
		if err != nil {
			s.Fatal("Failed to push video stream to ARC: ", err)
		}
		defer a.Command(ctx, "rm", arcFilePath+pushFile).Run()
	}

	return md
}

// runARCVideoTest runs c2_e2e_test APK in ARC.
// It fails if c2_e2e_test fails.
// It returns a map of perf statistics. If cfg.measurePerf is false, the map
// is empty. Otherwise it contains fps, dropped frame, cpu, and power stats.
func runARCVideoTest(ctx context.Context, s *testing.State, md decodeMetadata, cfg arcTestConfig) (perf map[string]float64) {
	shortCtx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	a := s.PreValue().(arc.PreData).ARC
	args, err := cfg.toArgsList(md)
	if err != nil {
		s.Fatal("Failed to generate args list: ", err)
	}

	s.Log("Starting APK main activity")
	var logFileName string
	if cfg.logPrefix != "" {
		logFileName = cfg.logPrefix + "_gtest_logs.txt"
	} else {
		logFileName = "gtest_logs.txt"
	}
	defer a.Command(ctx, "rm", arcFilePath+logFileName).Run()
	if cfg.testFilter != "" {
		args = args + ",--gtest_filter=" + cfg.testFilter
	}
	if cfg.measurePerf {
		args = args + ",--loop"
	}
	if err := a.Command(ctx, "am", "start", "-W", "-n", pkg+"/."+activityName,
		"--esa", "test-args", args,
		"--es", "log-file", arcFilePath+logFileName).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed starting APK main activity: ", err)
	}

	var stats map[string]float64
	if cfg.measurePerf {
		const measureDelaySec = 5
		if err := testing.Sleep(ctx, time.Duration(measureDelaySec)*time.Second); err != nil {
			s.Fatal("Failed waiting for CPU usage to stabilize: ", err)
		}

		s.Log("Starting cpu measurement")
		var err error
		stats, err = cpu.MeasureUsage(ctx, time.Duration(perfMeasurementDurationSec)*time.Second)
		if err != nil {
			s.Fatal("Failed measuring CPU usage: ", err)
		}

		s.Log("Stopping target")
		if err := a.Command(ctx, "am", "start", "-W", "-n", pkg+"/."+activityName,
			"--activity-single-top").Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed stopping loop: ", err)
		}
	}

	waitForFinish(ctx, s)

	if err := a.PullFile(shortCtx, arcFilePath+logFileName, s.OutDir()); err != nil {
		s.Fatal("Failed to pull logs: ", err)
	}

	outLogFile := s.OutDir() + "/" + logFileName
	b, err := ioutil.ReadFile(outLogFile)
	if err != nil {
		s.Fatal("Failed to read log file: ", err)
	}

	regExpFPS := regexp.MustCompile(`\[  PASSED  \] \d+ test.`)
	matches := regExpFPS.FindAllStringSubmatch(string(b), -1)
	if len(matches) != 1 {
		s.Fatal("Did not find pass marker in log file", outLogFile)
	}

	perfMap := make(map[string]float64)
	if cfg.measurePerf {
		s.Logf("CPU Usage = %.4f", stats["cpu"])
		perfMap["cpu"] = stats["cpu"]

		if power, ok := stats["power"]; ok {
			s.Logf("Power Usage = %.4f", power)
			perfMap["power"] = stats["power"]
		}

		if fps, df, ok := reportFrameStats(s, outLogFile); ok == nil {
			s.Logf("FPS = %.2f", fps)
			s.Logf("Dropped frames = %d", df)
			perfMap["fps"] = fps
			perfMap["df"] = float64(df)
		} else {
			s.Error("Failed to report FPS: ", ok)
		}
	}
	return perfMap
}

// RunAllARCVideoTests runs all tests in c2_e2e_test.
func RunAllARCVideoTests(ctx context.Context, s *testing.State, testVideo string) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	md := runARCVideoTestSetup(ctx, s, testVideo, true)

	runARCVideoTest(ctx, s, md, arcTestConfig{
		testVideo:      testVideo,
		requireMD5File: true,
	})
}

// reportFrameStats reports FPS info from log file and sets as the perf metric.
func reportFrameStats(s *testing.State, logPath string) (float64, int64, error) {
	b, err := ioutil.ReadFile(logPath)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to read file")
	}

	regExpFPS := regexp.MustCompile(`(?m)^\[LOG\] Measured decoder FPS: ([+\-]?[0-9.]+)$`)
	matches := regExpFPS.FindAllStringSubmatch(string(b), -1)
	if len(matches) != 1 {
		return 0, 0, errors.Errorf("found %d FPS matches in %v; want 1", len(matches), filepath.Base(logPath))
	}

	fps, err := strconv.ParseFloat(matches[0][1], 64)
	if err != nil {
		return 0, 0, errors.Wrapf(err, "failed to parse FPS value %q", matches[0][1])
	}

	regExpDroppedFrames := regexp.MustCompile(`(?m)^\[LOG\] Dropped frames: ([+\-]?[0-9.]+)$`)
	matches = regExpDroppedFrames.FindAllStringSubmatch(string(b), -1)
	if len(matches) != 1 {
		return 0, 0, errors.Errorf("found %d dropped frames matches in %v; want 1", len(matches), filepath.Base(logPath))
	}

	df, err := strconv.ParseInt(matches[0][1], 10, 64)
	if err != nil {
		return 0, 0, errors.Wrapf(err, "failed to parse dropped frame value %q", matches[0][1])
	}

	return fps, df, nil
}

// RunARCVideoPerfTest runs testFPS in c2_e2e_test and sets as perf metric.
func RunARCVideoPerfTest(ctx context.Context, s *testing.State, testVideo string) {
	const cleanupTime = 5 * time.Second // time reserved for cleanup.

	cleanUpBenchmark, err := cpu.SetUpBenchmark(ctx)
	if err != nil {
		s.Fatal("Failed to set up benchmark mode: ", err)
	}
	defer cleanUpBenchmark(ctx)

	// Leave a bit of time to clean up benchmark mode.
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed waiting for CPU to become idle: ", err)
	}

	p := perf.NewValues()

	md := runARCVideoTestSetup(ctx, s, testVideo, false)

	for doRender := 0; doRender < 2; doRender++ {
		var filter string
		var subtestName string
		if doRender == 1 {
			filter = "C2VideoDecoderSurfaceE2ETest.TestFPS"
			subtestName = "render"
		} else {
			filter = "C2VideoDecoderSurfaceNoRenderE2ETest.TestFPS"
			subtestName = "no_render"
		}

		s.Log("Running ", subtestName)
		stats := runARCVideoTest(ctx, s, md, arcTestConfig{
			testVideo:      testVideo,
			requireMD5File: false,
			testFilter:     filter,
			measurePerf:    true,
			logPrefix:      subtestName,
		})

		if doRender == 1 {
			p.Set(perf.Metric{
				Name:      "cpu_usage",
				Unit:      "percent",
				Direction: perf.SmallerIsBetter,
			}, stats["cpu"])

			if power, ok := stats["power"]; ok {
				p.Set(perf.Metric{
					Name:      "power_consumption",
					Unit:      "watts",
					Direction: perf.SmallerIsBetter,
				}, power)
			}

			p.Set(perf.Metric{
				Name:      "dropped_frames",
				Unit:      "count",
				Direction: perf.SmallerIsBetter,
			}, stats["df"])

		} else {
			p.Set(perf.Metric{
				Name:      "max_fps",
				Unit:      "fps",
				Direction: perf.BiggerIsBetter,
			}, stats["fps"])
		}
	}

	if err := p.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save performance metrics: ", err)
	}
}
