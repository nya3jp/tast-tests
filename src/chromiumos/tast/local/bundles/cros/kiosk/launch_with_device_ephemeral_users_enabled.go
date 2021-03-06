// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/kioskmode"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchWithDeviceEphemeralUsersEnabled,
		Desc: "Checks that Kiosk configuration starts correctly with DeviceEphemeralUsersEnabled policy set to true",
		Contacts: []string{
			"kamilszarek@google.com", // Test author
			"alt-modalities-stability@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "fakeDMSEnrolled",
	})
}

func LaunchWithDeviceEphemeralUsersEnabled(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*fakedms.FakeDMS)
	cr, err := chrome.New(
		ctx,
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}), // Required as refreshing policies require test API.
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment(),
	)
	if err != nil {
		s.Error("Failed to start Chrome: ", err)
	}

	defer func(ctx context.Context) {
		// This is required for cases when DeviceLocalAccountAutoLoginId policy
		// is used. If we won't clear policies and refresh then when test
		// completes and later Chrome is restarted then Kiosk modes starts
		// again.
		policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{})
		// Use cr as a reference to close the last started Chrome instance.
		if err := cr.Close(ctx); err != nil {
			s.Error("Failed to close Chrome connection: ", err)
		}
	}(ctx)

	// TODO(crbug.com/1218392): Clean up the setup phase for Kiosk mode
	// those are values that set Kiosk account.
	// kioskAppAccountID account identifier - set arbitrary.
	kioskAppAccountID := "anyIdThatCouldIdentifyThisAccount"
	// kioskAppAccountType type of the device local account.
	kioskAppAccountType := policy.AccountTypeKioskApp
	// kioskAppID application ID that will be used when Kiosk mode starts. It
	// was pick from the manual test case. For the sake of the reproducibility
	// of the bug it could be any app. It is the PrintTest app ID not pintrest.
	kioskAppID := "aajgmlihcokkalfjbangebcffdoanjfo"
	if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{
		&policy.DeviceLocalAccounts{
			Val: []policy.DeviceLocalAccountInfo{
				{
					AccountID:   &kioskAppAccountID,
					AccountType: &kioskAppAccountType,
					KioskAppInfo: &policy.KioskAppInfo{
						AppId: &kioskAppID,
					},
				},
			},
		},
		&policy.DeviceLocalAccountAutoLoginId{
			Val: kioskAppAccountID,
		},
		&policy.DeviceEphemeralUsersEnabled{
			Val: true,
		},
	}); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	// https://crbug.com/1202902 it is enough to restart Chrome to trigger the
	// problem -> Kiosk won't start successfully.

	// Close the previous Chrome instance.
	if err := cr.Close(ctx); err != nil {
		s.Error("Failed to close Chrome connection: ", err)
	}

	// reader has to be created here, otherwise we miss the first log that
	// indicates the start of Kiosk mode.
	reader, err := syslog.NewReader(ctx, syslog.Program("chrome"))
	if err != nil {
		s.Fatal("Failed to start log reader: ", err)
	}
	defer reader.Close()

	// Restart Chrome. After that Kiosk auto starts.
	cr, err = chrome.New(ctx,
		chrome.NoLogin(),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment(),
	)
	if err != nil {
		s.Fatal("Chrome restart failed: ", err)
	}

	if err := kioskmode.ConfirmKioskStarted(ctx, reader); err != nil {
		s.Fatal("There was a problem while checking chrome logs for Kiosk related entries: ", err)
	}
}
