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

// ConsentType is to be used for parameters to tests, to allow them to determine
// whether they should use mock consent or real consent.
type ConsentType int

const (
	// MockConsent indicates that a test should use the mock consent system.
	MockConsent ConsentType = iota
	// RealConsent indicates that a test should use the real consent system.
	RealConsent
)

// SetConsent enables or disables metrics consent, based on the value of |consent|.
// Pre: cr must point to a logged-in chrome session.
func SetConsent(ctx context.Context, cr *chrome.Chrome, consent bool) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := ensureSoftwareDeps(ctx); err != nil {
		return err
	}

	// First, ensure that device ownership has been taken, for two reasons:
	// 1. Due to https://crbug.com/1042951#c16, if we set consent while
	//    ownership is in the OWNERSHIP_UNKNOWN state, we'll never actually
	//    set it (it will stay pending forever).
	// 2. Even if setting consent to pending works (we set it when we're in
	//    state OWNERSHIP_NONE), there is a brief time after ownership is
	//    taken where consent is reset to false. See
	//    https://crbug.com/1041062#c23
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat("/var/lib/whitelist/owner.key"); err != nil {
			if os.IsNotExist(err) {
				return err
			}
			return testing.PollBreak(err)
		}
		return nil
	}, nil); err != nil {
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

	// Wait for consent to be set before we return.
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

	// If a test wants consent to be turned off, make sure mock consent doesn't
	// interfere.
	if err := os.Remove(filepath.Join(crashTestInProgressDir, mockConsentFile)); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "unable to remove mock consent file")
	}
	if err := os.Remove(filepath.Join(SystemCrashDir, mockConsentFile)); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "unable to remove mock consent file")
	}

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

// WithMockConsent indicates that the test should touch the mock metrics consent
// file which causes crash_reporter and crash_sender to act as if they had
// consent to process crashes.
func WithMockConsent() Option {
	return func(p *setUpParams) {
		p.setMockConsent = true
	}
}

// RebootingTest indicates that this test will reboot the machine, and the crash
// reporting state files (e.g. crash-test-in-progress) should also be placed in
// /var/spool/crash/ (so that the persist-crash-test task moves them over to
// /run/crash_reporter on boot).
func RebootingTest() Option {
	return func(p *setUpParams) {
		p.rebootTest = true
	}
}

// SetUpCrashTest indicates that we are running a test that involves the crash
// reporting system (crash_reporter, crash_sender, or anomaly_detector). The
// test should "defer TearDownCrashTest(ctx)" after calling this. If developer image
// behavior is required for the test, call SetUpDevImageCrashTest instead.
func SetUpCrashTest(ctx context.Context, opts ...Option) error {
	daemonStorePaths, err := GetDaemonStoreCrashDirs(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get daemon store crash directories")
	}
	var daemonStoreStashPaths []string
	for _, path := range daemonStorePaths {
		daemonStoreStashPaths = append(daemonStoreStashPaths, path+".real")
	}

	p := setUpParams{
		inProgDir:         crashTestInProgressDir,
		sysCrashDir:       SystemCrashDir,
		sysCrashStash:     systemCrashStash,
		daemonStoreDir:    daemonStorePaths,
		daemonStoreStash:  daemonStoreStashPaths,
		chronosCrashDir:   LocalCrashDir,
		chronosCrashStash: localCrashStash,
		userCrashDir:      UserCrashDir,
		userCrashStash:    userCrashStash,
		senderPausePath:   senderPausePath,
		senderProcName:    senderProcName,
		mockSendingPath:   mockSendingPath,
		sendRecordDir:     SendRecordDir,
	}
	for _, opt := range opts {
		opt(&p)
	}

	// This file usually doesn't exist; don't error out if it doesn't. "Not existing"
	// is the normal state, so we don't undo this in TearDownCrashTest(ctx).
	if err := os.Remove(collectChromeCrashFile); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to remove "+collectChromeCrashFile)
	}

	return setUpCrashTest(ctx, &p)
}

// SetUpDevImageCrashTest stashes away existing crash files to prevent tests which
// generate crashes from failing due to full crash directories. This function does
// not indicate to the DUT that a crash test is in progress, allowing the test to
// complete with standard developer image behavior. The test should
// "defer TearDownCrashTest(ctx)" after calling this
func SetUpDevImageCrashTest(ctx context.Context) error {
	return SetUpCrashTest(ctx, DevImage())
}

