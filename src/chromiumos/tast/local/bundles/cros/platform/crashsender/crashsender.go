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

var (
	leaveCrashSending      = true
	automaticConsentSaving = true
)

func checkHardware(ctx context.Context) error {
	rmap, err := lsbrelease.Load()
	if err != nil {
		return errors.Wrap(err, "failed to get lsb-release info")
	}
	// TODO(chavey) :  need to check that 'CHROMEOS_RELEASE_BOARD' matches
	//   result['output'] board
	if _, ok := rmap["CHROMEOS_RELEASE_BOARD"]; !ok {
		return errors.New("failed to get board from lsb-release info")
	}
	return nil
}

func initialize(ctx context.Context, s *testing.State) error {
	var err error
	leaveCrashSending = true
	automaticConsentSaving = true

	if tmpDir, err = ioutil.TempDir("/tmp", "crashsender"); err != nil {
		s.Log("Failed to create temporary directory")
		return errors.Wrap(err, "failed creating tmpdir")
	}
	if err = crashFiltering("none", true); err != nil {
		s.Log("Failed setting crash filteting: ", err)
		return errors.Wrap(err, "failed setting crash filtering")
	}
	if err = crashTestInProgress(true); err != nil {
		s.Log("Failed setting test in progress mode: ", err)
		return errors.Wrap(err, "failed setting test in progress state")
	}
	return nil
}

func cleanup(ctx context.Context, s *testing.State) error {
	cerr := false
	var err error

	if err = resetRateLimiting(); err != nil {
		s.Log("Failed reseting rate limiting: ", err)
		cerr = true
	}
	if err = clearSpooledCrashes(); err != nil {
		s.Log("Failed clearing spooled crashes: ", err)
		cerr = true
	}
	if err = systemSending(leaveCrashSending); err != nil {
		s.Log("Failed leaving crash sending mode: ", err)
		cerr = true
	}
	if err = sendingMock(false, true); err != nil {
		s.Log("Failed reseting sending mock: ", err)
		cerr = true
	}
	if automaticConsentSaving {
		if err = popConsent(); err != nil {
			s.Log("Failed reseting consent: ", err)
			cerr = true
		}
	}
	if err = crashFiltering("", false); err != nil {
		s.Log("Failed disabling crash filtering: ", err)
		cerr = true
	}
	if err = crashTestInProgress(false); err != nil {
		s.Log("Failed disabling crash test in progress: ", err)
		cerr = true
	}

	if len(tmpDir) != 0 {
		if err := os.RemoveAll(tmpDir); err != nil {
			s.Log("Failed to remove temp dir: ", err)
			cerr = true
		}
	}

	if cerr {
		return errors.New("failed during cleanup")
	}
	return nil
}

// RunTests runs the suite of tests.
func RunTests(ctx context.Context, s *testing.State) {
	InitializeCrashReporter := true //false
	LockCorePattern := false
	ClearSpoolFirst := true

	var err error

	s.Log("Initialize crash sender environment")
	err = initialize(ctx, s)
	if err != nil {
		s.Log("Failed initialization: ", err)
		goto cleanup
	}
	if automaticConsentSaving {
		if err = pushConsent(); err != nil {
			s.Log("Consent not pushed: ", err)
		}
	}

	if err = checkHardware(ctx); err != nil {
		s.Log("Failed to check hardware: ", err)
		goto cleanup
	}

	if InitializeCrashReporter {
		s.Log("Initialize crash reporter")
		if err := initializeCrashReporter(ctx, LockCorePattern); err != nil {
			s.Log("Crash reporter not initialized: ", err)
		}
	}
	if err = systemSending(false); err != nil {
		s.Log("Failed enabling crash_sender to run: ", err)
	}
	if err = killCrashSender(ctx); err != nil {
		s.Log("No crash_sender killed: ", err)
	}
	if err = resetRateLimiting(); err != nil {
		s.Log("Rate limiting not reset")
	}
	if ClearSpoolFirst {
		if err = clearSpooledCrashes(); err != nil {
			s.Log("Spooled crashes not clear: ", err)
		}
	}

	// start test section
	if err = checkSimpleMinidumpSend(ctx, s); err != nil {
		s.Log("Failed mini dump send: ", err)
	}

cleanup:
	s.Log("Cleanup crash sender environment")
	if err = cleanup(ctx, s); err != nil {
		s.Log("Failed cleaning up the test")
	}
}

func checkSimpleMinidumpSend(ctx context.Context, s *testing.State) error {
	s.Log("Check simple minidump send")
	err := callSenderOneCrash(ctx, s,
		&crashRequest{sendSuccess: true, reportsEnabled: true,
			report: "", shouldFail: false, ignorePause: true})
	return err
}

/*
func testSenderSimpleMinidump(ctx context.Context, s *testing.State) error {
	s.Log("Running testSenderSimpleMinidump")
	return nil
}

func testSenderSimpleOldMinidump(ctx context.Context, s *testing.State) error {
	s.Log("Running testSenderSimpleOldMinidump")
	return nil
}

func testSenderSimpleKernelCrash(ctx context.Context, s *testing.State) error {
	s.Log("Running testSenderSimpleKernelCrash")
	return nil
}

func testSenderPausing(ctx context.Context, s *testing.State) error {
	s.Log("Running testSenderPausing")
	return nil
}

func testSenderReportsDisabled(ctx context.Context, s *testing.State) error {
	s.Log("Running testSenderReportDisabled")
	return nil
}

func testSenderRateLimiting(ctx context.Context, s *testing.State) error {
	s.Log("Running testSenderRateLimiting")
	return nil
}

func testSenderSingleInstance(ctx context.Context, s *testing.State) error {
	s.Log("Running testSenderSingleInstance")
	return nil
}

func testSenderSendFails(ctx context.Context, s *testing.State) error {
	s.Log("Running testSenderSendFails")
	return nil
}
*/
