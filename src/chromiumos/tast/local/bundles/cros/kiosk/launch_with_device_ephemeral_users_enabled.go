// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
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
		// Use cr as a reference to close the last started Chrome instance.
		if err := cr.Close(ctx); err != nil {
			s.Error("Failed to close Chrome connection: ", err)
		}
	}(ctx)

	kioskAppAccountID := "2"
	kioskAppAccountType := policy.AccountTypeKioskApp
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
	reader, err := syslog.NewReader(ctx, syslog.ProgramNameFilter(syslog.Chrome))
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

	const (
		kioskStarting        = "Starting kiosk mode"
		kioskLaunchSucceeded = "Kiosk launch succeeded"
	)

	s.Logf("Wait for '%q' to be present in the log", kioskStarting)
	if _, err := reader.Wait(ctx, 60*time.Second,
		func(e *syslog.Entry) bool {
			return strings.Contains(e.Content, kioskStarting)
		},
	); err != nil {
		s.Fatal("Failed to verify starting of Kiosk mode: ", err)
	}

	s.Logf("Wait for '%q' to be present in the log", kioskLaunchSucceeded)
	if _, err := reader.Wait(ctx, 60*time.Second,
		func(e *syslog.Entry) bool {
			return strings.Contains(e.Content, kioskLaunchSucceeded)
		},
	); err != nil {
		s.Fatal("Failed to verify successful launch of Kiosk mode: ", err)
	}
}
