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
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

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

// toStreamDataArg returns a string that can be used for an argument of arcvideodecoder_test.
// dataPath is the absolute path of the video file.
func (d *decodeMetadata) toStreamDataArg(dataPath string) (string, error) {
	pEnum, found := videoCodecEnumValues[d.Profile]
	if !found {
		return "", errors.Errorf("cannot find enum value for profile %v", d.Profile)
	}

	// Set MinFPSNoRender and MinFPSWithRender to 0 for disabling FPS check because we would like
	// TestFPS to be always passed and store FPS value into perf metric.
	sdArg := fmt.Sprintf("--test_video_data=%s:%d:%d:%d:%d:0:0:%d",
		dataPath, d.Width, d.Height, d.NumFrames, d.NumFragments, pEnum)
	return sdArg, nil
}

// arcTestConfig stores test configuration to run arcvideodecoder_test.
type arcTestConfig struct {
	// testVideo stores the test video's name.
	testVideo string
	// requireMD5File indicates whether to prepare MD5 file for test.
	requireMD5File bool
	// testFilter specifies test pattern the test can run.
	// If unspecified, arcvideodecoder_test runs all tests.
	testFilter string
}

// toArgsList converts arcTestConfig to a list of argument strings.
// md is the decodeMetadata parsed from JSON file.
func (t *arcTestConfig) toArgsList(md decodeMetadata) ([]string, error) {
	// arcvideodecoder_test only.
	dataPath := filepath.Join(arc.ARCTmpDirPath, t.testVideo)
	sdArg, err := md.toStreamDataArg(dataPath)
	if err != nil {
		return nil, err
	}
	return []string{sdArg}, nil
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

// runARCVideoTest runs arcvideodecoder_test in ARC.
// It fails if arcvideodecoder_test fails.
// It returns logs where key is the exec name and value is the corresponding log path.
func runARCVideoTest(ctx context.Context, s *testing.State, cfg arcTestConfig) (logs map[string]string) {
	shortCtx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	a := s.PreValue().(arc.PreData).ARC

	videoPath := s.DataPath(cfg.testVideo)
	pushFiles := []string{videoPath}

	// Parse JSON metadata.
	// TODO(johnylin) Adapt ARC decoder test to use the json file directly.
	jf, err := os.Open(videoPath + ".json")
	if err != nil {
		s.Fatal("Failed to open JSON file: ", err)
	}
	defer jf.Close()

	var md decodeMetadata
	if err := json.NewDecoder(jf).Decode(&md); err != nil {
		s.Fatal("Failed to parse metadata from JSON file: ", err)
	}

	if cfg.requireMD5File {
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
		arcPath, err := a.PushFileToTmpDir(shortCtx, pushFile)
		if err != nil {
			s.Fatal("Failed to push video stream to ARC: ", err)
		}
		defer a.Command(ctx, "rm", arcPath).Run()
	}

	args, err := cfg.toArgsList(md)
	if err != nil {
		s.Fatal("Failed to generate args list: ", err)
	}

	// Push test binary files to ARC container. For x86_64 device we might install both amd64 and x86 binaries.
	const testexec = "arcvideodecoder_test"
	execs, err := a.PushTestBinaryToTmpDir(shortCtx, testexec)
	if err != nil {
		s.Fatal("Failed to push test binary to ARC: ", err)
	}
	if len(execs) == 0 {
		s.Fatal("Test binary is not found in ", arc.TestBinaryDirPath)
	}
	defer a.Command(ctx, "rm", execs...).Run()

	logs = make(map[string]string)

	// Execute binary in ARC.
	for _, exec := range execs {
		outputLogFile := filepath.Join(s.OutDir(), fmt.Sprintf("output_%s_%s.log", filepath.Base(exec), time.Now().Format("20060102-150405")))
		if report, err := gtest.New(
			exec,
			gtest.Logfile(outputLogFile),
			gtest.Filter(cfg.testFilter),
			gtest.ExtraArgs(args...),
			gtest.ARC(a),
		).Run(ctx); err != nil {
			s.Errorf("Failed to run %v: %v", exec, err)
			if report != nil {
				for _, name := range report.FailedTestNames() {
					s.Error(name, " failed")
				}
			}
			continue
		}

		logs[filepath.Base(exec)] = outputLogFile
	}
	return logs
}

// RunAllARCVideoTests runs all tests in arcvideodecoder_test.
func RunAllARCVideoTests(ctx context.Context, s *testing.State, testVideo string) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	runARCVideoTest(ctx, s, arcTestConfig{
		testVideo:      testVideo,
		requireMD5File: true,
	})
}

// reportFPS reports FPS info from log file and sets as the perf metric.
func reportFPS(p *perf.Values, name, logPath string) error {
	b, err := ioutil.ReadFile(logPath)
	if err != nil {
		return errors.Wrap(err, "failed to read file")
	}

	regExpFPS := regexp.MustCompile(`(?m)^\[LOG\] Measured decoder FPS: ([+\-]?[0-9.]+)$`)
	matches := regExpFPS.FindAllStringSubmatch(string(b), -1)
	if len(matches) != 1 {
		return errors.Errorf("found %d FPS matches in %v; want 1", len(matches), filepath.Base(logPath))
	}

	fps, err := strconv.ParseFloat(matches[0][1], 64)
	if err != nil {
		return errors.Wrapf(err, "failed to parse FPS value %q", matches[0][1])
	}

	p.Set(perf.Metric{
		Name:      fmt.Sprintf("tast_%s.fps", name),
		Unit:      "fps",
		Direction: perf.BiggerIsBetter,
	}, fps)
	return nil
}

// RunARCVideoPerfTest runs testFPS in arcvideodecoder_test and sets as perf metric.
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

	logs := runARCVideoTest(ctx, s, arcTestConfig{
		testVideo:      testVideo,
		requireMD5File: false,
		testFilter:     "ArcVideoDecoderE2ETest.TestFPS",
	})

	// Report FPS as perf metric for each exec.
	pv := perf.NewValues()
	for exec, logPath := range logs {
		s.Logf("Reporting FPS value parsed from %v for %v", logPath, exec)
		if err := reportFPS(pv, exec, logPath); err != nil {
			s.Errorf("Failed to report FPS for %v: %v", exec, err)
		}
	}
	pv.Save(s.OutDir())
}
