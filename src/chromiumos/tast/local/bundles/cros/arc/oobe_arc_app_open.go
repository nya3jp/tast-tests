// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/oobeutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/testing"
)

type oobeArcAppOpenTestOptions struct {
	consolidatedConsentEnabled bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         OobeArcAppOpen,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Launch ARC App post the OOBE Flow Setup Complete",
		Contacts:     []string{"cpiao@google.com", "cros-arc-te@google.com", "cros-oac@google.com", "cros-oobe@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Val: oobeArcAppOpenTestOptions{
				consolidatedConsentEnabled: false,
			},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: oobeArcAppOpenTestOptions{
				consolidatedConsentEnabled: false,
			},
		}, {
			Name:              "p_consolidated_consent",
			ExtraSoftwareDeps: []string{"android_p"},
			Val: oobeArcAppOpenTestOptions{
				consolidatedConsentEnabled: true,
			},
		}, {
			Name:              "vm_consolidated_consent",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: oobeArcAppOpenTestOptions{
				consolidatedConsentEnabled: true,
			},
		}},
		Timeout: chrome.GAIALoginTimeout + arc.BootTimeout + 25*time.Minute,
		VarDeps: []string{"arc.parentUser", "arc.parentPassword"},
	})
}

func OobeArcAppOpen(ctx context.Context, s *testing.State) {

	const (
		appPkgName  = "com.google.android.apps.books"
		appActivity = ".app.BooksActivity"
	)

	username := s.RequiredVar("arc.parentUser")
	password := s.RequiredVar("arc.parentPassword")

	testOptions := s.Param().(oobeArcAppOpenTestOptions)
	chromeOptions := []chrome.Option{
		chrome.DontSkipOOBEAfterLogin(),
		chrome.ARCSupported(),
		chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
	}
	if testOptions.consolidatedConsentEnabled {
		chromeOptions = append(chromeOptions, chrome.EnableFeatures("OobeConsolidatedConsent", "PerUserMetricsConsent"))
	} else {
		chromeOptions = append(chromeOptions, chrome.DisableFeatures("OobeConsolidatedConsent", "PerUserMetricsConsent"))
	}

	cr, err := chrome.New(ctx, chromeOptions...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	ui := uiauto.New(tconn)

	if testOptions.consolidatedConsentEnabled {
		if err := oobeutil.CompleteConsolidatedConsentOnboardingFlow(ctx, ui); err != nil {
			s.Fatal("Failed to go through the oobe flow: ", err)
		}
	} else {
		if err := oobeutil.CompleteRegularOnboardingFlow(ctx, ui /*reviewArcOptions=*/, false); err != nil {
			s.Fatal("Failed to go through the oobe flow: ", err)
		}
	}

	if err := oobeutil.CompleteTabletOnboarding(ctx, ui); err != nil {
		s.Fatal("Failed to test oobe Arc tablet flow: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	statusArea := nodewith.HasClass("ash/StatusAreaWidgetDelegate")

	s.Log("Waiting for setup complete notification")
	_, err = ash.WaitForNotification(ctx, tconn, 20*time.Minute, ash.WaitTitle("Setup complete"), ash.WaitMessageContains("Installed 6 out of 6 applications"))
	if err != nil {
		s.Log("Haven't found setup complete notification, will try to see if it's in notification UI")
		if err := ui.LeftClick(statusArea)(ctx); err != nil {
			s.Log("Failed to click status area : ", err)
		}
		uda := uidetection.NewDefault(tconn).WithScreenshotStrategy(uidetection.ImmediateScreenshot)

		setupCompleteNotification := uidetection.TextBlock([]string{"Setup", "complete"})
		if err := uda.WithTimeout(5 * time.Second).WaitUntilExists(setupCompleteNotification)(ctx); err != nil {
			s.Fatal("Failed waiting for Setup complete notification: ", err)
		}
	}

	s.Log("Waiting to check if app is installed before launching the app")
	if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.PlayBooks.ID, 2*time.Minute); err != nil {
		s.Fatal("Failed to wait for app to install: ", err)
	}

	s.Log("Launch the App")
	act, err := arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create a new activity: ", err)
	}
	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		act.Close()
		s.Fatal("Failed to start the activity: ", err)
	}

}
