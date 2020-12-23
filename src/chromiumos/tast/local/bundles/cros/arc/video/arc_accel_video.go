// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
//

// Package video provides common code for arc.VideoDecodeAccel* tests.
package video

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/c2e2etest"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	// arcFilePath must be on the sdcard because of android permissions
	arcFilePath = "/sdcard/Download/c2_e2e_test/"
	textLogName = "gtest_logs.txt"
	xmlLogName  = "gtest_logs.xml"

	perfMeasurementDuration = time.Duration(30) * time.Second
	perfTestSlack           = time.Duration(180) * time.Second

	// PerfTestRuntime is the runtime for a single performance test case
	// * 2 because two sets of perf measurements are gathered per test (rendering, no rendering)
	PerfTestRuntime = (perfMeasurementDuration * 2) + perfTestSlack
)

// DecoderType represents the type of video decoder that can be used.
type DecoderType int

const (
	// HardwareDecoder is the decoder type that uses hardware decoding.
	HardwareDecoder DecoderType = iota
	// SoftwareDecoder is the decoder type that uses software decoding.
	SoftwareDecoder
)

// DecodeTestOptions contains all options for the video decoder test.
type DecodeTestOptions struct {
	// TestVideo stores the test video's name.
	TestVideo string
	// DecoderType indicates whether a HW or SW decoder will be used.
	DecoderType DecoderType
}

// arcTestConfig stores GoogleTest configuration passed to c2_e2e_test APK.
type arcTestConfig struct {
	// opts stores the decoder test options
	opts DecodeTestOptions
	// videoMetadata stores video metadata.
	metadata *c2e2etest.VideoMetadata
	// testFilter specifies test pattern the test can run.
	// If unspecified, c2_e2e_test runs all tests.
	testFilter string
	// logPrefix stores a special prefix added for a logfile if needed.
	logPrefix string
	// isPerf indicates whether this is a performance test.
	isPerf bool
}

// argsList converts arcTestConfig to a list of argument strings.
func (t *arcTestConfig) argsList() ([]string, error) {
	var args = []string{
		// Report results as an XML.
		fmt.Sprintf("--gtest_output=xml:%s%s", arcFilePath, xmlLogName),
	}

	// Generate '--test_video_data' flag from metadata
	dataPath := filepath.Join(arcFilePath, t.opts.TestVideo)
	sdArg, err := t.metadata.StreamDataArg(dataPath)
	if err != nil {
		return nil, err
	}
	args = append(args, sdArg)

	if t.testFilter != "" {
		args = append(args, "--gtest_filter="+t.testFilter)
	}

	if t.isPerf {
		args = append(args, "--loop")
	}

	if t.opts.DecoderType == SoftwareDecoder {
		args = append(args, "--use_sw_decoder")
	}

	return args, nil
}

