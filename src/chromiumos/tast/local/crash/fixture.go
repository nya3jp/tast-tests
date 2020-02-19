// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/metrics"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	// crashUserAccessGID is the GID for crash-user-access, as defined in
	// third_party/eclass-overlay/profiles/base/accounts/group/crash-user-access.
	crashUserAccessGID = 420

	// collectChromeCrashFile is the name of a special file that tells crash_reporter's
	// UserCollector to always dump Chrome crashes. (Instead of the normal behavior
	// of skipping those crashes in user_collector in favor of letting ChromeCollector
	// handle them.) This behavior change will mess up several crash tests.
	collectChromeCrashFile = "/mnt/stateful_partition/etc/collect_chrome_crashes"
)

// SetConsent enables or disables metrics consent, based on the value of |consent|.
// Pre: cr must point to a logged-in chrome session.
func SetConsent(ctx context.Context, cr *chrome.Chrome, consent bool) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	if err := ensureSoftwareDeps(ctx); err != nil {
		return err
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "creating test API connection failed")
	}

	testing.ContextLogf(ctx, "Setting metrics consent to %t", consent)

	code := fmt.Sprintf("tast.promisify(chrome.autotestPrivate.setMetricsEnabled)(%t)", consent)
	if err := tconn.EvalPromise(ctx, code, nil); err != nil {
		return errors.Wrap(err, "running autotestPrivate.setMetricsEnabled failed")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		state, err := metrics.HasConsent(ctx)
		if err != nil {
			return testing.PollBreak(err)
		}
		if state != consent {
			return errors.Errorf("consent state mismatch: got %t, want %t", state, consent)
		}
		return nil
	}, nil); err != nil {
		return err
	}
	// Make sure that the updated status is polled by crash_reporter.
	// crash_reporter holds a cache of the consent status until the integer value of time() changes since last time.
	// https://chromium.googlesource.com/chromiumos/platform2/+/3fe852bfa/metrics/metrics_library.cc#154
	// The fraction of the end time is intentionally rounded down here.
	// For example, if the system clock were 12:34:56.700, the cache would be purged no later than 12:34:57.000.
	end := time.Unix(time.Now().Add(1*time.Second).Unix(), 0)
	testing.Sleep(ctx, end.Sub(time.Now()))
	return nil
}

// ensureSoftwareDeps checks that the current test declares appropriate software
// dependencies for crash tests.
func ensureSoftwareDeps(ctx context.Context) error {
	deps, ok := testing.ContextSoftwareDeps(ctx)
	if !ok {
		return errors.New("failed to extract software dependencies from context (using wrong context?)")
	}

	const exp = "metrics_consent"
	for _, dep := range deps {
		if dep == exp {
			return nil
		}
	}
	return errors.Errorf("crash tests must declare %q software dependency", exp)
}

// moveAllCrashesTo moves crashes from |source| to |target|. This allows us to
// start crash tests with an empty spool directory, reducing risk of flakes if
// the dir is already full when the test starts.
func moveAllCrashesTo(source, target string) error {
	files, err := ioutil.ReadDir(source)
	if err != nil {
		// Bubble this up so caller can check whether IsNotExist and behave accordingly.
		return err
	}
	if err := os.MkdirAll(target, 0755); err != nil {
		return errors.Wrapf(err, "couldn't make stash crash dir %s", target)
	}
	for _, f := range files {
		if err := os.Rename(filepath.Join(source, f.Name()), filepath.Join(target, f.Name())); err != nil {
			return errors.Wrapf(err, "couldn't move file: %v", f.Name())
		}
	}
	return nil
}

// Option is a self-referential function can be used to configure crash tests.
// See https://commandcenter.blogspot.com.au/2014/01/self-referential-functions-and-design.html
// for details about this pattern.
type Option func(p *setUpParams)

// DevImage prevents the test library from indicating to the DUT that a crash
// test is in progress, allowing the test to complete with standard developer
// image behavior.
func DevImage() Option {
	return func(p *setUpParams) {
		p.isDevImageTest = true
	}
}

// WithConsent indicates that the test should enable metrics consent.
// Pre: cr should be a logged-in chrome session.
func WithConsent(cr *chrome.Chrome) Option {
	return func(p *setUpParams) {
		p.setConsent = true
		p.chrome = cr
	}
}

