// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"
	"time"

	"go.chromium.org/chromiumos/tast-tests/common/fixture"
	"go.chromium.org/chromiumos/tast-tests/common/policy/fakedms"
	"go.chromium.org/chromiumos/tast-tests/local/chrome"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/faillog"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/nodewith"
	"go.chromium.org/chromiumos/tast-tests/local/kioskmode"
	"go.chromium.org/chromiumos/tast-tests/local/syslog"
	"go.chromium.org/chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         StartAppFromSignInScreen,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Adds 2 Kiosk accounts, checks if both are available then starts one of them",
		Contacts: []string{
			"kamilszarek@google.com", // Test author
			"vkovalova@google.com",   // Lacros test author
			"chromeos-kiosk-eng+TAST@google.com",
		},
		Vars: []string{"ui.signinProfileTestExtensionManifestKey"},
		// Informational attribute can only be removed when
		// https://crbug.com/1207293 is resolved.
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.FakeDMSEnrolled,
		Params: []testing.Param{
			{
				Name: "ash",
				Val:  chrome.ExtraArgs(""),
			},
			{
				Name:              "lacros",
				Val:               chrome.ExtraArgs("--enable-features=LacrosSupport,WebKioskEnableLacros", "--lacros-availability-ignore"),
				ExtraSoftwareDeps: []string{"lacros"},
			},
		},
	})
}

func StartAppFromSignInScreen(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	chromeOptions := s.Param().(chrome.Option)
	kiosk, cr, err := kioskmode.New(
		ctx,
		fdms,
		kioskmode.DefaultLocalAccounts(),
		kioskmode.ExtraChromeOptions(
			chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
			chromeOptions,
		),
	)
	if err != nil {
		s.Error("Failed to start Chrome on Signin screen with set Kiosk apps: ", err)
	}

	defer kiosk.Close(ctx)

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

	reader, err := syslog.NewReader(ctx, syslog.Program("chrome"))
	if err != nil {
		s.Fatal("Failed to start log reader: ", err)
	}
	defer reader.Close()

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
