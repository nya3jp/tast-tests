// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/kioskmode"
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
		Vars: []string{"ui.signinProfileTestExtensionManifestKey"},
		// Informational attribute can only be removed when
		// https://crbug.com/1207293 is resolved.
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

	// Update policies.
	if err := kioskmode.ServerPoliciesForDefaultApplications(ctx, fdms, cr); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	// Close the previous Chrome instance.
	if err := cr.Close(ctx); err != nil {
		s.Error("Failed to close Chrome connection: ", err)
	}

	reader, err := syslog.NewReader(ctx, syslog.Program("chrome"))
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

	localAccountsBtn := nodewith.Name("Apps").ClassName("MenuButton")
	ui := uiauto.New(testConn)
	if err := uiauto.Combine("open Kiosk application menu",
		ui.WaitUntilExists(localAccountsBtn),
		ui.LeftClick(localAccountsBtn),
		ui.WaitUntilExists(nodewith.ClassName("MenuItemView").First()),
	)(ctx); err != nil {
		s.Fatal("Failed to find local account button: ", err)
	}

	// Get applications that show up after clicking App button.
	menuItems, err := ui.NodesInfo(ctx, nodewith.ClassName("MenuItemView"))
	if err != nil {
		s.Fatal("Failed to get local accounts: ", err)
	}

	const expectedLocalAccountsCount = 2
	if len(menuItems) != expectedLocalAccountsCount {
		s.Fatalf("Expected %d local accounts, but found %v app(s) %q", expectedLocalAccountsCount, len(menuItems), menuItems)
	}

	// Open Kiosk application.
	if err := ui.LeftClick(kioskmode.KioskAppBtnNode); err != nil {
		s.Fatal("Failed to find Kiosk application button: ", err)
	}

	if err := kioskmode.CheckLogsConfirmingKioskStartedSucceesfully(ctx, reader); err != nil {
		s.Fatal("There was a problem while checking chrome logs for Kiosk related entries: ", err)
	}
}
