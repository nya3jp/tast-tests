// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/bundles/cros/video/lib/vm"
	"chromiumos/tast/local/chrome/bintest"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CaptureUnittests,
		Desc:         "Run Chrome capture_unittests to exercise Chrome's video capture stack",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.USBCamera},
		Data:         []string{"bear.mjpeg"},
	})
}

// copyVideoData copies videofile to /usr/local/media/test/data/, where test data for
// capture_unittests should exist under.
func copyVideoFile(ctx context.Context, videofile string) error {
	cmd := testexec.CommandContext(ctx, "mkdir", "-p", "/usr/local/media/test/data/")
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return errors.Wrap(err, "failed to create /usr/local/media/test/data/")
	}

	cmd = testexec.CommandContext(ctx, "cp", videofile, "/usr/local/media/test/data/")
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return errors.Wrapf(err, "failed to copy %s", videofile)
	}

	return nil
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

	// Copy bear.mjpeg to /usr/local/media/test/data/, where capture_unittests
	// require test data to exist in.
	if err := copyVideoFile(ctx, s.DataPath("bear.mjpeg")); err != nil {
		s.Fatal("failed to copy video file: ", err)
	}

	// We skip CameraConfigChromeOSTest.ParseSuccessfully.
	// This is because this test will create a temporal file at working directory,
	// but it's not allowed in Tast.
	skipPattern := "CameraConfigChromeOSTest.ParseSuccessfully"
	if vm.IsRunningOnVM() {
		// Since vivid doesn't support MJPEG,
		// we cannot run CaptureMJpeg tests on ChromeOS VM
		skipPattern += ":*UsingRealWebcam_CaptureMjpeg*"
	}

	args := []string{
		logging.ChromeVmoduleFlag(),
		"--test-launcher-jobs=1",
		// Skip test cases that match with skipPattern
		"--gtest_filter=-" + skipPattern,
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
