// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DisableScreenshotsHotkey,
		// TODO(crbug.com/1125556): check whether screenshot can be taken by extensions APIs.
		Desc: "Behavior of the DisableScreenshots policy, check whether screenshot can be taken by pressing hotkeys",
		Contacts: []string{
			"lamzin@google.com", // Test port author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func DisableScreenshotsHotkey(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	defer func() {
		if err := screenshot.RemoveScreenshots(); err != nil {
			s.Error("Failed to remove screenshots after all tests: ", err)
		}
	}()

	// Connect to Test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	for _, tc := range []struct {
		name             string
		value            []policy.Policy
		wantAllowed      bool
		wantNotification string
	}{
		{
			name:             "true",
			value:            []policy.Policy{&policy.DisableScreenshots{Val: true}},
			wantAllowed:      false,
			wantNotification: "Can't capture content",
		},
		{
			name:             "false",
			value:            []policy.Policy{&policy.DisableScreenshots{Val: false}},
			wantAllowed:      true,
			wantNotification: "Screenshot taken",
		},
		{
			name:             "unset",
			value:            []policy.Policy{},
			wantAllowed:      true,
			wantNotification: "Screenshot taken",
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+tc.name)

			// Minimum interval between screenshot commands is 1 second, so we
			// must sleep for 1 seconds to be able to take screenshot,
			// otherwise hotkey pressing will be ignored.
			//
			// Please check kScreenshotMinimumIntervalInMS constant in
			// ui/snapshot/screenshot_grabber.cc
			if err := testing.Sleep(ctx, time.Second); err != nil {
				s.Fatal("Failed to sleep: ", err)
			}

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			if err := ash.CloseNotifications(ctx, tconn); err != nil {
				s.Fatal("Failed to close notifications: ", err)
			}

			if err := screenshot.RemoveScreenshots(); err != nil {
				s.Fatal("Failed to remove screenshots: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, tc.value); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			if err := keyboard.Accel(ctx, "Ctrl+F5"); err != nil {
				s.Fatal("Failed to press Ctrl+F5 to take screenshot: ", err)
			}

			if _, err := ash.WaitForNotification(ctx, tconn, 15*time.Second, ash.WaitIDContains("capture_mode_notification"), ash.WaitTitle(tc.wantNotification)); err != nil {
				s.Fatalf("Failed to wait notification with title %q: %v", tc.wantNotification, err)
			}

			has, err := screenshot.HasScreenshots()
			if err != nil {
				s.Fatal("Failed to check whether screenshot is present: ", err)
			}
			if has != tc.wantAllowed {
				s.Errorf("Unexpected screenshot allowed: get %t; want %t", has, tc.wantAllowed)
			}
		})
	}
}
