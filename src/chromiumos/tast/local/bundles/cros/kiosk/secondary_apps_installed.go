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
// between the types of mounts.
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
		Timeout: 2 * time.Minute, // Starting Kiosk twice requires longer timeout.
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

	accountID := "kiosk_account"
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
				chrome.ExtraArgs("--force-devtools-available"),
			),
		)
	} else {
		kiosk, cr, err = kioskmode.New(
			ctx,
			fdms,
			kioskmode.CustomLocalAccounts(account),
			kioskmode.ExtraChromeOptions(
				chrome.ExtraArgs("--force-devtools-available"),
				chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
			),
		)
	}
	if err != nil {
		s.Fatal("Failed to start Chrome in Kiosk mode: ", err)
	}
	defer kiosk.Close(ctx)

	if !param.autoLaunch {
		reader, err := syslog.NewReader(ctx, syslog.Program("chrome"))
		if err != nil {
			s.Fatal("Failed to start log reader: ", err)
		}
		defer reader.Close()

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
		appButtonNode := nodewith.Name(appName).ClassName("MenuItemView")
		ui := uiauto.New(testConn)
		if err := uiauto.Combine("launch Kiosk app from menu",
			ui.WaitUntilExists(localAccountsBtn),
			ui.LeftClick(localAccountsBtn),
			ui.WaitUntilExists(appButtonNode),
			ui.LeftClick(appButtonNode),
		)(ctx); err != nil {
			s.Fatal("Failed to start Kiosk application from Sign-in screen: ", err)
		}

		if err := kioskmode.ConfirmKioskStarted(ctx, reader); err != nil {
			s.Fatal("There was a problem while checking chrome logs for Kiosk related entries: ", err)
		}
	}

	if err := checkSecondaryAppAndExtension(ctx, cr); err != nil {
		s.Fatal("Failed to check secondary app and extension")
	}
}

// checkSecondaryAppAndExtension checks if secondary apps and extensions work.
func checkSecondaryAppAndExtension(ctx context.Context, cr *chrome.Chrome) error {
	const (
		appMainPageFormat = "chrome-extension://%s/app_main.html"

		primaryAppID   = "bkledbfligfdnfkmccllbllealecompm"
		secondaryAppID = "ogkfoejnfclpcafcnmdfbgambipkiake"

		extensionCheckText    = "Hello from extension: cobcmnlihjaffmjpeajkldoldonoaelf"
		secondaryAppCheckText = "App1 activated"
	)

	// Connecting to a Kiosk app before the app page is shown results in blank page.
	if err := testing.Sleep(ctx, 3*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep to wait for app launch")
	}

	testing.ContextLog(ctx, "Connecting to primary app")
	tconn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(fmt.Sprintf(appMainPageFormat, primaryAppID)))
	if err != nil {
		return errors.Wrap(err, "failed to connect to primary app")
	}

	testing.ContextLog(ctx, "Trying to click extension test button")
	if err := findAndClickButton(ctx, tconn, "test_extension"); err != nil {
		return errors.Wrap(err, "failed to click extension test button")
	}

	testing.ContextLog(ctx, "Trying to check extension result")
	if err := tconn.WaitForExpr(ctx, matchTextContentJS("extension_response", extensionCheckText)); err != nil {
		return errors.Wrap(err, "failed to check extension result")
	}

	testing.ContextLog(ctx, "Trying to click secondary app button")
	if err := findAndClickButton(ctx, tconn, "test_app1"); err != nil {
		return errors.Wrap(err, "failed to click secondary app button")
	}

	if err := tconn.Close(); err != nil {
		return errors.Wrap(err, "failed to close connection to primary app")
	}

	// Connecting to a Kiosk app before the app page is shown results in blank page.
	if err := testing.Sleep(ctx, time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep to wait for app launch")
	}

	testing.ContextLog(ctx, "Connecting to secondary app")
	tconn, err = cr.NewConnForTarget(ctx, chrome.MatchTargetURL(fmt.Sprintf(appMainPageFormat, secondaryAppID)))
	if err != nil {
		return errors.Wrap(err, "failed to connect to secondary app")
	}

	testing.ContextLog(ctx, "Trying to click secondary app close button")
	if err := findAndClickButton(ctx, tconn, "close_window"); err != nil {
		return errors.Wrap(err, "failed to click secondary app close button")
	}

	if err := tconn.Close(); err != nil {
		return errors.Wrap(err, "failed to close connection to secondary app")
	}

	testing.ContextLog(ctx, "Re-connecting to primary app")
	tconn, err = cr.NewConnForTarget(ctx, chrome.MatchTargetURL(fmt.Sprintf(appMainPageFormat, primaryAppID)))
	if err != nil {
		return errors.Wrap(err, "failed to re-connect to primary app")
	}

	testing.ContextLog(ctx, "Trying to check secondary app result")
	if err := tconn.WaitForExpr(ctx, matchTextContentJS("app1_response", secondaryAppCheckText)); err != nil {
		return errors.Wrap(err, "failed to check secondary app result")
	}

	return nil
}

// findAndClickButton finds a button on web page and clicks it.
func findAndClickButton(ctx context.Context, tconn *chrome.Conn, element string) error {
	testing.ContextLogf(ctx, "Trying to find %s button", element)
	if err := tconn.WaitForExpr(ctx, fmt.Sprintf("document.getElementById('%s') != null", element)); err != nil {
		return errors.Wrapf(err, "error occurred waiting for %s button", element)
	}

	testing.ContextLogf(ctx, "Trying to click %s button", element)
	if err := tconn.Eval(ctx, fmt.Sprintf("document.getElementById('%s').click()", element), nil); err != nil {
		return errors.Wrapf(err, "failed to click %s button", element)
	}

	return nil
}

// matchTextContentJS generates javascript for checking if text content of a web
// page element matches desired content.
func matchTextContentJS(element, text string) string {
	return fmt.Sprintf("document.getElementById('%s').textContent == '%s'", element, text)
}
