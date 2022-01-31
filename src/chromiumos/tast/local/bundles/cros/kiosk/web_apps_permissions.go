// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/kioskmode"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebAppsPermissions,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks that Kiosk permissions are granted or blocked without prompts",
		Contacts: []string{
			"greengrape@google.com", // Test author
			"chromeos-kiosk-eng+TAST@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.KioskAutoLaunchCleanup,
		Params: []testing.Param{
			{
				Name: "ash",
				Val:  chrome.ExtraArgs(""),
			},
			{
				Name: "lacros",
				Val:  chrome.ExtraArgs("--enable-features=LacrosSupport,WebKioskEnableLacros", "--lacros-availability-ignore"),
			},
		},
		Timeout: 5 * time.Minute,
	})
}

func WebAppsPermissions(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	chromeOptions := s.Param().(chrome.Option)

	webAppURL := "https://permission.site"
	webAppTitle := "Website for testing various permission requests."

	accountID := "permissions@permissions.permissions"
	accountType := policy.AccountTypeKioskWebApp

	kioskWebAppPolicy := policy.DeviceLocalAccountInfo{
		AccountID:   &accountID,
		AccountType: &accountType,
		WebKioskAppInfo: &policy.WebKioskAppInfo{
			Url:   &webAppURL,
			Title: &webAppTitle,
		}}

	accountsConfiguration := policy.DeviceLocalAccounts{
		Val: []policy.DeviceLocalAccountInfo{
			kioskWebAppPolicy,
		},
	}

	for _, param := range []struct {
		name                 string
		permissionButtonName string
		permissionRequest    string
	}{
		{
			name:                 "Camera",
			permissionButtonName: "Camera",
			permissionRequest:    "Use your camera",
		},
		{
			name:                 "Microphone",
			permissionButtonName: "Microphone",
			permissionRequest:    "Use your microphone",
		},
		{
			name:                 "Location",
			permissionButtonName: "Location",
			permissionRequest:    "Know your location",
		},
		{
			name:                 "Notifications",
			permissionButtonName: "Notifications",
			permissionRequest:    "Show notifications",
		},
		{
			name:                 "Download",
			permissionButtonName: "Auto Download",
			permissionRequest:    "Download multiple files",
		},
		{
			name:                 "ReadFromClipboard",
			permissionButtonName: "Read text",
			permissionRequest:    "See text and images copied to the clipboard",
		},
		{
			// TODO: Figure out the correct ui node for write prompts
			name:                 "WriteToClipboard",
			permissionButtonName: "Write text",
			permissionRequest:    "???",
		},
		{
			// TODO: Figure out the correct ui node for security key prompts
			// TODO: why is it actually shown? :)
			name:                 "SecurityKey",
			permissionButtonName: "WebAuthn Attestation",
			permissionRequest:    "Insert your security key and touch it",
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			kiosk, cr, err := kioskmode.New(
				ctx,
				fdms,
				kioskmode.CustomLocalAccounts(&accountsConfiguration),
				kioskmode.ExtraChromeOptions(chromeOptions),
				kioskmode.AutoLaunch(accountID),
			)

			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, fmt.Sprintf("kiosk_WebAppsPermissions_%s", param.name))

			if err != nil {
				s.Error("Failed to start Chrome in Kiosk mode: ", err)
			}

			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create Test API connection: ", err)
			}

			ui := uiauto.New(tconn)
			button := nodewith.Role(role.Button).Name(param.permissionButtonName)
			alertDialog := nodewith.Role(role.AlertDialog).NameContaining(param.permissionRequest)

			// Click the specified button and ensure that permission prompt is not shown.
			// Be careful with crbug/215674045 -- the logic below won't function properly without WaitUntilExists()
			// even though the ui is fully loaded by the time of the call.
			if err := uiauto.Combine(fmt.Sprintf("Click the %s button", param.permissionButtonName),
				ui.WaitUntilExists(button),
				ui.LeftClick(button),
				ui.EnsureGoneFor(alertDialog, 10*time.Second),
			)(ctx); err != nil {
				s.Fatal(fmt.Sprintf("Failed to click the %s button: ", param.permissionButtonName), err)
			}

			testing.Sleep(ctx, 2*time.Second)
			faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), func() bool { return true }, cr, fmt.Sprintf("kiosk_WebAppsPermissions_228_%s", param.name))

			defer kiosk.Close(ctx)
		})
	}
}