// SetUpCrashTest indicates that we are running a test that involves the crash
// reporting system (crash_reporter, crash_sender, or anomaly_detector). The
// test should "defer TearDownCrashTest()" after calling this. If developer image
// behavior is required for the test, call SetUpDevImageCrashTest instead.
func SetUpCrashTest(ctx context.Context, opts ...Option) error {
	p := setUpParams{
		inProgDir:         crashTestInProgressDir,
		sysCrashDir:       SystemCrashDir,
		sysCrashStash:     systemCrashStash,
		chronosCrashDir:   LocalCrashDir,
		chronosCrashStash: localCrashStash,
		userCrashDir:      UserCrashDir,
		userCrashStash:    userCrashStash,
		senderPausePath:   senderPausePath,
		senderProcName:    senderProcName,
		mockSendingPath:   mockSendingPath,
		sendRecordDir:     SendRecordDir,
		isDevImageTest:    false,
		setConsent:        false,
		chrome:            nil,
	}
	for _, opt := range opts {
		opt(&p)
	}

	// This file usually doesn't exist; don't error out if it doesn't. "Not existing"
	// is the normal state, so we don't undo this in TearDownCrashTest().
	if err := os.Remove(collectChromeCrashFile); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to remove "+collectChromeCrashFile)
	}

	return setUpCrashTest(ctx, &p)
}

// SetUpDevImageCrashTest stashes away existing crash files to prevent tests which
// generate crashes from failing due to full crash directories. This function does
// not indicate to the DUT that a crash test is in progress, allowing the test to
// complete with standard developer image behavior. The test should
// "defer TearDownCrashTest()" after calling this
func SetUpDevImageCrashTest(ctx context.Context) error {
	return SetUpCrashTest(ctx, DevImage())
}

// setUpParams is a collection of parameters to setUpCrashTest.
type setUpParams struct {
	inProgDir         string
	sysCrashDir       string
	sysCrashStash     string
	chronosCrashDir   string
	chronosCrashStash string
	userCrashDir      string
	userCrashStash    string
	senderPausePath   string
	senderProcName    string
	mockSendingPath   string
	sendRecordDir     string
	isDevImageTest    bool
	setConsent        bool
	chrome            *chrome.Chrome
}

// SetCrashTestInProgress creates a file to tell crash_reporter that a crash_reporter test is in progress.
func SetCrashTestInProgress() error {
	filePath := filepath.Join(crashTestInProgressDir, crashTestInProgressFile)
	if err := ioutil.WriteFile(filePath, []byte("in-progress"), 0644); err != nil {
		return errors.Wrapf(err, "failed writing in-progress state file %s", filePath)
	}
	return nil
}

// UnsetCrashTestInProgress tells crash_reporter that no crash_reporter test is in progress.
func UnsetCrashTestInProgress() error {
	filePath := filepath.Join(crashTestInProgressDir, crashTestInProgressFile)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed to remove in-progress state file %s", filePath)
	}
	return nil
}

