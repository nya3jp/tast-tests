// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/media/vm"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CaptureUnittests,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Runs Chrome capture_unittests to exercise Chrome's video capture stack",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		// TODO(b/187020361): removed from group:camera-postsubmit.
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"bear.mjpeg"},
	})
}

func parseLastTestCase(logFile string) (string, error) {
	file, err := os.Open(logFile)
	if err != nil {
		return "", errors.Wrap(err, "failed to open log file")
	}
	defer file.Close()

	pattern := regexp.MustCompile(`\[\s*RUN\s*\]\s*(.*)`)
	scanner := bufio.NewScanner(file)
	lastTestCase := ""
	for scanner.Scan() {
		if matches := pattern.FindStringSubmatch(scanner.Text()); matches != nil {
			lastTestCase = matches[1]
		}
	}

	if err := scanner.Err(); err != nil {
		return "", errors.Wrap(err, "failed to scan log file")
	}
	return lastTestCase, nil
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
	logFile := filepath.Join(s.OutDir(), exec+".log")
	if report, err := gtest.New(
		filepath.Join(chrome.BinTestDir, exec),
		gtest.Logfile(logFile),
		gtest.Filter(filter),
		gtest.ExtraArgs(logging.ChromeVmoduleFlag(), "--test-launcher-jobs=1"),
		gtest.UID(int(sysutil.ChronosUID)),
	).Run(shortCtx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			if testCase, err := parseLastTestCase(logFile); err != nil {
				s.Error("Test timeout but failed to get last test case: ", err)
			} else {
				s.Error("Test timeout. The last test case: ", testCase)
			}
		} else {
			s.Errorf("Failed to run %v: %v", exec, err)
		}
		if report != nil {
			for _, name := range report.FailedTestNames() {
				s.Error(name, " failed")
			}
		}
	}
}
