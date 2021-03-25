// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package optin provides set of util functions used to control ARC provisioning.
package optin

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
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

// SetPlayStoreEnabled is a wrapper for chrome.autotestPrivate.setPlayStoreEnabled.
func SetPlayStoreEnabled(ctx context.Context, tconn *chrome.TestConn, enabled bool) error {
	return tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.setPlayStoreEnabled)`, enabled)
}

// FindOptInExtensionPageAndAcceptTerms finds the opt-in extension page, optins if verified,
// and optionally waits for completion.
func FindOptInExtensionPageAndAcceptTerms(ctx context.Context, cr *chrome.Chrome, wait bool) error {
	bgURL := chrome.ExtensionBackgroundPageURL(apps.PlayStore.ID)
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
	if err != nil {
		return errors.Wrapf(err, "failed to find optin extension page %v", bgURL)
	}
	defer conn.Close()

	const (
		errorPageLoaded = "appWindow.contentWindow.document.querySelector('#error.section:not([hidden])')"
		errorMessage    = "appWindow.contentWindow.document.getElementById('error-message')?.innerText"
	)

	for _, condition := range []string{
		"port != null",
		"termsPage != null",
		fmt.Sprintf("termsPage.isManaged_ || termsPage.state_ == LoadState.LOADED || %s", errorPageLoaded),
	} {
		if err := conn.WaitForExpr(ctx, condition); err != nil {
			return errors.Wrap(err, "failed to find terms of service page")
		}
	}

	var msg string
	if err := conn.Eval(ctx, fmt.Sprintf("%s?%s:''", errorPageLoaded, errorMessage), &msg); err == nil && msg != "" {
		return errors.New(msg)
	}

	if err := conn.Eval(ctx, "termsPage.onAgree()", nil); err != nil {
		return errors.Wrap(err, "failed to execute 'termsPage.onAgree()'")
	}

	if wait {
		if err := conn.WaitForExpr(ctx, fmt.Sprintf("!appWindow || %s", errorPageLoaded)); err != nil {
			return errors.Wrap(err, "failed to wait for optin completion")
		}

		var msg string
		if err := conn.Eval(ctx, fmt.Sprintf("!appWindow ? '' : %s  ?? 'Unknown error'", errorMessage), &msg); err != nil {
			return errors.Wrap(err, "failed to evaluate optin result")
		}

		if msg != "" {
			return errors.New(msg)
		}
	}

	return nil
}

// Perform steps through opt-in flow and waits for it to complete.
func Perform(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) error {
	ctx, cancel := context.WithTimeout(ctx, OptinTimeout)
	defer cancel()

	SetPlayStoreEnabled(ctx, tconn, true)

	if err := FindOptInExtensionPageAndAcceptTerms(ctx, cr, true); err != nil {
		return err
	}

	// TODO(niwa): Check if we still need to handle non-tos_needed case.
	return nil
}

// PerformAndClose performs opt-in, and then close the play store window.
func PerformAndClose(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) error {
	ctx, cancel := context.WithTimeout(ctx, OptinTimeout+PlayStoreCloseTimeout)
	defer cancel()

	if err := Perform(ctx, cr, tconn); err != nil {
		return errors.Wrap(err, "failed to perform Play Store optin")
	}
	if err := WaitForPlayStoreShown(ctx, tconn, time.Minute); err != nil {
		// When we get here, play store is probably not shown, or it failed to be detected.
		// Just log the message and continue.
		testing.ContextLogf(ctx, "Play store window is not detected: %v; continue to try to close it", err)
	}

	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		// Log the message and continue.
		testing.ContextLogf(ctx, "Failed to close Play Store window: %v; continue to check if it has been closed", err)
	}

	// Additional attempt to Close if Play Store remains open before returning error.
	// This logic takes care when WaitForPlayStoreShown didn't ensure window is open & apps.Close didn't close the app.
	if err := ash.WaitForAppClosed(ctx, tconn, apps.PlayStore.ID); err != nil {
		testing.ContextLogf(ctx, "Failed to close Play Store window: %v; continue to close", err)
		if window, err := ash.GetARCAppWindowInfo(ctx, tconn, "com.android.vending"); err != nil {
			errors.Wrap(err, "failed to get window info")
		} else if err := window.CloseWindow(ctx, tconn); err != nil {
			errors.Wrap(err, "failed to close app window")
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
	}, &testing.PollOptions{Timeout: 90 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for Play Store app to become ready")
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
