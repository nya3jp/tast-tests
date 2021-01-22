// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package optin provides set of util functions used to control ARC provisioning.
package optin

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

// OptinTimeout is the maximum amount of time that Optin is expected to take.
const OptinTimeout = 5 * time.Minute

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

	if err := conn.Eval(ctx, "termsPage.onAgree()", nil); err != nil {
		return errors.Wrap(err, "failed to execute 'termsPage.onAgree()'")
	}

	if wait {
		if err := conn.WaitForExpr(ctx, "!appWindow"); err != nil {
			return errors.Wrap(err, "failed to wait for '!appWindow'")
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
		return errors.Wrap(err, "failed to find optin extension page")
	}

	// TODO(niwa): Check if we still need to handle non-tos_needed case.
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
func WaitForPlayStoreShown(ctx context.Context, tconn *chrome.TestConn) error {
	return ash.WaitForApp(ctx, tconn, apps.PlayStore.ID)
}

// GetPlayStoreState is a wrapper for chrome.autotestPrivate.getPlayStoreState.
func GetPlayStoreState(ctx context.Context, tconn *chrome.TestConn) (map[string]bool, error) {
	state := make(map[string]bool)
	if err := tconn.Call(ctx, &state, `tast.promisify(chrome.autotestPrivate.getPlayStoreState)`); err != nil {
		return nil, errors.Wrap(err, "failed running autotestPrivate.getPlayStoreState")
	}
	return state, nil
}
