// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/kioskmode"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AutoLoginBailout,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Stop a kiosk app launch on a splash screen",
		Contacts: []string{
			"pbond@google.com", // Test author
			"chromeos-kiosk-eng+TAST@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      fixture.KioskAutoLaunchCleanup,
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
		},
	})
}

func AutoLoginBailout(ctx context.Context, s *testing.State) {
	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer kw.Close()

	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	// Lacros test only, since PWA kiosk is going to be launched with lacros.
	chromeOptions := chrome.ExtraArgs("--enable-features=LacrosSupport,WebKioskEnableLacros", "--lacros-availability-ignore")

	kiosk, _, err := kioskmode.New(
		ctx,
		fdms,
		kioskmode.DefaultLocalAccounts(),
		kioskmode.ExtraChromeOptions(chromeOptions),
		kioskmode.AutoLaunch(kioskmode.WebKioskAccountID),
		// Instead of waiting for startup, it waits for kiosk mode to be ready to launch.
		kioskmode.SkipSuccessfulLaunchCheck(),
	)

	if err != nil {
		s.Error("Failed to start Chrome in Kiosk mode: ", err)
	}

	defer kiosk.Close(ctx)

	// Sign-in profile extension is needed to check the error message on the UI.
	cr, err := kiosk.CancelKioskLaunch(
		ctx,
		chrome.NoLogin(),
		chrome.DMSPolicy(fdms.URL),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
		chrome.KeepState(),
		chromeOptions)
	if err != nil {
		s.Fatal("Failed to connect to new chrome instance: ", err)
	}

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	ui := uiauto.New(tconn)
	if err := ui.WaitUntilExists(nodewith.Name("Kiosk application launch canceled."))(ctx); err != nil {
		s.Fatal("Launch cancelled message did not appear: ", err)
	}
}