// writeLinesToFile writes lines to filepath line by line.
func writeLinesToFile(lines []string, filepath string) error {
	return ioutil.WriteFile(filepath, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

func arcVideoTestCleanup(ctx context.Context, a *arc.ARC) {
	if err := a.Command(ctx, "rm", "-rf", arcFilePath).Run(testexec.DumpLogOnError); err != nil {
		testing.ContextLog(ctx, "Failed cleaning test data dir")
	}
}

func makeActivityFullscreen(ctx context.Context, activity *arc.Activity, tconn *chrome.TestConn) error {
	if err := activity.SetWindowState(ctx, tconn, arc.WindowStateFullscreen); err != nil {
		return err
	}

	return ash.WaitForARCAppWindowState(ctx, tconn, activity.PackageName(), ash.WindowStateFullscreen)
}

func getPreData(s *testing.State) *arc.PreData {
	// TODO(b/177029198): Always use fixtures and remove this function.
	if pre, ok := s.FixtValue().(*arc.PreData); ok {
		return pre
	}
	if pre, ok := s.PreValue().(arc.PreData); ok {
		return &pre
	}
	s.Fatal("Failed to get PreData")
	return nil
}

func runARCVideoTestSetup(ctx context.Context, s *testing.State, testVideo string, requireMD5File bool) *c2e2etest.VideoMetadata {
	a := getPreData(s).ARC

	videoPath := s.DataPath(testVideo)
	pushFiles := []string{videoPath}

	// Parse JSON metadata.
	md, err := c2e2etest.LoadMetadata(videoPath + ".json")
	if err != nil {
		s.Fatal("Failed to get metadata: ", err)
	}

	apkName, err := c2e2etest.ApkNameForArch(ctx, a)
	if err != nil {
		s.Fatal("Failed to get apk: ", err)
	}

	s.Log("Installing APK ", apkName)
	if err := a.Install(ctx, s.DataPath(apkName)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	s.Log("Granting storage permissions")
	if err := c2e2etest.GrantApkPermissions(ctx, a); err != nil {
		s.Fatal("Failed granting storage permission: ", err)
	}

	if requireMD5File {
		// Prepare frames MD5 file.
		frameMD5Path := videoPath + ".frames.md5"
		s.Logf("Preparing frames MD5 file %v from JSON metadata", frameMD5Path)
		if err := writeLinesToFile(md.MD5Checksums, frameMD5Path); err != nil {
			s.Fatalf("Failed to prepare frames MD5 file %s: %v", frameMD5Path, err)
		}

		pushFiles = append(pushFiles, frameMD5Path)
	}

	if err := a.Command(ctx, "mkdir", arcFilePath).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed creating test data dir: ", err)
	}
	// Push files to ARC container.
	for _, pushFile := range pushFiles {
		err := a.PushFile(ctx, pushFile, arcFilePath)
		if err != nil {
			s.Fatal("Failed to push video stream to ARC: ", err)
		}
	}

	return md
}

// runARCVideoTest runs c2_e2e_test APK in ARC.
// It fails if c2_e2e_test fails.
func runARCVideoTest(ctx context.Context, s *testing.State, cfg arcTestConfig) {
	shortCtx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := getPreData(s).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	a := getPreData(s).ARC
	defer a.Command(ctx, "rm", arcFilePath+textLogName).Run()

	args, err := cfg.argsList()
	if err != nil {
		s.Fatal("Failed to generate args list: ", err)
	}

	s.Log("Starting APK main activity")
	act, err := arc.NewActivity(a, c2e2etest.Pkg, c2e2etest.ActivityName)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.StartWithArgs(ctx, tconn, []string{"-W", "-n"}, []string{
		"--esa", "test-args", strings.Join(args, ","),
		"--es", "log-file", arcFilePath + textLogName}); err != nil {
		s.Fatal("Failed starting APK main activity: ", err)
	}

	s.Log("Making activity fullscreen")
	if err := makeActivityFullscreen(ctx, act, tconn); err != nil {
		s.Fatal("Failed to make activity fullscreen: ", err)
	}

	s.Log("Waiting for activity to finish")
	if err := act.WaitForFinished(ctx, ctxutil.MaxTimeout); err != nil {
		s.Fatal("Failed to wait for activity: ", err)
	}

	_, localXMLLogFile, err := c2e2etest.PullLogs(shortCtx, a, arcFilePath, s.OutDir(), cfg.logPrefix, textLogName, xmlLogName)

	if err != nil {
		s.Fatal("Failed to pull logs: ", err)
	}

	if err := c2e2etest.ValidateXMLLogs(localXMLLogFile); err != nil {
		s.Fatal("Failed to validate logs: ", err)
	}
}

// runARCVideoPerfTest runs c2_e2e_test APK in ARC and gathers perf statistics.
// It fails if c2_e2e_test fails.
// It returns a map of perf statistics containing fps, dropped frame, and cpu stats.
func runARCVideoPerfTest(ctx context.Context, s *testing.State, cfg arcTestConfig) (perf map[string]float64) {
	cr := getPreData(s).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	a := getPreData(s).ARC
	// Clean this up separately from the main cleanup function because a perf test run will invoke
	// this function multiple times.
	// We don't need to remove the XML log because GoogleTest overwrites it.
	// Note that if the ctx times out, the cleanup isn't necessary.
	defer a.Command(ctx, "rm", arcFilePath+textLogName).Run()

	args, err := cfg.argsList()
	if err != nil {
		s.Fatal("Failed to generate args list: ", err)
	}

	s.Log("Starting APK main activity")
	act, err := arc.NewActivity(a, c2e2etest.Pkg, c2e2etest.ActivityName)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()
	if err := act.StartWithArgs(ctx, tconn, []string{"-W", "-n"}, []string{
		"--esa", "test-args", strings.Join(args, ","),
		"--ez", "delay-start", "true",
		"--es", "log-file", arcFilePath + textLogName}); err != nil {
		s.Fatal("Failed starting APK main activity: ", err)
	}

	s.Log("Making activity fullscreen")
	if err := makeActivityFullscreen(ctx, act, tconn); err != nil {
		s.Fatal("Failed to make activity fullscreen: ", err)
	}

	s.Log("Starting test")
	if err := act.StartWithArgs(ctx, tconn, []string{"-W", "-n"}, []string{
		"-a", "org.chromium.c2.test.START_TEST"}); err != nil {
		s.Fatal("Failed to start test: ", err)
	}

	const measureDelay = time.Duration(5) * time.Second
	if err := testing.Sleep(ctx, measureDelay); err != nil {
		s.Fatal("Failed waiting for CPU usage to stabilize: ", err)
	}

	s.Log("Starting cpu measurement")
	var stats map[string]float64
	stats, err = cpu.MeasureUsage(ctx, perfMeasurementDuration)
	if err != nil {
		s.Fatal("Failed measuring CPU usage: ", err)
	}

	// Send a second start to trigger onNewIntent and stop the test
	s.Log("Stopping target")
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed stopping loop: ", err)
	}

	s.Log("Waiting for activity to finish")
	if err := act.WaitForFinished(ctx, ctxutil.MaxTimeout); err != nil {
		s.Fatal("Failed to wait for activity: ", err)
	}

	outLogFile, outXMLLogFile, err := c2e2etest.PullLogs(ctx, a, arcFilePath, s.OutDir(), cfg.logPrefix, textLogName, xmlLogName)
	if err != nil {
		s.Fatal("Failed to pull logs: ", err)
	}

	if err := c2e2etest.ValidateXMLLogs(outXMLLogFile); err != nil {
		s.Fatal("Failed to validate logs: ", err)
	}

	perfMap := make(map[string]float64)
	s.Logf("CPU Usage = %.4f", stats["cpu"])
	perfMap["cpu"] = stats["cpu"]

	fps, df, err := reportFrameStats(outLogFile)
	if err != nil {
		s.Fatal("Failed to report fps/dropped frames: ", err)
	}
	s.Logf("FPS = %.2f", fps)
	s.Logf("Dropped frames rate = %.2f%%", df*100)
	perfMap["fps"] = fps
	perfMap["df"] = df * 100
	return perfMap
}

// RunAllARCVideoTests runs all tests in c2_e2e_test.
func RunAllARCVideoTests(ctx context.Context, s *testing.State, opts DecodeTestOptions) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()
	defer arcVideoTestCleanup(ctx, getPreData(s).ARC)

	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	md := runARCVideoTestSetup(ctx, s, opts.TestVideo, true)

	runARCVideoTest(ctx, s, arcTestConfig{
		opts:       opts,
		metadata:   md,
		testFilter: "C2VideoDecoder*",
		isPerf:     false,
	})
}

