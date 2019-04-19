// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/fsutil"
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
		Desc:         "Runs Chrome capture_unittests to exercise Chrome's video capture stack",
		Contacts:     []string{"keiichiw@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinCamera, "camera_720p"},
		Data:         []string{"bear.mjpeg"},
	})
}

// CaptureUnittests runs Chrome's capture_unittests.
func CaptureUnittests(ctx context.Context, s *testing.State) {
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
	// requires test data to exist in.
	const (
		dataDir  = "/usr/local/media/test/data/"
		testFile = "bear.mjpeg"
	)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		s.Fatalf("Failed to create %s: %v", dataDir, err)
	}
	if err := fsutil.CopyFile(s.DataPath(testFile),
		filepath.Join(dataDir, testFile)); err != nil {
		s.Fatalf("Failed to copy %s: %v", testFile, err)
	}

	args := []string{
		logging.ChromeVmoduleFlag(),
		"--test-launcher-jobs=1",
	}

	if vm.IsRunningOnVM() {
		// Since vivid doesn't support MJPEG,
		// we cannot run CaptureMJpeg tests on ChromeOS VM.
		args = append(args, "--gtest_filter=-*UsingRealWebcam_CaptureMjpeg*")
	}

	const exec = "capture_unittests"
	if ts, err := bintest.Run(shortCtx, exec, args, s.OutDir()); err != nil {
		s.Errorf("Failed to run %v: %v", exec, err)
		for _, t := range ts {
			s.Error(t, " failed")
		}
	}
}
