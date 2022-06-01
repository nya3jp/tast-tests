// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/kioskmode"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

// secondaryAppsInstalledParam contains the test parameters which are different
// between auto launch and manual launch.
type secondaryAppsInstalledParam struct {
	// True if Kiosk app is launched automatically. False if launched from menu.
	autoLaunch bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         SecondaryAppsInstalled,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks if secondary apps and extensions in a Kiosk app can be installed and launched",
		Contacts: []string{
			"yixie@google.com", // Test author
			"chromeos-kiosk-eng+TAST@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
		},
		Timeout: 2 * time.Minute,
		Params: []testing.Param{
			{
				Name: "auto_launch",
				Val: secondaryAppsInstalledParam{
					autoLaunch: true,
				},
				Fixture: fixture.KioskAutoLaunchCleanup,
			},
			{
				Name: "manual_launch",
				Val: secondaryAppsInstalledParam{
					autoLaunch: false,
				},
				Fixture: fixture.FakeDMSEnrolled,
			},
		},
	})
}

func SecondaryAppsInstalled(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	param := s.Param().(secondaryAppsInstalledParam)

	accountID := "kiosk_account@managedchrome.com"
	accountType := policy.AccountTypeKioskApp
	appID := "bkledbfligfdnfkmccllbllealecompm"
	appName := "Dev Kiosk App for multi-apps"

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
	var kiosk *kioskmode.Kiosk = nil
	var cr *chrome.Chrome = nil
	var err error = nil
	if param.autoLaunch {
		kiosk, cr, err = kioskmode.New(
			ctx,
			fdms,
			kioskmode.CustomLocalAccounts(account),
			kioskmode.AutoLaunch(accountID),
			kioskmode.ExtraChromeOptions(
				chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
			),
		)
	} else {
		kiosk, cr, err = kioskmode.New(
			ctx,
			fdms,
			kioskmode.CustomLocalAccounts(account),
			kioskmode.ExtraChromeOptions(
				chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
			),
		)
	}
	if err != nil {
		s.Fatal("Failed to start Chrome in Kiosk mode: ", err)
	}
	defer kiosk.Close(ctx)

	testConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, testConn)

	ui := uiauto.New(testConn)

	if !param.autoLaunch {
		reader, err := syslog.NewReader(ctx, syslog.Program("chrome"))
		if err != nil {
			s.Fatal("Failed to start log reader: ", err)
		}
		defer reader.Close()

		// It looks like UI is not stable to interact even when polling for
		// elements. When waiting for elements and then clicking on
		// kioskmode.KioskAppBtnNode the UI element froze. I was not able to find
		// out how to overcome flakiness other than using sleep before interacting
		// with UI.
		testing.Sleep(ctx, 3*time.Second)

		if err := kioskmode.StartFromSignInScreen(ctx, ui, appName); err != nil {
			s.Fatal("Failed to start Kiosk application from Sign-in screen: ", err)
		}

		if err := kioskmode.ConfirmKioskStarted(ctx, reader); err != nil {
			s.Fatal("There was a problem while checking chrome logs for Kiosk related entries: ", err)
		}
	}

	if err := checkSecondaryAppAndExtension(ctx, ui); err != nil {
		s.Fatal("Failed to check secondary app and extension")
	}
}

// checkSecondaryAppAndExtension checks if secondary apps and extensions work.
func checkSecondaryAppAndExtension(ctx context.Context, ui *uiauto.Context) error {
	const (
		extensionCheckText    = "Hello from extension: cobcmnlihjaffmjpeajkldoldonoaelf"
		secondaryAppCheckText = "App1 activated"
	)

	testing.ContextLog(ctx, "Checking secondary extension")
	extensionTestBtn := nodewith.Name("Test secondary extension").Focusable()
	extensionTestResult := nodewith.Name(extensionCheckText).Role("staticText")
	if err := uiauto.Combine("test secondary extension",
		ui.WaitUntilExists(extensionTestBtn),
		ui.LeftClick(extensionTestBtn),
		ui.WaitUntilExists(extensionTestResult),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to test secondary extension")
	}

	testing.ContextLog(ctx, "Checking secondary app")
	appTestBtn := nodewith.Name("Test secondary app").Focusable()
	appTestResult := nodewith.Name(secondaryAppCheckText).Role("staticText")
	closeSecondaryAppBtn := nodewith.Name("Close Window").Focusable()
	if err := uiauto.Combine("test secondary app",
		ui.WaitUntilExists(appTestBtn),
		ui.LeftClick(appTestBtn),
		ui.WaitUntilExists(closeSecondaryAppBtn),
		ui.LeftClick(closeSecondaryAppBtn),
		ui.WaitUntilGone(closeSecondaryAppBtn),
		ui.WaitUntilExists(appTestResult),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to test secondary app")
	}

	return nil
}
