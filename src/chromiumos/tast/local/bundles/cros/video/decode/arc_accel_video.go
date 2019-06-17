// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package decode provides common code to run Chrome binary tests for video decoding.
package decode

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/video/lib/arctest"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
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
	pEnum := toVideoCodecEnum(d.Profile)
	if pEnum < 0 {
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
	args := []string{sdArg}

	// Common arguments.
	if t.testFilter != "" {
		args = append(args, fmt.Sprintf("--gtest_filter=%s", t.testFilter))
	}
	return args, nil
}

// writeLinesToFile writes lines to filepath line by line.
func writeLinesToFile(lines []string, filepath string) error {
	return ioutil.WriteFile(filepath, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

// toVideoCodecEnum coverts profile string to its enum value.
// These values must match integers in VideoCodecProfile in https://cs.chromium.org/chromium/src/media/base/video_codecs.h
func toVideoCodecEnum(profile string) int {
	switch profile {
	case "H264PROFILE_MAIN":
		return 1
	case "VP8PROFILE_ANY":
		return 11
	case "VP9PROFILE_PROFILE0":
		return 12
	default:
		return -1
	}
}

// runARCVideoTest runs arcvideodecoder_test in ARC.
// It fails if arcvideodecoder_test fails.
func runARCVideoTest(ctx context.Context, s *testing.State, a *arc.ARC, cfg arcTestConfig) {
	shortCtx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

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

	// Execute binary in ARC.
	for _, exec := range execs {
		outputLogFile := filepath.Join(s.OutDir(), fmt.Sprintf("output_%s_%s.log", filepath.Base(exec), time.Now().Format("20060102-150405")))
		outFile, err := os.Create(outputLogFile)
		if err != nil {
			s.Fatal("failed to create output log file: ", err)
		}
		defer outFile.Close()

		if err := arctest.RunARCBinary(shortCtx, a, exec, args, s.OutDir(), outFile); err != nil {
			s.Errorf("Failed to run %v: %v", exec, err)
		}
	}
}

// RunAllARCVideoTests runs all tests in arcvideodecoder_test.
func RunAllARCVideoTests(ctx context.Context, s *testing.State, a *arc.ARC, testVideo string) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	runARCVideoTest(ctx, s, a, arcTestConfig{
		testVideo:      testVideo,
		requireMD5File: true,
	})
}
