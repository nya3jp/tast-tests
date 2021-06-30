// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package decode provides common code to run Chrome binary tests for video decoding.
package decode

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// RunAccelVideoSmokeTest runs the FlushAtEndOfStream test in the
// video_decode_accelerator_tests. The test only fails if the test binary
// crashes or the video decoder's kernel driver crashes.
// The motivation of the smoke test: on certain devices, when playing VP9
// profile 1 or 3, the kernel crashed. Though the profile was not supported
// by the decoder, kernel driver should not crash in any circumstances.
// Refer to https://crbug.com/951189 for more detail.
func RunAccelVideoSmokeTest(ctx context.Context, s *testing.State, filename string) {
	const cleanupTime = 10 * time.Second

	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging: ", err)
	}
	defer vl.Close()

	// Only a single process can have access to the GPU, so we are required to
	// call "stop ui" at the start of the test. This will shut down the chrome
	// process and allow us to claim ownership of the GPU.
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to stop ui: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	// Reserve time to restart the ui job and perform cleanup at the end of the test.
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	// Run the FlushAtEndOfStream test, but ignore errors. We expect the test
	// binary to cleanly terminate with an "exit status 1" error. We ignore the
	// contents of the test report as we're not interested in actual failures.
	// The tast test only fails if the process or kernel crashed.
	// TODO(crbug.com/998464) Kernel crashes will currently cause remaining
	// tests to be aborted.
	const exec = "video_decode_accelerator_tests"
	testing.ContextLogf(ctx, "Running %v with an invalid video stream, "+
		"test failures are expected but no crashes should occur", exec)
	if _, err := gtest.New(
		filepath.Join(chrome.BinTestDir, exec),
		gtest.Logfile(filepath.Join(s.OutDir(), exec+".log")),
		gtest.Filter("*FlushAtEndOfStream"),
		gtest.ExtraArgs(
			s.DataPath(filename),
			s.DataPath(filename+".json"),
			"--output_folder="+s.OutDir(),
			"--validator_type=none"),
		gtest.UID(int(sysutil.ChronosUID)),
	).Run(ctx); err != nil {
		// The test binary should run without crashing, but we expect the tests
		// themselves to fail. We can check the exit code to differentiate
		// between tests failing (exit code 1) and the test binary crashing
		// (e.g. exit code 139 on Linux).
		waitStatus, ok := testexec.GetWaitStatus(err)
		if !ok {
			s.Fatal("Failed to get gtest exit status")
		}
		if waitStatus.ExitStatus() != 1 {
			s.Fatalf("Failed to run %v: %v", exec, err)
		}
		testing.ContextLog(ctx, "No crashes detected, running video smoke test successful")
	}
}
