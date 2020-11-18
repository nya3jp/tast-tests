// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DisableScreenshotsCaptureMode,
		Desc: "Behavior of the DisableScreenshots policy, check whether screenshot can be taken from capture mode in quick settings",
		Contacts: []string{
			"lamzin@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

func DisableScreenshotsCaptureMode(ctx context.Context, s *testing.State) {
	fdms, err := fakedms.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start FakeDMS: ", err)
	}
	defer fdms.Stop(ctx)

	if err := fdms.WritePolicyBlob(fakedms.NewPolicyBlob()); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	cr, err := chrome.New(ctx,
		chrome.Auth(pre.Username, pre.Password, pre.GaiaID),
		chrome.DMSPolicy(fdms.URL),
		chrome.EnableFeatures("CaptureMode"))
	if err != nil {
		s.Fatal("Failed to create Chrome instance: ", err)
	}
	defer cr.Close(ctx)

	conn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test API connection: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, conn)

	for _, tc := range []struct {
		name      string
		value     []policy.Policy
		wantTitle string
	}{
		{
			name: "true",
			value: []policy.Policy{
				&policy.DisableScreenshots{Val: true},
				&policy.ScreenCaptureAllowed{Val: false},
			},
			wantTitle: "screen capture not allowed",
		},
		// {
		// 	name:      "false",
		// 	value:     []policy.Policy{&policy.DisableScreenshots{Val: false}},
		// 	wantTitle: "screen capture allowed",
		// },
		// {
		// 	name:      "unset",
		// 	value:     []policy.Policy{},
		// 	wantTitle: "screen capture allowed",
		// },
	} {
		// Update policies.
		if err := policyutil.ServeAndVerify(ctx, fdms, cr, tc.value); err != nil {
			s.Fatal("Failed to update policies: ", err)
		}

		if err := ash.OpenSystemTray(ctx, conn); err != nil {
			s.Fatal("Failed to open system tray: ", err)
		}

		// time.Sleep(3*time.Second)

		// if err := ash.OpenSystemTray(ctx, conn); err != nil {
		// 	s.Fatal("Failed to open system tray: ", err)
		// }

		if err := ash.TakeAreaScreenshot(ctx, conn); err != nil {
			s.Fatal("Failed to open system tray: ", err)
		}

		// s.Fatal("Failed")

		// TODO implement
	}
}
