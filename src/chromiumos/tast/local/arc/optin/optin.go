// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package optin provides set of util functions used to control ARC provisioning.
package optin

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/testing"
)

const (
	// OptinTimeout is the maximum amount of time that Optin is expected to take.
	OptinTimeout = 5 * time.Minute

	// PlayStoreCloseTimeout is the timeout value waiting for Play Store window to show up
	// and then close it after optin.
	PlayStoreCloseTimeout = 1 * time.Minute
)

// arcApp maps ArcAppDict definition
// https://cs.chromium.org/chromium/src/chrome/common/extensions/api/autotest_private.idl
type arcApp struct {
	Name                 string  `json:"name"`
	PackageName          string  `json:"packageName"`
	Activity             string  `json:"activity"`
	IntentURI            string  `json:"intentUri"`
	IconResourceID       string  `json:"iconResourceId"`
	LastLaunchTime       float64 `json:"lastLaunchTime"`
	InstallTime          float64 `json:"installTime"`
	Sticky               bool    `json:"sticky"`
	NotificationsEnabled bool    `json:"notificationsEnabled"`
	Ready                bool    `json:"ready"`
	Suspended            bool    `json:"suspended"`
	ShowInLauncher       bool    `json:"showInLauncher"`
	Shortcut             bool    `json:"shortcut"`
	Launchable           bool    `json:"launchable"`
}

const (
	errorPageLoaded = "appWindow.contentWindow.document.querySelector('#error.section:not([hidden])')"
	errorMessage    = "appWindow.contentWindow.document.getElementById('error-message')?.innerText"
)

