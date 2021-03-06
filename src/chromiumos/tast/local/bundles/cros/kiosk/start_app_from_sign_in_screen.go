// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"
	"time"

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
	if err := kioskmode.SetDefaultAppPolicies(ctx, fdms, cr); err != nil {
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

	// It looks like UI is not stable to interact even when polling for
	// elements. When waiting for elements and then clicking on
	// kioskmode.KioskAppBtnNode the UI element froze. I was not able to find
	// out how to overcome flakiness other than using sleep before interacting
	// with UI.
	testing.Sleep(ctx, 3*time.Second)

	localAccountsBtn := nodewith.Name("Apps").ClassName("MenuButton")
	ui := uiauto.New(testConn)
	if err := uiauto.Combine("open Kiosk application menu",
		ui.WaitUntilExists(localAccountsBtn),
		ui.LeftClick(localAccountsBtn),
		ui.WaitUntilExists(kioskmode.KioskAppBtnNode),
	)(ctx); err != nil {
		s.Fatal("Failed to open menu with local accounts: ", err)
	}

	// Get applications that show up after clicking Apps button.
	menuItems, err := ui.NodesInfo(ctx, nodewith.ClassName("MenuItemView"))
	if err != nil {
		s.Fatal("Failed to get local accounts: ", err)
	}

	const expectedLocalAccountsCount = 2
	if len(menuItems) != expectedLocalAccountsCount {
		s.Fatalf("Expected %d local accounts, but found %v app(s) %+v", expectedLocalAccountsCount, len(menuItems), menuItems)
	}

	// When I had here only clicking the menu item that should be visible, the
	// test failed at interacting the menu item.
	if err := uiauto.Combine("close and open Kiosk application menu then click on one menu item",
		ui.WaitUntilExists(kioskmode.KioskAppBtnNode), // Wait again for the menu item to be visible.
		ui.LeftClick(kioskmode.KioskAppBtnNode),       // Launch the Kiosk app.
	)(ctx); err != nil {
		s.Fatal("Failed to start Kiosk application from Sign-in screen: ", err)
	}

	if err := kioskmode.ConfirmKioskStarted(ctx, reader); err != nil {
		s.Fatal("There was a problem while checking chrome logs for Kiosk related entries: ", err)
	}
}