// setUpParams is a collection of parameters to setUpCrashTest.
type setUpParams struct {
	inProgDir         string
	sysCrashDir       string
	sysCrashStash     string
	daemonStoreDir    []string
	daemonStoreStash  []string
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
	setMockConsent    bool
	rebootTest        bool
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
				daemonStoreDir:    p.daemonStoreDir,
				daemonStoreStash:  p.daemonStoreStash,
				chronosCrashDir:   p.chronosCrashDir,
				chronosCrashStash: p.chronosCrashStash,
				userCrashDir:      p.userCrashDir,
				userCrashStash:    p.userCrashStash,
				senderPausePath:   p.senderPausePath,
				mockSendingPath:   p.mockSendingPath,
			})
		}
	}()

	if p.setConsent && p.setMockConsent {
		return errors.New("Should not set consent and mock consent at the same time")
	}

	if err := os.MkdirAll(p.inProgDir, 0755); err != nil {
		return errors.Wrapf(err, "could not make directory %v", p.inProgDir)
	}

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
	for i := range p.daemonStoreDir {
		if err := moveAllCrashesTo(p.daemonStoreDir[i], p.daemonStoreStash[i]); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if err := moveAllCrashesTo(p.chronosCrashDir, p.chronosCrashStash); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := moveAllCrashesTo(p.userCrashDir, p.userCrashStash); err != nil && !os.IsNotExist(err) {
		return err
	}

	// We must set mock consent _after_ stashing crashes, or we'll stash
	// the mock consent for reboot tests.
	if p.setMockConsent {
		mockConsentPath := filepath.Join(p.inProgDir, mockConsentFile)
		if err := ioutil.WriteFile(mockConsentPath, nil, 0644); err != nil {
			return errors.Wrapf(err, "failed writing mock consent file %s", mockConsentPath)
		}
		if p.rebootTest {
			mockConsentPersistent := filepath.Join(p.sysCrashDir, mockConsentFile)
			if err := ioutil.WriteFile(mockConsentPersistent, nil, 0644); err != nil {
				return errors.Wrapf(err, "failed writing mock consent file %s", mockConsentPersistent)
			}
		}
	}

	// If the test is meant to run with developer image behavior, return here to
	// avoid creating the file that indicates a crash test is in progress.
	if p.isDevImageTest {
		return nil
	}

	filePath := filepath.Join(p.inProgDir, crashTestInProgressFile)
	if err := ioutil.WriteFile(filePath, nil, 0644); err != nil {
		return errors.Wrapf(err, "could not create %v", filePath)
	}

	if p.rebootTest {
		filePath = filepath.Join(p.sysCrashDir, crashTestInProgressFile)
		if err := ioutil.WriteFile(filePath, nil, 0644); err != nil {
			return errors.Wrapf(err, "could not create %v", filePath)
		}
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

// TearDownCrashTest undoes the work of SetUpCrashTest. We assume here that the set of active sessions hasn't changed since SetUpCrashTest was called for the purpose of restoring the per-user-cryptohome crash directories.
func TearDownCrashTest(ctx context.Context) error {
	var firstErr error

	// This could return a different list of paths then the original setup if a session has started or ended in the meantime. If a new session has started then the restore will just become a no-op, but if a session has ended we won't restore the crash files inside that session's cryptohome. We can't do much about this since we can't touch a cryptohome while that user isn't logged in.
	daemonStorePaths, err := GetDaemonStoreCrashDirs(ctx)
	if err != nil && firstErr == nil {
		firstErr = errors.Wrap(err, "failed to get daemon store crash directories")
	}
	var daemonStoreStashPaths []string
	for _, path := range daemonStorePaths {
		daemonStoreStashPaths = append(daemonStoreStashPaths, path+".real")
	}

	p := tearDownParams{
		inProgDir:         crashTestInProgressDir,
		sysCrashDir:       SystemCrashDir,
		sysCrashStash:     systemCrashStash,
		daemonStoreDir:    daemonStorePaths,
		daemonStoreStash:  daemonStoreStashPaths,
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
	// Only do this if the local crash dir actually exists.
	if _, err := os.Stat(LocalCrashDir); err == nil {
		if err := os.Chown(LocalCrashDir, int(sysutil.ChronosUID), crashUserAccessGID); err != nil && firstErr == nil {
			firstErr = errors.Wrapf(err, "couldn't chown %s", LocalCrashDir)
		}
	}
	return firstErr
}

// tearDownParams is a collection of parameters to tearDownCrashTest.
type tearDownParams struct {
	inProgDir         string
	sysCrashDir       string
	sysCrashStash     string
	daemonStoreDir    []string
	daemonStoreStash  []string
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
	filePath = filepath.Join(p.sysCrashDir, crashTestInProgressFile)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) && firstErr == nil {
		firstErr = err
	}

	if err := cleanUpStashDir(p.sysCrashStash, p.sysCrashDir); err != nil && firstErr == nil {
		firstErr = err
	}
	for i := range p.daemonStoreDir {
		if err := cleanUpStashDir(p.daemonStoreStash[i], p.daemonStoreDir[i]); err != nil && firstErr == nil {
			firstErr = err
		}
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

	mockConsentPath := filepath.Join(p.inProgDir, mockConsentFile)
	if err := os.Remove(mockConsentPath); err != nil && !os.IsNotExist(err) && firstErr == nil {
		firstErr = err
	}
	mockConsentPersistent := filepath.Join(p.sysCrashDir, mockConsentFile)
	if err := os.Remove(mockConsentPersistent); err != nil && !os.IsNotExist(err) && firstErr == nil {
		firstErr = err
	}

	if err := os.Remove(p.senderPausePath); err != nil && !os.IsNotExist(err) && firstErr == nil {
		firstErr = err
	}

	return firstErr
}
