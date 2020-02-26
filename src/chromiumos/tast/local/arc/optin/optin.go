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

// OptinTimeout is the maximum amount of time that Optin is expected to take.
const OptinTimeout = time.Minute

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
	expr := fmt.Sprintf(`tast.promisify(chrome.autotestPrivate.setPlayStoreEnabled)(%t)`, enabled)
	return tconn.EvalPromise(ctx, expr, nil)
}

// Perform steps through opt-in flow and wait for it to complete.
func Perform(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) error {
	SetPlayStoreEnabled(ctx, tconn, true)

	bgURL := chrome.ExtensionBackgroundPageURL(apps.PlayStore.ID)
	conn, err := cr.NewConnForTarget(ctx, func(t *chrome.Target) bool {
		return t.URL == bgURL
	})
	if err != nil {
		return errors.Wrapf(err, "failed to find %v", bgURL)
	}
	defer conn.Close()

	for _, condition := range []string{
		"port != null",
		"termsPage != null",
		"termsPage.isManaged_ || termsPage.state_ == LoadState.LOADED",
	} {
		if err := conn.WaitForExpr(ctx, condition); err != nil {
			return errors.Wrapf(err, "failed to wait for %v", condition)
		}
	}

	if err := conn.Exec(ctx, "termsPage.onAgree()"); err != nil {
		return errors.Wrap(err, "failed to execute 'termsPage.onAgree()'")
	}

	if err := conn.WaitForExpr(ctx, "!appWindow"); err != nil {
		return errors.Wrap(err, "failed to wait for '!appWindow'")
	}

	// TODO(niwa): Check if we still need to handle non-tos_needed case.
	return nil
}

// WaitForPlayStoreReady waits for Play Store app to be ready.
func WaitForPlayStoreReady(ctx context.Context, tconn *chrome.TestConn) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var app arcApp
		expr := fmt.Sprintf(`tast.promisify(chrome.autotestPrivate.getArcApp)('%s')`, apps.PlayStore.ID)
		if err := tconn.EvalPromise(ctx, expr, &app); err != nil {
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
func WaitForPlayStoreShown(ctx context.Context, tconn *chrome.TestConn) error {
	return ash.WaitForApp(ctx, tconn, apps.PlayStore.ID)
}
