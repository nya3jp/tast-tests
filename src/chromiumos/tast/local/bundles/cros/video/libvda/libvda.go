// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package libvda provides common code for testing libvda's GPU implementation (go/libvda).
// Libvda's GPU implementation connects to the GPU process via Chrome's LibvdaService D-Bus service,
// and then communicates via mojo to do accelerated video decoding.
package libvda

import (
	"context"
	"path/filepath"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/testing"
)

const (
	gpuTestBinaryPath  = "/usr/local/libexec/libvda-gpu-tests/libvda_gpu_unittest"
	fileDecodeTestCase = "LibvdaGpuTest.DecodeFileGpu"
)

func startChrome(ctx context.Context) (*chrome.Chrome, error) {
	chromeArgs := []string{
		logging.ChromeVmoduleFlag(),
		// This flag enables LibvdaService D-Bus service in Chrome.
		"--enable-arcvm",
	}
	// Login to Chrome so that LibvdaService is started.
	return chrome.New(ctx, chrome.ExtraArgs(chromeArgs...), chrome.ARCEnabled())
}

// RunGPUFileDecodeTest runs libvda_gpu_unittest for the GPU file decode test.
// videoFile is the target file to be decoded and test output will be saved to logFile.
func RunGPUFileDecodeTest(ctx context.Context, s *testing.State, logFileName string, videoFile string) {
	cr, err := startChrome(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	if _, err := gtest.New(gpuTestBinaryPath,
		gtest.Logfile(filepath.Join(s.OutDir(), logFileName)),
		gtest.Filter(fileDecodeTestCase),
		gtest.ExtraArgs("--test_video_file="+s.DataPath(videoFile)),
	).Run(ctx); err != nil {
		s.Error("GPU file decode test failed: ", err)
	}
}

// RunGPUNonDecodeTests runs libvda_gpu_unittest for the non-file decoding tests.
func RunGPUNonDecodeTests(ctx context.Context, s *testing.State) {
	cr, err := startChrome(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	list, err := gtest.ListTests(ctx, gpuTestBinaryPath)
	if err != nil {
		s.Fatal("Failed to list gtest test cases: ", err)
	}

	for _, testcase := range list {
		if testcase == fileDecodeTestCase {
			continue
		}
		if _, err := gtest.New(gpuTestBinaryPath,
			gtest.Logfile(filepath.Join(s.OutDir(), testcase+".log")),
			gtest.Filter(testcase),
		).Run(ctx); err != nil {
			s.Errorf("GPU test case %s failed: %v", testcase, err)
		}
	}
}
