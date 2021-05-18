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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: StartAppFromSignInScreen,
		Desc: "Adds 2 Kiosk accounts, checks if both are available then starts one of them",
		Contacts: []string{
			"kamilszarek@google.com", // Test author
			"alt-modalities-stability@google.com",
		},
		Vars:         []string{"ui.signinProfileTestExtensionManifestKey"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "fakeDMSEnrolled",
	})
}

func StartAppFromSignInScreen(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*fakedms.FakeDMS)
	cr, err := chrome.New(
		ctx,
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
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
	// Update policies.
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

	// Restart Chrome.
	cr, err = chrome.New(ctx,
		chrome.DeferLogin(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment(),
	)
	if err != nil {
		s.Fatal("Chrome restart failed: ", err)
	}

	testConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, testConn)

	localAccountsButton := nodewith.Name("Apps").ClassName("MenuButton")
	simplePrintTestApplicationButton := nodewith.Name("Simple Printest").ClassName("MenuItemView")
	ui := uiauto.New(testConn)
	if err := uiauto.Combine("open Kiosk application menu",
		ui.LeftClick(localAccountsButton),
		ui.WaitUntilExists(simplePrintTestApplicationButton),
	)(ctx); err != nil {
		s.Fatal("Failed to find local account button: ", err)
	}

	// Gat applications that show up after clicking App button.
	menuItems, err := ui.NodesInfo(ctx, nodewith.ClassName("MenuItemView"))
	if err != nil {
		s.Fatal("Failed to get local accounts: ", err)
	}

	const expectedLocalAccountsCount = 2
	if len(menuItems) != expectedLocalAccountsCount {
		s.Fatalf("Expected %d local accounts, but found %v app(s) %q", expectedLocalAccountsCount, len(menuItems), menuItems)
	}

	// Open Kiosk application.
	s.Log("Open one Kiosk app")
	if err := uiauto.Combine("open web Kiosk application",
		ui.LeftClick(simplePrintTestApplicationButton),
	)(ctx); err != nil {
		s.Fatal("Failed to find local account button: ", err)
	}

	// Check Kiosk starts successfully.
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