// SetPlayStoreEnabled is a wrapper for chrome.autotestPrivate.setPlayStoreEnabled.
func SetPlayStoreEnabled(ctx context.Context, tconn *chrome.TestConn, enabled bool) error {
	return tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.setPlayStoreEnabled)`, enabled)
}

// waitForTerms waits for terms of service page to load in optin dialog.
func waitForTerms(ctx context.Context, conn *chrome.Conn) error {
	for _, condition := range []string{
		"port != null",
		"termsPage != null",
		fmt.Sprintf("termsPage.isManaged_ || termsPage.state_ == LoadState.LOADED || %s", errorPageLoaded),
	} {
		if err := conn.WaitForExpr(ctx, condition); err != nil {
			return err
		}
	}

	var msg string
	if err := conn.Eval(ctx, fmt.Sprintf("%s?%s:''", errorPageLoaded, errorMessage), &msg); err == nil && msg != "" {
		return errors.New(msg)
	}

	return nil
}

// sleepAndPressRetryButton sleeps for 5 seconds and presses the retry button
func sleepAndPressRetryButton(ctx context.Context, conn *chrome.Conn) error {
	// Sleep briefly before retrying.
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep for re-attempt")
	}

	if err := conn.Eval(ctx, "appWindow.contentWindow.document.getElementById('button-retry').click()", nil); err != nil {
		return errors.Wrap(err, "failed to press the retry button")
	}

	return nil
}

// withRetry returns a function that attempts to perform an action, if action fails, presses retry button
// and attempts to perform the action again until action succeeds or until attempt count exceeds maxAttempts.
func withRetry(actionName string, action func(context.Context, *chrome.Conn) error, maxAttempts int, conn *chrome.Conn) uiauto.Action {
	isRecoveryNeeded := false
	return uiauto.Retry(maxAttempts, uiauto.NamedAction(actionName, func(ctx context.Context) error {
		if isRecoveryNeeded {
			if err := sleepAndPressRetryButton(ctx, conn); err != nil {
				return err
			}
		}

		err := action(ctx, conn)
		if err == nil {
			return nil
		}

		isRecoveryNeeded = true

		return err
	}))
}

// waitForOptin waits for optin to complete either with an error or by closing the window.
func waitForOptin(ctx context.Context, conn *chrome.Conn) error {
	if err := conn.WaitForExpr(ctx, fmt.Sprintf("!appWindow || %s", errorPageLoaded)); err != nil {
		return errors.Wrap(err, "failed to wait for optin completion")
	}

	return ensureNoError(ctx, conn)
}

// ensureNoError ensures that the Play Store window doesn't display an error.
func ensureNoError(ctx context.Context, conn *chrome.Conn) error {
	var errMsg string
	if err := conn.Eval(ctx, fmt.Sprintf("!appWindow ? '' : %s  ?? 'Unknown error'", errorMessage), &errMsg); err != nil {
		return errors.Wrap(err, "failed to evaluate optin result")
	}

	if errMsg != "" {
		return errors.New(errMsg)
	}

	return nil
}

// EnsureNoPlayStoreError ensures that the Play Store window doesn't display an error.
func EnsureNoPlayStoreError(ctx context.Context, cr *chrome.Chrome) error {
	bgURL := chrome.ExtensionBackgroundPageURL(apps.PlayStore.ID)
	found, err := cr.IsTargetAvailable(ctx, chrome.MatchTargetURL(bgURL))

	if err != nil {
		return errors.Wrap(err, "failed to check Play Store window")
	} else if !found {
		return nil
	}

	conn, err := newConnForPlayStore(ctx, cr)
	if err != nil {
		return err
	}
	defer conn.Close()
	return ensureNoError(ctx, conn)
}

// newConnForPlayStore opens a connection with Play Store extension page.
func newConnForPlayStore(ctx context.Context, cr *chrome.Chrome) (*chrome.Conn, error) {
	bgURL := chrome.ExtensionBackgroundPageURL(apps.PlayStore.ID)
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
	if err != nil {
		return nil, errors.Wrap(err, "failed to find Play Store page")
	}

	return conn, nil
}

// FindOptInExtensionPageAndAcceptTerms finds the opt-in extension page, optins if verified,
// and optionally waits for completion.
func FindOptInExtensionPageAndAcceptTerms(ctx context.Context, cr *chrome.Chrome, maxAttempts int, wait bool) error {
	conn, err := newConnForPlayStore(ctx, cr)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := withRetry("load terms of service", waitForTerms, maxAttempts, conn)(ctx); err != nil {
		return errors.Wrap(err, "failed to load terms of service")
	}

	if err := conn.Eval(ctx, "termsPage.onAgree()", nil); err != nil {
		return errors.Wrap(err, "failed to execute 'termsPage.onAgree()'")
	}

	if wait {
		return waitForOptin(ctx, conn)
	}

	return nil
}

// Perform steps through opt-in flow and waits for it to complete.
func Perform(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) error {
	ctx, cancel := context.WithTimeout(ctx, OptinTimeout)
	defer cancel()

	SetPlayStoreEnabled(ctx, tconn, true)

	if err := FindOptInExtensionPageAndAcceptTerms(ctx, cr, 2 /*maxAttempts*/, true /*wait*/); err != nil {
		return err
	}

	// TODO(niwa): Check if we still need to handle non-tos_needed case.
	return nil
}

// PerformWithRetry steps through opt-in flow, waits for it to complete, and re-attempts in case of failure.
func PerformWithRetry(ctx context.Context, cr *chrome.Chrome, maxAttempts int) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create test API connection")
	}

	attempts := 1
	for {
		err := Perform(ctx, cr, tconn)
		if err == nil {
			break
		}

		if err := DumpLogCat(ctx, strconv.Itoa(attempts)); err != nil {
			testing.ContextLog(ctx, "WARNING: Failed to dump logcat: ", err)
		}

		if attempts >= maxAttempts {
			return err
		}

		testing.ContextLog(ctx, "Retrying optin, previous attempt failed: ", err)
		attempts = attempts + 1

		// Opt out.
		if err := SetPlayStoreEnabled(ctx, tconn, false); err != nil {
			return errors.Wrap(err, "failed to optout")
		}
	}

	return nil
}

// PerformAndClose performs opt-in with retries, and then closes the play store window.
func PerformAndClose(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) error {
	ctx, cancel := context.WithTimeout(ctx, OptinTimeout+PlayStoreCloseTimeout)
	defer cancel()

	maxAttempts := 2
	if err := PerformWithRetry(ctx, cr, maxAttempts); err != nil {
		return errors.Wrap(err, "failed to perform Play Store optin")
	}
	if err := WaitForPlayStoreShown(ctx, tconn, time.Minute); err != nil {
		// When we get here, play store is probably not shown, or it failed to be detected.
		// Just log the message and continue.
		testing.ContextLogf(ctx, "Play store window is not detected: %v; continue to try to close it", err)
	}

	if err := ClosePlayStore(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to close Play Store")
	}
	return nil
}

// ClosePlayStore closes the Play Store app.
func ClosePlayStore(ctx context.Context, tconn *chrome.TestConn) error {
	// Attempt to find the Play Store window by it's package name, or ID
	// since the package name may not always be set.
	var window *ash.Window
	err := testing.Poll(ctx, func(ctx context.Context) error {
		var findWindowErr error
		if window, findWindowErr = ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
			return w.ARCPackageName == "com.android.vending" || w.AppID == apps.PlayStore.ID
		}); findWindowErr != nil {
			return errors.New("Play Store window is not yet open")
		}
		return nil
	}, &testing.PollOptions{Timeout: 60 * time.Second})

	// CloseWindow is done only when Play Store is open and sometimes when it
	// remains in minimized state this method just returns nil.
	if err == nil {
		testing.ContextLog(ctx, "Play store window is open and hence closing")
		if err := window.CloseWindow(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to close app window")
		}
	}
	return nil
}

// WaitForPlayStoreReady waits for Play Store app to be ready.
func WaitForPlayStoreReady(ctx context.Context, tconn *chrome.TestConn) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var app arcApp
		if err := tconn.Call(ctx, &app, `tast.promisify(chrome.autotestPrivate.getArcApp)`, apps.PlayStore.ID); err != nil {
			return testing.PollBreak(err)
		}
		if !app.Ready {
			return errors.New("Play Store app is not yet ready")
		}
		return nil
	}, &testing.PollOptions{Timeout: OptinTimeout}); err != nil {
		return errors.Wrap(err, "failed to wait for Play Store app to become ready")
	}
	return nil
}

// LaunchAndWaitForPlayStore launches Play Store and ensures that it doesn't display an error.
func LaunchAndWaitForPlayStore(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := apps.Launch(ctx, tconn, apps.PlayStore.ID); err != nil {
		return errors.Wrap(err, "failed to launch Play Store")
	}

	if err := WaitForPlayStoreShown(ctx, tconn, timeout); err != nil {
		return errors.Wrap(err, "failed to wait for Play Store to show up")
	}

	if err := EnsureNoPlayStoreError(ctx, cr); err != nil {
		return errors.Wrap(err, "Play Store launched with an error")
	}

	return nil
}

// WaitForPlayStoreShown waits for Play Store window to be shown.
func WaitForPlayStoreShown(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration) error {
	return ash.WaitForApp(ctx, tconn, apps.PlayStore.ID, timeout)
}

// GetPlayStoreState is a wrapper for chrome.autotestPrivate.getPlayStoreState.
func GetPlayStoreState(ctx context.Context, tconn *chrome.TestConn) (map[string]bool, error) {
	state := make(map[string]bool)
	if err := tconn.Call(ctx, &state, `tast.promisify(chrome.autotestPrivate.getPlayStoreState)`); err != nil {
		return nil, errors.Wrap(err, "failed running autotestPrivate.getPlayStoreState")
	}
	return state, nil
}

// DumpLogCat saves logcat to test output directory.
func DumpLogCat(ctx context.Context, filesuffix string) error {
	cmd := testexec.CommandContext(ctx, "/usr/sbin/android-sh", "-c", "/system/bin/logcat -d")
	log, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to pull logcat")
	}

	fileName := fmt.Sprintf("logcat_%s.txt", filesuffix)
	if err := writeLog(ctx, fileName, log); err != nil {
		return errors.Wrap(err, "failed to write logcat dump")
	}
	return nil
}

// writeLog writes the log to test output directory.
func writeLog(ctx context.Context, fileName string, data []byte) error {
	dir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("failed to get out dir")
	}

	logPath := filepath.Join(dir, fileName)
	err := ioutil.WriteFile(logPath, data, 0644)
	if err != nil {
		return errors.Wrapf(err, "failed to save %q", fileName)
	}
	testing.ContextLog(ctx, "Saved ", fileName)
	return nil
}