// setUpCrashTest is a helper function for SetUpCrashTest. We need
// this as a separate function for testing.
func setUpCrashTest(ctx context.Context, p *setUpParams) (retErr error) {
	defer func() {
		if retErr != nil {
			tearDownCrashTest(&tearDownParams{
				inProgDir:         p.inProgDir,
				sysCrashDir:       p.sysCrashDir,
				sysCrashStash:     p.sysCrashStash,
				chronosCrashDir:   p.chronosCrashDir,
				chronosCrashStash: p.chronosCrashStash,
				userCrashDir:      p.userCrashDir,
				userCrashStash:    p.userCrashStash,
				senderPausePath:   p.senderPausePath,
				mockSendingPath:   p.mockSendingPath,
			})
		}
	}()

	if p.setConsent {
		if err := SetConsent(ctx, p.chrome, true); err != nil {
			return errors.Wrap(err, "couldn't enable metrics consent")
		}
	}

	// Pause the periodic crash_sender job.
	if err := ioutil.WriteFile(p.senderPausePath, nil, 0644); err != nil {
		return err
	}
	// If crash_sender happens to be running, touching senderPausePath does not
	// stop it. Kill crash_sender processes to make sure there is no running
	// instance.
	if err := testexec.CommandContext(ctx, "pkill", "-9", "--exact", p.senderProcName).Run(); err != nil {
		// pkill exits with code 1 if it could find no matching process (see: man 1 pkill).
		// It is perfectly fine for our case.
		if ws, ok := testexec.GetWaitStatus(err); !ok || !ws.Exited() || ws.ExitStatus() != 1 {
			return errors.Wrap(err, "failed to kill crash_sender processes")
		}
	}
	// Configure crash_sender to prevent uploading crash reports actually.
	if err := enableMockSending(p.mockSendingPath, true); err != nil {
		return err
	}
	if err := resetSendRecords(p.sendRecordDir); err != nil {
		return err
	}

	// Move all crashes into stash directory so a full directory won't stop
	// us from saving a new crash report.
	if err := moveAllCrashesTo(p.sysCrashDir, p.sysCrashStash); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := moveAllCrashesTo(p.chronosCrashDir, p.chronosCrashStash); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := moveAllCrashesTo(p.userCrashDir, p.userCrashStash); err != nil && !os.IsNotExist(err) {
		return err
	}

	// If the test is meant to run with developer image behavior, return here to
	// avoid creating the directory that indicates a crash test is in progress.
	if p.isDevImageTest {
		return nil
	}

	if err := os.MkdirAll(p.inProgDir, 0755); err != nil {
		return errors.Wrapf(err, "could not make directory %v", p.inProgDir)
	}

	filePath := filepath.Join(p.inProgDir, crashTestInProgressFile)
	if err := ioutil.WriteFile(filePath, nil, 0644); err != nil {
		return errors.Wrapf(err, "could not create %v", filePath)
	}
	return nil
}

func cleanUpStashDir(stashDir, realDir string) error {
	// Stash dir should exist, so error if it doesn't.
	if err := moveAllCrashesTo(stashDir, realDir); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Remove(stashDir); err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrapf(err, "couldn't remove stash dir: %v", stashDir)
		}
	}
	return nil
}

// TearDownCrashTest undoes the work of SetUpCrashTest.
func TearDownCrashTest() error {
	var firstErr error
	p := tearDownParams{
		inProgDir:         crashTestInProgressDir,
		sysCrashDir:       SystemCrashDir,
		sysCrashStash:     systemCrashStash,
		chronosCrashDir:   LocalCrashDir,
		chronosCrashStash: localCrashStash,
		userCrashDir:      UserCrashDir,
		userCrashStash:    userCrashStash,
		senderPausePath:   senderPausePath,
		mockSendingPath:   mockSendingPath,
	}
	if err := tearDownCrashTest(&p); err != nil && firstErr == nil {
		firstErr = err
	}
	// The user crash directory should always be owned by chronos not root. The
	// unit tests don't run as root and can't chown, so skip this in tests.
	if err := os.Chown(LocalCrashDir, int(sysutil.ChronosUID), crashUserAccessGID); err != nil && firstErr == nil {
		firstErr = errors.Wrapf(err, "couldn't chown %s", LocalCrashDir)
	}
	return nil
}

// tearDownParams is a collection of parameters to tearDownCrashTest.
type tearDownParams struct {
	inProgDir         string
	sysCrashDir       string
	sysCrashStash     string
	chronosCrashDir   string
	chronosCrashStash string
	userCrashDir      string
	userCrashStash    string
	senderPausePath   string
	mockSendingPath   string
}

// tearDownCrashTest is a helper function for TearDownCrashTest. We need
// this as a separate function for testing.
func tearDownCrashTest(p *tearDownParams) error {
	var firstErr error

	// If crashTestInProgressFile does not exist, something else already removed the file
	// or it was never created (See SetUpDevImageCrashTest).
	// Well, whatever, we're in the correct state now (the file is gone).
	filePath := filepath.Join(p.inProgDir, crashTestInProgressFile)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) && firstErr == nil {
		firstErr = err
	}

	if err := cleanUpStashDir(p.sysCrashStash, p.sysCrashDir); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := cleanUpStashDir(p.chronosCrashStash, p.chronosCrashDir); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := cleanUpStashDir(p.userCrashStash, p.userCrashDir); err != nil && firstErr == nil {
		firstErr = err
	}

	if err := disableMockSending(p.mockSendingPath); err != nil {
		firstErr = err
	}

	if err := os.Remove(p.senderPausePath); err != nil && !os.IsNotExist(err) && firstErr == nil {
		firstErr = err
	}

	return firstErr
}
