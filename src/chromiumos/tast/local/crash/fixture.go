// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/metrics"
	"chromiumos/tast/local/sysutil"
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

	// rebootPersistenceCount is the number of reboots across which the
	// mock-consent and crash-test-in-progress files should persist, in
	// case the DUT reboots multiple times during the test. It is only used
	// when the RebootingTest option is passed.
	rebootPersistenceCount = "4"

	// rebootPersistDir is the directory to which mock-consent and
	// crash-test-in-progress written to in order to preserve them across
	// reboot.
	rebootPersistDir = "/mnt/stateful_partition/unencrypted/preserve/"

	// daemonStoreConsentName is the name of file in daemon-store that
	// gives per-user consent state.
	daemonStoreConsentName = "consent-enabled"
)

// ConsentType is to be used for parameters to tests, to allow them to determine
// whether they should use mock consent or real consent.
type ConsentType int

const (
	// MockConsent indicates that a test should use the mock consent system.
	MockConsent ConsentType = iota
	// RealConsent indicates that a test should use the real consent system.
	RealConsent
	// RealConsentPerUserOn indicates that a test should use the real
	// consent system, and also turn *on* per-user consent.
	RealConsentPerUserOn
	// RealConsentPerUserOff indicates that a test should use the real
	// consent system, and also turn *off* per-user consent.
	// Crashes should not be collected in this case.
	RealConsentPerUserOff
)

