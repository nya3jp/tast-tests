// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crashsender

import (
	"context"
	"io/ioutil"
	"os"

	"chromiumos/tast/errors"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/testing"
)

// TODO(chavey) Those are similar to the ones used by udev_crash, need
// to merge. Right now we have to use this long name to isolate the collision
// with udev_crash in the same platform package.
const (
	MockMetricsOffPolicyFile = "crash_sender_mock_metrics_off_policy.bin"
	MockMetricsOnPolicyFile  = "crash_sender_mock_metrics_on_policy.bin"
	MockMetricsOwnerKeyFile  = "crash_sender_mock_metrics_owner.key"
)

func checkHardware(ctx context.Context) error {
	rmap, err := lsbrelease.Load()
	if err != nil {
		return errors.Wrap(err, "failed to get lsb-release info")
	}
	// TODO(chavey): Need to check that 'CHROMEOS_RELEASE_BOARD' matches
	//   result['output'] board
	if _, ok := rmap["CHROMEOS_RELEASE_BOARD"]; !ok {
		return errors.New("failed to get board from lsb-release info")
	}
	return nil
}

// RunTests runs the suite of tests.
func RunTests(ctx context.Context, s *testing.State) {
	s.Log("Setup crash sender environment")
	tmpDir, err := ioutil.TempDir("/tmp", "crashsender")
	if err != nil {
		s.Errorf("Failed to create temporary directory: %s", err)
		return
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			s.Errorf("Failed to remove temp dir: %s", err)
		}
	}()

	s.Log("Setup crash filtering")
	if err = crashFiltering("none"); err != nil {
		s.Errorf("Failed setting crash filteting: %s", err)
		return
	}
	defer func() {
		if err := crashFiltering(""); err != nil {
			s.Errorf("Failed disabling crash filtering: %s", err)
		}
	}()

	s.Log("Setup crash in progress state")
	if err = crashTestInProgress(true); err != nil {
		s.Errorf("Failed setting test in progress mode: %s", err)
		return
	}
	defer func() {
		if err := crashTestInProgress(false); err != nil {
			s.Errorf("Failed disabling crash test in progress: %s", err)
		}
	}()

	if err = checkHardware(ctx); err != nil {
		s.Errorf("Failed to check hardware: %s", err)
		return
	}

	s.Log("Initialize crash reporter")
	if err = initializeCrashReporter(ctx, false); err != nil {
		s.Errorf("Crash reporter not initialized: %s", err)
		return
	}

	if err = enableCrashSender(); err != nil {
		s.Errorf("Failed enabling crash_sender: %s", err)
		return
	}
	defer func() {
		if err := disableCrashSender(); err != nil {
			s.Errorf("Failed disabling crash_sender: %s", err)
		}
	}()

	// TODO(chavey): Until we fix the function to check that the process
	//  exist, ignore the return.
	if err = killCrashSender(ctx); err != nil {
		s.Logf("No crash_sender killed: %s", err)
	}

	if err = resetRateLimiting(); err != nil {
		s.Errorf("Rate limiting not reset: %s", err)
		return
	}

	if err = clearSpooledCrashes(); err != nil {
		s.Errorf("Spooled crashes not clear: %s", err)
		return
	}

	defer func() {
		if err := resetRateLimiting(); err != nil {
			s.Errorf("Failed reseting rate limiting: %s", err)
		}
		if err := clearSpooledCrashes(); err != nil {
			s.Errorf("Failed clearing spooled crashes: %s", err)
		}
		if err := disableCrashSenderMock(); err != nil {
			s.Errorf("Failed reseting sending mock: %s", err)
		}
	}()

	// start test section
	s.Log("Test simple minidump send")
	if err = testSimpleMinidumpSend(ctx, s); err != nil {
		s.Errorf("Failed mini dump send: %s", err)
	}
	return
}

func testSimpleMinidumpSend(ctx context.Context, s *testing.State) error {
	err := callSenderOneCrash(ctx,
		&crashRequest{sendSuccess: true, reportsEnabled: true,
			report: "", shouldFail: false, ignorePause: true,
			mockOffPolicyFile: s.DataPath(MockMetricsOffPolicyFile),
			mockOnPolicyFile:  s.DataPath(MockMetricsOnPolicyFile),
			mockKeyFile:       s.DataPath(MockMetricsOffPolicyFile)})
	return err
}
