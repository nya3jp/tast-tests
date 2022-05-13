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
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OpenAndroidApp,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests Assistant open Android app feature",
		Contacts:     []string{"updowndota@chromium.org", "xiaohuic@chromium.org", "assistive-eng@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Fixture:      "assistantWithArc",
		Timeout:      3 * time.Minute,
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
	fixtData := s.FixtValue().(*assistant.FixtData)
	cr := fixtData.Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

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
	}, &testing.PollOptions{Timeout: 3 * time.Minute}); err != nil {
		return errors.Wrap(err, "failed to wait for ARC package list to be refreshed")
	}
	return nil
}
