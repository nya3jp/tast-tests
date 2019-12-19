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
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

// crashUserAccessGID is the GID for crash-user-access, as defined in
// third_party/eclass-overlay/profiles/base/accounts/group/crash-user-access.
const crashUserAccessGID = 420

// SetConsent enables or disables metrics consent, based on the value of |consent|.
// Pre: cr must point to a logged-in chrome session.
func SetConsent(ctx context.Context, cr *chrome.Chrome, consent bool) error {
	testing.ContextLogf(ctx, "SetConsent(%t)", consent)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "creating test API connection failed")
	}
	code := fmt.Sprintf("tast.promisify(chrome.autotestPrivate.setMetricsEnabled)(%t)", consent)
	if err := tconn.EvalPromise(ctx, code, nil); err != nil {
		return errors.Wrap(err, "running autotestPrivate.setMetricsEnabled failed")
	}
	err = testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat("/home/chronos/Consent To Send Stats"); os.IsNotExist(err) {
			return err
		} else if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to stat"))
		}
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second})
	return err
}

// moveAllCrashesTo moves crashes from |source| to |target|. This allows us to
// start crash tests with an empty spool directory, reducing risk of flakes if
// the dir is already full when the test starts.
func moveAllCrashesTo(source, target string) error {
	if err := os.MkdirAll(target, 0755); err != nil {
		return errors.Wrapf(err, "couldn't make stash crash dir %s", target)
	}
	files, err := ioutil.ReadDir(source)
	if err != nil {
		// Bubble this up so caller can check whether IsNotExist and behave accordingly.
		return err
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
type Option func(p *crashTestConfig)

// DevImage prevents the test library from indicating to the DUT that a crash
// test is in progress, allowing the test to complete with standard developer
// image behavior.
func DevImage() Option {
	return func(p *crashTestConfig) {
		p.isDevImageTest = true
	}
}

// WithConsent indicates that the test should enable metrics consent.
// Pre: cr should be a logged-in chrome session.
func WithConsent(cr *chrome.Chrome) Option {
	return func(p *crashTestConfig) {
		p.setConsent = true
		p.chrome = cr
	}
}

// SetUpCrashTest indicates that we are running a test that involves the crash
// reporting system (crash_reporter, crash_sender, or anomaly_detector).
// Returns a function to restore modified environment if no error occurred.
// Invoker should typically call the function by the defer statement for
// tearing down. If developer image behavior is required for the test, call
// SetUpDevImageCrashTest instead.
func SetUpCrashTest(ctx context.Context, opts ...Option) (func() error, error) {
	p := crashTestConfig{
		inProgDir:      crashTestInProgressDir,
		sysCrashDir:    SystemCrashDir,
		sysCrashStash:  systemCrashStash,
		userCrashDir:   LocalCrashDir,
		userCrashStash: localCrashStash,
		isDevImageTest: false,
		setConsent:     false,
		chrome:         nil,
	}
	for _, opt := range opts {
		opt(&p)
	}

	return setUpCrashTest(ctx, &p)
}

// SetUpDevImageCrashTest stashes away existing crash files to prevent tests which
// generate crashes from failing due to full crash directories. This function does
// not indicate to the DUT that a crash test is in progress, allowing the test to
// complete with standard developer image behavior. Returns a function to restore
// environment if no error occurred. Invoker should call that function by a defer
// statement for tearing down.
func SetUpDevImageCrashTest(ctx context.Context) (func() error, error) {
	return SetUpCrashTest(ctx, DevImage())
}

// crashTestConfig is a collection of parameters for the system setup during crash test.
type crashTestConfig struct {
	inProgDir      string
	sysCrashDir    string
	sysCrashStash  string
	userCrashDir   string
	userCrashStash string
	isDevImageTest bool
	setConsent     bool
	chrome         *chrome.Chrome
}

// setUpCrashTest is a helper function for SetUpCrashTest. We need
// this as a separate function for testing.
func setUpCrashTest(ctx context.Context, p *crashTestConfig) (tearDown func() error, retErr error) {
	tearDown = func() error {
		var firstErr error
		if err := tearDownCrashTest(p); err != nil && firstErr == nil {
			firstErr = err
		}
		// The user crash directory should always be owned by chronos not root. The
		// unit tests don't run as root and can't chown, so skip this in tests.
		if err := os.Chown(LocalCrashDir, int(sysutil.ChronosUID), crashUserAccessGID); err != nil && !os.IsNotExist(err) && firstErr == nil {
			firstErr = errors.Wrapf(err, "couldn't chown %s", LocalCrashDir)
		}
		return firstErr
	}
	defer func() {
		if retErr != nil {
			tearDown()
		}
	}()

	if p.setConsent {
		if err := SetConsent(ctx, p.chrome, true); err != nil {
			return nil, errors.Wrap(err, "couldn't enable metrics consent")
		}
	}

	// Move all crashes into stash directory so a full directory won't stop
	// us from saving a new crash report.
	if err := moveAllCrashesTo(p.sysCrashDir, p.sysCrashStash); err != nil && !os.IsNotExist(err) {
		return tearDown, nil
	}
	if err := moveAllCrashesTo(p.userCrashDir, p.userCrashStash); err != nil && !os.IsNotExist(err) {
		return tearDown, nil
	}

	// If the test is meant to run with developer image behavior, return here to
	// avoid creating the directory that indicates a crash test is in progress.
	if p.isDevImageTest {
		return tearDown, nil
	}

	if err := os.MkdirAll(p.inProgDir, 0755); err != nil {
		return nil, errors.Wrapf(err, "could not make directory %v", p.inProgDir)
	}

	filePath := filepath.Join(p.inProgDir, crashTestInProgressFile)
	if err := ioutil.WriteFile(filePath, nil, 0644); err != nil {
		return nil, errors.Wrapf(err, "could not create %v", filePath)
	}
	return tearDown, nil
}

func cleanUpStashDir(stashDir, realDir string) error {
	// Stash dir should exist, so error if it doesn't.
	if err := moveAllCrashesTo(stashDir, realDir); err != nil {
		return err
	}
	if err := os.Remove(stashDir); err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrapf(err, "couldn't remove stash dir: %v", stashDir)
		}
	}
	return nil
}

// tearDownCrashTest is a helper function for TearDownCrashTest. We need
// this as a separate function for testing.
func tearDownCrashTest(p *crashTestConfig) error {
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
	if err := cleanUpStashDir(p.userCrashStash, p.userCrashDir); err != nil && firstErr == nil {
		firstErr = err
	}

	return firstErr
}
