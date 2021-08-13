// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package assistant supports interaction with Assistant service.
package assistant

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OpenAndroidApp,
		Desc:         "Tests Assistant open Android app feature",
		Contacts:     []string{"updowndota@chromium.org", "xiaohuic@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		VarDeps:      []string{"assistant.username", "assistant.password"},
		Timeout:      chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

// OpenAndroidApp tests the open Android apps feature of the Assistant.
func OpenAndroidApp(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(
		ctx,
		chrome.ARCEnabled(),
		chrome.GAIALogin(chrome.Creds{
			User: s.RequiredVar("assistant.username"),
			Pass: s.RequiredVar("assistant.password"),
		}),
		chrome.EnableFeatures("AssistantAppSupport"),
		chrome.ExtraArgs(
			"--arc-disable-app-sync",
			"--arc-disable-locale-sync",
			"--arc-play-store-auto-update=off"),
		assistant.VerboseLogging(),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Enable the Assistant and wait for the ready signal.
	if err := assistant.EnableAndWaitForReady(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Assistant: ", err)
	}
	defer func() {
		if err := assistant.Cleanup(ctx, s.HasError, cr, tconn); err != nil {
			s.Fatal("Failed to disable Assistant: ", err)
		}
	}()

	s.Log("Waiting for ARC package list initial refreshed")
	if err := waitForArcPackageListInitialRefreshed(ctx, s, tconn); err != nil {
		s.Fatal("Failed to wait for ARC package to become ready: ", err)
	}

	s.Log("Sending open Play Store query to the Assistant")
	if _, err := assistant.SendTextQuery(ctx, tconn, "open Play Store"); err != nil {
		s.Fatal("Failed to send the Assistant text query: ", err)
	}

	s.Log("Waiting for Play Store window to be shown")
	if err := ash.WaitForApp(ctx, tconn, apps.PlayStore.ID, time.Minute); err != nil {
		s.Fatal("Play Store failed to launch: ", err)
	}
}

// waitForArcPackageListInitialRefreshed waits until the ARC package list is refreshed.
func waitForArcPackageListInitialRefreshed(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var refreshed bool
		if err := tconn.Eval(ctx, `tast.promisify(chrome.autotestPrivate.isArcPackageListInitialRefreshed)()`, &refreshed); err != nil {
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
