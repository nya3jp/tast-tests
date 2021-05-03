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
		Func: Autostart,
		Desc: "Checks that Kiosk configuration starts when set to autologin",
		Contacts: []string{
			"kamilszarek@google.com", // Test author
			"alt-modalities-stability@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "fakeDMSEnrolled",
	})
}

func Autostart(ctx context.Context, s *testing.State) {
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
		if cr != nil {
			if err := cr.Close(ctx); err != nil {
				s.Error("Failed to close Chrome connection: ", err)
			}
		}
	}(ctx)

	webKioskAccountID := "1"
	webKioskAccountType := policy.AccountTypeKioskWebApp
	webKioskIconURL := "https://www.google.com"
	webKioskTitle := "TastKioskModeSetByPolicyGooglePage"
	webKioskURL := "https://www.google.com"

	kioskAppAccountID := "2"
	kioskAppAccountType := policy.AccountTypeKioskApp
	kioskAppID := "aajgmlihcokkalfjbangebcffdoanjfo"
	// TODO: kamilszarek@ - turn that into a default Kiosk mode fixture
	// Update policies and refresh. Here we wait for test API, hence login was required
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
				{
					AccountID:   &webKioskAccountID,
					AccountType: &webKioskAccountType,
					WebKioskAppInfo: &policy.WebKioskAppInfo{
						Url:     &webKioskURL,
						Title:   &webKioskTitle,
						IconUrl: &webKioskIconURL,
					},
				},
			},
		},
		&policy.DeviceLocalAccountAutoLoginId{
			Val: webKioskAccountID,
		},
	}); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}
	// Close the previous Chrome instance.
	if err := cr.Close(ctx); err != nil {
		s.Error("Failed to close Chrome connection: ", err)
	}

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

	// In this particular case when Kiosk mode starts the entry
	// '[...] Starting kiosk mode of type 2...'
	// '[...] Kiosk launch succeeded, wait for app window.'
	// is logged in /var/log/messages. This part will grep for that entry.
	// TODO: kamilszarek@ this may need to be changed not to pick up entries
	// from other Kiosk tests.

	const (
		kioskStarting        = "Starting kiosk mode"
		kioskLaunchSucceeded = "Kiosk launch succeeded"
	)

	if _, err := reader.Wait(ctx, 60*time.Second,
		func(e *syslog.Entry) bool {
			s.Log(e.Content)
			return strings.Contains(e.Content, kioskStarting)
		},
	); err != nil {
		s.Fatal("Failed to verify starting of Kiosk mode: ", err)
	}

	if _, err := reader.Wait(ctx, 60*time.Second,
		func(e *syslog.Entry) bool {
			return strings.Contains(e.Content, kioskLaunchSucceeded)
		},
	); err != nil {
		s.Fatal("Failed to verify successful launch of Kiosk mode: ", err)
	}
}
