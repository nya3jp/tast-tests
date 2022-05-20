// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/kioskmode"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchErrorMetrics,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks if Kiosk.Launch.Error UMA is logged when there is an error",
		Contacts: []string{
			"yixie@google.com", // Test author
			"chromeos-kiosk-eng+TAST@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
		},
		Params: []testing.Param{{
			Name: "ash",
			Val:  chrome.ExtraArgs(""),
		}, {
			Name:              "lacros",
			Val:               chrome.ExtraArgs("--enable-features=LacrosSupport,ChromeKioskEnableLacros", "--lacros-availability-ignore"),
			ExtraSoftwareDeps: []string{"lacros"},
		}},
		Fixture: fixture.FakeDMSEnrolled,
	})
}

const (
	launchErrorName = "Kiosk.Launch.Error"
	launchErrorEnum = 7
)

func LaunchErrorMetrics(ctx context.Context, s *testing.State) {
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
		s.Fatal("Failed to start Chrome in Kiosk mode: ", err)
	}
	defer kiosk.Close(ctx)

	testConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, testConn)

	oldHistogram, err := metrics.GetHistogram(ctx, testConn, launchErrorName)
	if err != nil {
		s.Fatal("Failed to fetch old histogram: ", err)
	}

	ui := uiauto.New(testConn)

	// It looks like UI is not stable to interact even when polling for
	// elements. When waiting for elements and then clicking on
	// kioskmode.KioskAppBtnNode the UI element froze. I was not able to find
	// out how to overcome flakiness other than using sleep before interacting
	// with UI.
	testing.Sleep(ctx, 3*time.Second)

	localAccountsBtn := nodewith.Name("Apps").HasClass("MenuButton")
	cancelLaunchText := nodewith.Name("Press Ctrl + Alt + S to switch to ChromeOS").Role("staticText")
	if err := uiauto.Combine("launch Kiosk app from menu",
		ui.WaitUntilExists(localAccountsBtn),
		ui.LeftClick(localAccountsBtn),
		ui.WaitUntilExists(kioskmode.KioskAppBtnNode),
		ui.LeftClick(kioskmode.KioskAppBtnNode),
		ui.WaitUntilExists(cancelLaunchText),
	)(ctx); err != nil {
		s.Fatal("Failed to start Kiosk application from Sign-in screen: ", err)
	}
	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer kw.Close()

	if err := kw.Accel(ctx, "Ctrl+Alt+S"); err != nil {
		s.Error("Failed to hit ctrl+alt+s and attempt to quit a kiosk app: ", err)
	}
	// The current Chrome process will exit after pressing Ctrl+Alt+S. Waiting for
	// UI events is therefore not possible. A short delay is needed to make sure
	// launch error event will be triggered before restarting UI.
	testing.Sleep(ctx, time.Second)

	// Restart Chrome without closing since the current Chrome process has already
	// exited itself. Signin profile test extension is needed to check the error
	// message on the UI.
	cr, err = kiosk.RestartChromeNoCloseWithOptions(
		ctx,
		chrome.NoLogin(),
		chrome.DMSPolicy(fdms.URL),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
		chrome.KeepState(),
		chromeOptions)
	if err != nil {
		s.Fatal("Failed to connect to new chrome instance: ", err)
	}

	testConn, err = cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	ui = uiauto.New(testConn)
	if err := ui.WaitUntilExists(nodewith.Name("Kiosk application launch canceled."))(ctx); err != nil {
		s.Fatal("Kiosk application failed to be canceled by user: ", err)
	}

	// Check the diff of histogram, expecting a new bucket of Kiosk.Launch.Error.
	histogram, err := metrics.WaitForHistogramUpdate(ctx, testConn, launchErrorName, oldHistogram, 15*time.Second)
	if err != nil {
		s.Fatal("Timed out waiting for Kiosk.Launch.Error metrics: ", err)
	}

	// Check that the histogram matches exactly:
	//   Kiosk.Launch.Error, min=7, max=8, count=1
	if histogram.Name != launchErrorName ||
		len(histogram.Buckets) == 0 ||
		histogram.Buckets[0].Min != launchErrorEnum ||
		histogram.Buckets[0].Max != launchErrorEnum+1 ||
		histogram.Buckets[0].Count != 1 {
		s.Fatal("Unexpected histogram: ", histogram)
	}
	testing.ContextLog(ctx, "Histogram: ", histogram)

	// Restart Chrome into regular user session to do cleanup. Policy refresh
	// doesn't work on the login screen, and it will be stuck during cleanup.
	cr, err = kiosk.RestartChromeWithOptions(
		ctx,
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}), // Required as refreshing policies require test API.
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepState(),
		chromeOptions)
}
