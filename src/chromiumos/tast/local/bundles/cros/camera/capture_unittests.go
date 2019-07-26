// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/media/vm"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CaptureUnittests,
		Desc: "Runs Chrome capture_unittests to exercise Chrome's video capture stack",
		Contacts: []string{
			"keiichiw@chromium.org", // Video team
			"shik@chromium.org",     // Camera team
			"chromeos-camera-eng@google.com",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera, "camera_720p"},
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

	if err := upstart.StopJob(shortCtx, "ui"); err != nil {
		s.Error("Failed to stop ui: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	// The cros-camera job exists only on boards that use the new camera stack.
	if upstart.JobExists(shortCtx, "cros-camera") {
		// Ensure that cros-camera service is running, because the service
		// might stopped due to the errors from some previous tests, and failed
		// to restart for some reasons.
		if err := upstart.EnsureJobRunning(shortCtx, "cros-camera"); err != nil {
			s.Fatal("Failed to start cros-camera: ", err)
		}
	}

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

	var filter string
	if vm.IsRunningOnVM() {
		// Since vivid doesn't support MJPEG,
		// we cannot run CaptureMJpeg tests on ChromeOS VM.
		filter = "-*UsingRealWebcam_CaptureMjpeg*"
	}

	const exec = "capture_unittests"
	if report, err := gtest.New(
		filepath.Join(chrome.BinTestDir, exec),
		gtest.Logfile(filepath.Join(s.OutDir(), exec+".log")),
		gtest.Filter(filter),
		gtest.ExtraArgs(logging.ChromeVmoduleFlag(), "--test-launcher-jobs=1"),
		gtest.UID(int(sysutil.ChronosUID)),
	).Run(shortCtx); err != nil {
		s.Errorf("Failed to run %v: %v", exec, err)
		if report != nil {
			for _, name := range report.FailedTestNames() {
				s.Error(name, " failed")
			}
		}
	}
}
