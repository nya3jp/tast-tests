// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package libvda provides common code for decoding videos using libvda (go/libvda).
// Libvda connects to the GPU process via Chrome's LibvdaService D-Bus service, and
// then communicates via mojo to do accelerated video decoding.
package libvda

import (
	"context"
	"path/filepath"

	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/testing"
)

// RunGPUFileDecodeTest starts the libvda_gpu_unittest binary for a GPU file decode test.
// videoFile is the target file to be decoded and test output will be saved to logFile.
func RunGPUFileDecodeTest(ctx context.Context, s *testing.State, logFileName string, videoFile string) {
	chromeArgs := []string{
		logging.ChromeVmoduleFlag(),
		// This flag enables LibvdaService D-Bus service in Chrome.
		"--enable-arcvm",
	}
	// Login to Chrome so that LibvdaService is started.
	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromeArgs...))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	const exec = "/usr/local/libexec/libvda-gpu-tests/libvda_gpu_unittest"
	if err := gtest.RunCaseWithFlags(ctx, exec, "DecodeFileGpu", filepath.Join(s.OutDir(), logFileName), []string{
		"--test_video_file=" + s.DataPath(videoFile),
	}); err != nil {
		s.Error("GPU file decode test failed: ", err)
	}
}
