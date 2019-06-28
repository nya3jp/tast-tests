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

// TODO(chavey): Looks like these files are removed from udev_crash.
//   https://crrev.com/c/1730246/5
//   The all consent work can be simplified by using  call metrics.SetConsent()
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
// TODO(chavey): add comments from http://cs/chromeos_public/src/third_party/autotest/files/client/cros/crash/crash_test.py?l=22
func RunTests(ctx context.Context, s *testing.State) {
	tmpDir, err := ioutil.TempDir("/tmp", "crashsender")
	if err != nil {
		s.Fatal("Failed to create temporary directory: ", err)
	}
	defer func() {
		// TODO(chavey): Once the test is stable, we should not fail the
		//   test if we can not remove the temp data, an info in the log
		//   warning? is all that is needed.
		if err := os.RemoveAll(tmpDir); err != nil {
			s.Fatal("Failed to remove temp dir: ", err)
		}
	}()

	origCorePattern, err := readCorePattern()
	if err != nil {
		s.Fatal("Failed to get core pattern: ", err)
	}
	defer func() {
		if err := writeCorePattern(origCorePattern); err != nil {
			s.Fatal("Failed reseting core pattern: ", err)
		}
	}()
	if err := replaceCrashFilterIn("none"); err != nil {
		s.Fatal("Failed setting crash filteting: ", err)
	}

	if err := crashTestInProgress(true); err != nil {
		s.Fatal("Failed setting test in progress mode: ", err)
	}
	defer func() {
		if err := crashTestInProgress(false); err != nil {
			s.Fatal("Failed disabling crash test in progress: ", err)
		}
	}()

	if err := checkHardware(ctx); err != nil {
		s.Fatal("Failed to check hardware: ", err)
	}

	if err := initializeCrashReporter(ctx, false); err != nil {
		s.Fatal("Crash reporter not initialized: ", err)
	}

	if err := disableCrashSender(); err != nil {
		s.Fatal("Failed disabling crash_sender: ", err)
	}
	defer func() {
		if err := enableCrashSender(); err != nil {
			s.Fatal("Failed enabling crash_sender: ", err)
		}
	}()

	if err := killCrashSender(ctx); err != nil {
		s.Fatal("Failed to kill crash sender: ", err)
	}

	if err := resetRateLimiting(); err != nil {
		s.Fatal("Rate limiting not reset: ", err)
	}

	if err := clearSpooledCrashes(); err != nil {
		s.Fatal("Spooled crashes not clear: ", err)
	}

	defer func() {
		if err := resetRateLimiting(); err != nil {
			s.Fatal("Failed reseting rate limiting: ", err)
		}
		if err := clearSpooledCrashes(); err != nil {
			s.Fatal("Failed clearing spooled crashes: ", err)
		}
		if err := disableCrashSenderMock(); err != nil {
			s.Fatal("Failed disabling mock: ", err)
		}
	}()

	if err := testSimpleMinidumpSend(ctx, s); err != nil {
		s.Fatal("Failed mini dump send: ", err)
	}
	return
}

func testSimpleMinidumpSend(ctx context.Context, s *testing.State) error {
	_, err := callCrashSender(ctx,
		&crashRequest{
			sendSuccess:       true,
			reportsEnabled:    true,
			report:            "",
			shouldFail:        false,
			ignorePause:       true,
			mockOffPolicyFile: s.DataPath(MockMetricsOffPolicyFile),
			mockOnPolicyFile:  s.DataPath(MockMetricsOnPolicyFile),
			mockKeyFile:       s.DataPath(MockMetricsOwnerKeyFile)})
	if err != nil {
		return err
	}
	// TODO(chavey): Need to implement checking the content of the crash
	//   information. callCrashSender() returns a map of key:value pairs that
	//   are used to validate that a simple minidump was sent.
	return nil
}
