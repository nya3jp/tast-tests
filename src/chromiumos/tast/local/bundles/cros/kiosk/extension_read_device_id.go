// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/kioskmode"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ExtensionReadDeviceID,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Verifies that kiosk extensions can access the Directory API ID",
		Contacts: []string{
			"zubeil@google.com", // Test author
			"chromeos-kiosk-eng+TAST@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.FakeDMSEnrolled,
	})
}

func ExtensionReadDeviceID(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	accountID := "foo@managedchrome.com"
	accountType := policy.AccountTypeKioskApp
	appID := "ilnpadgckeacioehlommkaafedibdeob"
	deviceID := "123e4567-e89b-12d3-a456-426614174000"
	account := &policy.DeviceLocalAccounts{
		Val: []policy.DeviceLocalAccountInfo{
			{
				AccountID:   &accountID,
				AccountType: &accountType,
				KioskAppInfo: &policy.KioskAppInfo{
					AppId: &appID,
				},
			},
		},
	}

	kiosk, cr, err := kioskmode.New(
		ctx,
		fdms,
		kioskmode.CustomLocalAccounts(account),
		kioskmode.AutoLaunch(accountID),
		kioskmode.CustomDirectoryAPIID(deviceID),
	)

	if err != nil {
		s.Fatal("Failed to start Chrome in Kiosk mode: ", err)
	}

	defer kiosk.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)
	assetButton := nodewith.Role("button").Name("Display Asset ID")
	assetText := nodewith.Role("staticText").NameStartingWith("Device Directory API ID: " + deviceID)

	if err := uiauto.Combine("Clicking button",
		ui.WaitUntilExists(assetButton),
		ui.LeftClick(assetButton),
		ui.WaitUntilExists(assetText),
	)(ctx); err != nil {
		s.Fatal("Failed to verify Device Directory API ID: ", err)
	}
}
