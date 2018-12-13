// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/bundles/cros/video/lib/vm"
	"chromiumos/tast/local/chrome/bintest"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CaptureUnittests,
		Desc:         "Run Chrome capture_unittests to exercise Chrome's video capture stack",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.USBCamera},
	})
}

// runCaptureUnittests runs capture_unittests.
// It fails if capture_unittests fails.
func runCaptureUnittests(ctx context.Context, s *testing.State) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging: ", err)
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

	// TODO(crbug.com/914774): Skip tests requiring a test data file
	// until capture_unittests supports a way to specify test data files' path.
	excludePattern := "FileVideoCaptureDeviceTest*"
	if vm.IsRunningOnVM() {
		// Since vivid doesn't support MJPEG,
		// we cannot run CaptureMJpeg tests on ChromeOS VM
		excludePattern += ":*UsingRealWebcam_CaptureMjpeg*"
	}

	args := []string{
		logging.ChromeVmoduleFlag(),
		"--test-launcher-jobs=1",
		"--gtest_filter=-" + excludePattern,
	}

	const exec = "capture_unittests"
	if err := bintest.Run(shortCtx, exec, args, s.OutDir()); err != nil {
		s.Fatalf("Failed to run %v: %v", exec, err)
	}
}

// CaptureUnittests runs Chrome's capture_unittests.
func CaptureUnittests(ctx context.Context, s *testing.State) {
	runCaptureUnittests(ctx, s)
}