// reportFrameStats parses FPS and dropped frame info from log file
func reportFrameStats(logPath string) (float64, float64, error) {
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

	regExpDroppedFrames := regexp.MustCompile(`(?m)^\[LOG\] Dropped frames rate: ([+\-]?[0-9.]+)$`)
	matches = regExpDroppedFrames.FindAllStringSubmatch(string(b), -1)
	if len(matches) != 1 {
		return 0, 0, errors.Errorf("wrong number of dropped frame matches in %v (got %d, want 1)", filepath.Base(logPath), len(matches))
	}

	df, err := strconv.ParseFloat(matches[0][1], 64)
	if err != nil {
		return 0, 0, errors.Wrapf(err, "failed to parse dropped frame value %q", matches[0][1])
	}

	return fps, df, nil
}

// RunARCVideoPerfTest runs testFPS in c2_e2e_test and sets as perf metric.
func RunARCVideoPerfTest(ctx context.Context, s *testing.State, opts DecodeTestOptions) {
	cleanUpBenchmark, err := cpu.SetUpBenchmark(ctx)
	if err != nil {
		s.Fatal("Failed to set up benchmark mode: ", err)
	}
	defer cleanUpBenchmark(ctx)
	defer arcVideoTestCleanup(ctx, getPreData(s).ARC)

	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed waiting for CPU to become idle: ", err)
	}

	p := perf.NewValues()
	md := runARCVideoTestSetup(ctx, s, opts.TestVideo, false)

	s.Log("Running render test")
	stats := runARCVideoPerfTest(ctx, s, arcTestConfig{
		opts:       opts,
		metadata:   md,
		testFilter: "C2VideoDecoderSurfaceE2ETest.TestFPS",
		logPrefix:  "render_",
		isPerf:     true,
	})

	p.Set(perf.Metric{
		Name:      "cpu_usage",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, stats["cpu"])

	p.Set(perf.Metric{
		Name:      "frame_drop_percentage",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, stats["df"])

	s.Log("Running no_render test")
	stats = runARCVideoPerfTest(ctx, s, arcTestConfig{
		opts:       opts,
		metadata:   md,
		testFilter: "C2VideoDecoderSurfaceNoRenderE2ETest.TestFPS",
		logPrefix:  "no_render_",
		isPerf:     true,
	})

	p.Set(perf.Metric{
		Name:      "max_fps",
		Unit:      "fps",
		Direction: perf.BiggerIsBetter,
	}, stats["fps"])

	if err := p.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save performance metrics: ", err)
	}
}