// SetConsent enables or disables metrics consent, based on the value of consent.
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
		if _, err := os.Stat("/var/lib/devicesettings/owner.key"); err != nil {
			if os.IsNotExist(err) {
				return err
			}
			return testing.PollBreak(err)
		}
		return nil
	}, nil); err != nil {
		return errors.Wrap(err, "timed out while waiting for device ownership")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "creating test API connection failed")
	}

	testing.ContextLogf(ctx, "Setting metrics consent to %t", consent)

	if err := tconn.Call(ctx, nil, "tast.promisify(chrome.autotestPrivate.setMetricsEnabled)", consent); err != nil {
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
		return errors.Wrap(err, "timed out while waiting for consent")
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

// moveAllCrashesTo moves crashes from source to target. This allows us to
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
		if f.IsDir() {
			// Don't move directories (like the "attachments" directory that crashpad creates).
			continue
		}

		err := os.Rename(filepath.Join(source, f.Name()), filepath.Join(target, f.Name()))
		// Ignore error if the source was removed.
		// This could happen, for example, if moveAllCrashesTo races with early-failure-cleanup.
		// NOTE: We need to check both the os.Rename() return value as well as Stat()'ing the source
		// file because our destination is of form "target/foo", and "target" may not exist.
		if errors.Is(err, os.ErrNotExist) {
			if _, err := os.Stat(filepath.Join(source, f.Name())); err != nil && errors.Is(err, os.ErrNotExist) {
				continue
			}
		}
		if err != nil {
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
// rebootPersistDir (so that the persist-crash-test task moves them over to
// /run/crash_reporter on boot).
func RebootingTest() Option {
	return func(p *setUpParams) {
		p.rebootTest = true
	}
}

// FilterCrashes puts s into the filter-in file, so that the crash reporter only
// processes matching crashes.
// If this option is used, then only invocation of crash_reporter with arguments
// that contains s will be processed. See platform2/crash-reporter/README.md and
// search for "filter-in" for more info.
func FilterCrashes(s string) Option {
	return func(p *setUpParams) {
		p.filterIn = s
	}
}

// SetUpCrashTest indicates that we are running a test that involves the crash
// reporting system (crash_reporter, crash_sender, or anomaly_detector). The
// test should "defer TearDownCrashTest(ctx)" after calling this. If developer image
// behavior is required for the test, call SetUpDevImageCrashTest instead.
func SetUpCrashTest(ctx context.Context, opts ...Option) error {
	crashDirs := []crashAndStash{
		{SystemCrashDir, systemCrashStash},
		{LocalCrashDir, localCrashStash},
		{ClobberCrashDir, clobberCrashStash},
	}

	p := setUpParams{
		inProgDir:        crashTestInProgressDir,
		crashDirs:        crashDirs,
		rebootPersistDir: rebootPersistDir,
		senderPausePath:  senderPausePath,
		filterInPath:     FilterInPath,
		senderProcName:   senderProcName,
		mockSendingPath:  mockSendingPath,
		sendRecordDir:    SendRecordDir,
	}
	for _, opt := range opts {
		opt(&p)
	}
	// Unconditionally stash daemon-store crash dirs now that we're almost-always using daemon-store.
	daemonStorePaths, err := GetDaemonStoreCrashDirs(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get daemon store crash directories")
	}
	for _, path := range daemonStorePaths {
		p.crashDirs = append(p.crashDirs, crashAndStash{path, path + ".real"})
	}

	// This file usually doesn't exist; don't error out if it doesn't. "Not existing"
	// is the normal state, so we don't undo this in TearDownCrashTest(ctx).
	if err := os.Remove(collectChromeCrashFile); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to remove "+collectChromeCrashFile)
	}

	// Clean up per-user consent so that we do not inadvertently use consent left over from a prior test.
	if err := RemovePerUserConsent(ctx); err != nil {
		return errors.Wrap(err, "failed to clean up per-user consent")
	}

	// Reinitialize crash_reporter in case previous tests have left bad state
	// in core_pattern, etc.
	if out, err := testexec.CommandContext(ctx, "/sbin/crash_reporter", "--init").CombinedOutput(); err != nil {
		testing.ContextLog(ctx, "Couldn't initialize crash reporter: ", string(out))
		return errors.Wrap(err, "initializing crash reporter")
	}

	return setUpCrashTest(ctx, &p)
}

type crashAndStash struct {
	crashDir string
	stashDir string
}

// setUpParams is a collection of parameters to setUpCrashTest.
type setUpParams struct {
	inProgDir string
	// crashDirs is a list of all crash directories, along with the directories to which we should stash them.
	crashDirs        []crashAndStash
	rebootPersistDir string
	senderPausePath  string
	senderProcName   string
	mockSendingPath  string
	filterInPath     string
	sendRecordDir    string
	isDevImageTest   bool
	setConsent       bool
	setMockConsent   bool
	rebootTest       bool
	filterIn         string
	chrome           *chrome.Chrome
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

// CreatePerUserConsent creates the per-user consent file with the specified state.
func CreatePerUserConsent(ctx context.Context, enable bool) error {
	dirs, err := GetDaemonStoreConsentDirs(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get daemon store consent dirs")
	}
	// Set consent for all active dirs.
	for _, d := range dirs {
		f := filepath.Join(d, daemonStoreConsentName)
		contents := "0"
		if enable {
			contents = "1"
		}
		if err := ioutil.WriteFile(f, []byte(contents), 0644); err != nil {
			return errors.Wrapf(err, "failed writing consent-enabled file %s", f)
		}
	}
	return nil
}

// RemovePerUserConsent deletes the per-user consent files so that we fall back to device policy state.
func RemovePerUserConsent(ctx context.Context) error {
	dirs, err := GetDaemonStoreConsentDirs(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get daemon store consent dirs")
	}
	// Clear per-user consent for all active dirs.
	var firstErr error
	for _, d := range dirs {
		f := filepath.Join(d, daemonStoreConsentName)
		if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
			testing.ContextLogf(ctx, "Error removing consent-enabled file %s: %v", f, err)
			if firstErr == nil {
				firstErr = errors.Wrapf(err, "failed removing consent-enabled file %s", f)
			}
		}
	}
	return firstErr
}

// setUpCrashTest is a helper function for SetUpCrashTest. We need
// this as a separate function for testing.
func setUpCrashTest(ctx context.Context, p *setUpParams) (retErr error) {
	defer func() {
		if retErr != nil {
			tearDownCrashTest(ctx, &tearDownParams{
				inProgDir:        p.inProgDir,
				crashDirs:        p.crashDirs,
				rebootPersistDir: p.rebootPersistDir,
				senderPausePath:  p.senderPausePath,
				mockSendingPath:  p.mockSendingPath,
				filterInPath:     p.filterInPath,
			})
		}
	}()

	if p.filterIn == "" {
		if err := disableCrashFiltering(p.filterInPath); err != nil {
			return errors.Wrap(err, "couldn't disable crash filtering")
		}
	} else {
		if err := enableCrashFiltering(ctx, p.filterInPath, p.filterIn); err != nil {
			return errors.Wrap(err, "couldn't enable crash filtering")
		}
	}

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
		return errors.Wrapf(err, "couldn't write sender pause file %s", p.senderPausePath)
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
		return errors.Wrapf(err, "couldn't enable mock sending with path %s", p.mockSendingPath)
	}
	if err := resetSendRecords(p.sendRecordDir); err != nil {
		return errors.Wrapf(err, "couldn't reset send records at dir %s", p.sendRecordDir)
	}

	// Move all crashes into stash directory so a full directory won't stop
	// us from saving a new crash report, and so that we don't improperly
	// interpret preexisting crashes as being created during the test.
	for _, crashAndStash := range p.crashDirs {
		if err := moveAllCrashesTo(crashAndStash.crashDir, crashAndStash.stashDir); err != nil && !os.IsNotExist(err) {
			return errors.Wrapf(err, "couldn't stash crashes from %s to %s", crashAndStash.crashDir, crashAndStash.stashDir)
		}
	}

	// We must set mock consent _after_ stashing crashes, or we'll stash
	// the mock consent for reboot tests.
	if p.setMockConsent {
		mockConsentPath := filepath.Join(p.inProgDir, mockConsentFile)
		if err := ioutil.WriteFile(mockConsentPath, nil, 0644); err != nil {
			return errors.Wrapf(err, "failed writing mock consent file %s", mockConsentPath)
		}
		if p.rebootTest {
			mockConsentPersistent := filepath.Join(p.rebootPersistDir, mockConsentFile)
			if err := ioutil.WriteFile(mockConsentPersistent, []byte(rebootPersistenceCount), 0644); err != nil {
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
		filePath = filepath.Join(p.rebootPersistDir, crashTestInProgressFile)
		if err := ioutil.WriteFile(filePath, []byte(rebootPersistenceCount), 0644); err != nil {
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

// tearDownOption is a self-referential function can be used to configure crash tests.
// See https://commandcenter.blogspot.com.au/2014/01/self-referential-functions-and-design.html
// for details about this pattern.
type tearDownOption func(p *tearDownParams)

// TearDownCrashTest undoes the work of SetUpCrashTest. We assume here that the
// set of active sessions hasn't changed since SetUpCrashTest was called for
// the purpose of restoring the per-user-cryptohome crash directories.
func TearDownCrashTest(ctx context.Context, opts ...tearDownOption) error {
	var firstErr error

	crashDirs := []crashAndStash{
		{SystemCrashDir, systemCrashStash},
		{LocalCrashDir, localCrashStash},
		{ClobberCrashDir, clobberCrashStash},
	}

	p := tearDownParams{
		inProgDir:        crashTestInProgressDir,
		crashDirs:        crashDirs,
		rebootPersistDir: rebootPersistDir,
		senderPausePath:  senderPausePath,
		mockSendingPath:  mockSendingPath,
		filterInPath:     FilterInPath,
	}

	for _, opt := range opts {
		opt(&p)
	}

	// This could return a different list of paths then the original setup
	// if a session has started or ended in the meantime. If a new session
	// has started then the restore will just become a no-op, but if a
	// session has ended we won't restore the crash files inside that
	// session's cryptohome. We can't do much about this since we can't
	// touch a cryptohome while that user isn't logged in.
	daemonStorePaths, err := GetDaemonStoreCrashDirs(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Failed to get daemon store crash dirs: ", err)
		if firstErr == nil {
			firstErr = errors.Wrap(err, "failed to get daemon store crash directories")
		}
	}
	for _, path := range daemonStorePaths {
		p.crashDirs = append(p.crashDirs, crashAndStash{path, path + ".real"})
	}

	if err := tearDownCrashTest(ctx, &p); err != nil {
		testing.ContextLog(ctx, "Failed to tearDownCrashTest: ", err)
		if firstErr == nil {
			firstErr = errors.Wrap(err, "couldn't tear down crash test")
		}
	}
	// The user crash directory should always be owned by chronos not root. The
	// unit tests don't run as root and can't chown, so skip this in tests.
	// Only do this if the local crash dir actually exists.
	if _, err := os.Stat(LocalCrashDir); err == nil {
		if err := os.Chown(LocalCrashDir, int(sysutil.ChronosUID), crashUserAccessGID); err != nil {
			testing.ContextLogf(ctx, "Couldn't chown %s: %v", LocalCrashDir, err)
			if firstErr == nil {
				firstErr = errors.Wrapf(err, "couldn't chown %s", LocalCrashDir)
			}
		}
	}
	return firstErr
}

// tearDownParams is a collection of parameters to tearDownCrashTest.
type tearDownParams struct {
	inProgDir string
	// crashDirs is a list of all crash directories, along with the directories to which we stashed them in setUp
	crashDirs        []crashAndStash
	rebootPersistDir string
	senderPausePath  string
	mockSendingPath  string
	filterInPath     string
}

// tearDownCrashTest is a helper function for TearDownCrashTest. We need
// this as a separate function for testing.
func tearDownCrashTest(ctx context.Context, p *tearDownParams) error {
	var firstErr error

	// If crashTestInProgressFile does not exist, something else already removed the file
	// or it was never created (See SetUpDevImageCrashTest).
	// Well, whatever, we're in the correct state now (the file is gone).
	filePath := filepath.Join(p.inProgDir, crashTestInProgressFile)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		testing.ContextLogf(ctx, "Error removing crash test in progress file %s: %v", filePath, err)
		if firstErr == nil {
			firstErr = errors.Wrapf(err, "removing crash test in progress file %s", filePath)
		}
	}
	filePath = filepath.Join(p.rebootPersistDir, crashTestInProgressFile)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		testing.ContextLogf(ctx, "Error removing persistent crash test in progress file %s: %v", filePath, err)
		if firstErr == nil {
			firstErr = errors.Wrapf(err, "removing persistent crash test in progress file %s", filePath)
		}
	}

	if err := disableCrashFiltering(p.filterInPath); err != nil {
		testing.ContextLog(ctx, "Couldn't disable crash filtering: ", err)
		if firstErr == nil {
			firstErr = errors.Wrap(err, "couldn't disable crash filtering")
		}
	}

	// Clean up stash directories
	for _, crashAndStash := range p.crashDirs {
		if err := cleanUpStashDir(crashAndStash.stashDir, crashAndStash.crashDir); err != nil {
			testing.ContextLogf(ctx, "Error cleaning up stash dir %s (real dir %s): %v", crashAndStash.stashDir, crashAndStash.crashDir, err)
			if firstErr == nil {
				firstErr = errors.Wrapf(err, "couldn't clean up stash dir %s to %s", crashAndStash.stashDir, crashAndStash.crashDir)
			}
		}
	}

	if err := disableMockSending(p.mockSendingPath); err != nil {
		testing.ContextLogf(ctx, "Error disabling mock sending with path %s: %v", p.mockSendingPath, err)
		if firstErr == nil {
			firstErr = err
		}
	}

	mockConsentPath := filepath.Join(p.inProgDir, mockConsentFile)
	if err := os.Remove(mockConsentPath); err != nil && !os.IsNotExist(err) {
		testing.ContextLogf(ctx, "Error removing mock consent file %s: %v", mockConsentPath, err)
		if firstErr == nil {
			firstErr = errors.Wrapf(err, "couldn't remove mock consent file %s", mockConsentPath)
		}
	}
	mockConsentPersistent := filepath.Join(p.rebootPersistDir, mockConsentFile)
	if err := os.Remove(mockConsentPersistent); err != nil && !os.IsNotExist(err) {
		testing.ContextLogf(ctx, "Error removing persistent mock consent file %s: %v", mockConsentPersistent, err)
		if firstErr == nil {
			firstErr = errors.Wrapf(err, "couldn't remove persistent mock consent file %s", mockConsentPersistent)
		}
	}

	if err := os.Remove(p.senderPausePath); err != nil && !os.IsNotExist(err) {
		testing.ContextLogf(ctx, "Error removing sender pause file %s: %v", p.senderPausePath, err)
		if firstErr == nil {
			firstErr = errors.Wrapf(err, "couldn't remove sender pause file %s", p.senderPausePath)
		}
	}

	return firstErr
}
