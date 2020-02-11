// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// playStoreAppID is app id of Play Store app.
const playStoreAppID = "cnbgggchhmkkdmeppjobngjoejnihlei"

func init() {
	testing.AddTest(&testing.Test{
		Func:         AssistantOpenAndroidApp,
		Desc:         "Tests Assistant open Android app feature",
		Contacts:     []string{"updowndota@chromium.org", "xiaohuic@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal", "android_both"},
		Vars:         []string{"assistant.username", "assistant.password"},
	})
}

// AssistantOpenAndroidApp tests the open Android apps feature of the Assistant.
func AssistantOpenAndroidApp(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(
		ctx,
		chrome.ARCEnabled(),
		chrome.Auth(s.RequiredVar("assistant.username"), s.RequiredVar("assistant.password"), ""),
		chrome.ExtraArgs("--enable-features=AssistantAppSupport", "--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off"),
		chrome.GAIALogin(),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Start Assistant service.
	if err := assistant.Enable(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Assistant: ", err)
	}

	// TODO(b/129896357): Replace the waiting logic once Libassistant has a reliable signal for
	// its readiness to watch for in the signed out mode.
	s.Log("Waiting for Assistant to be ready to answer queries")
	if err := assistant.WaitForServiceReady(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for Libassistant to become ready: ", err)
	}

	s.Log("Waiting for ARC package list initial refreshed")
	waitForArcPackageListInitialRefreshed(ctx, tconn, s)

	s.Log("Sending open Play Store query to the Assistant")
	assistant.SendTextQuery(ctx, tconn, "open Play Store")

	s.Log("Waiting for Play Store window to be shown")
	if err := optin.WaitForPlayStoreShown(ctx, tconn); err != nil {
		s.Fatal("Play Store failed to launch: ", err)
	}
}

// waitForArcPackageListInitialRefreshed waits until the ARC package list is refreshed.
func waitForArcPackageListInitialRefreshed(ctx context.Context, tconn *chrome.Conn, s *testing.State) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var refreshed bool
		if err := tconn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.isArcPackageListInitialRefreshed)()`, &refreshed); err != nil {
			return testing.PollBreak(err)
		}
		if !refreshed {
			return errors.New("ARC package list is not refreshed yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: 60 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for ARC package list to be refreshed")
	}
	return nil
}
