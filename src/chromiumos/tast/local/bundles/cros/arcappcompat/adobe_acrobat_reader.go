// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AdobeAcrobatReader,
		Desc:         "Functional test for Adobe Acrobat Reader that installs the app also verifies it is logged in and that the main page is open",
		Contacts:     []string{"archanasing@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.BootedAppCompat(),
		}, {
			Name:              "tablet_mode",
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               arc.BootedInTabletModeAppCompat(),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBootedAppCompat(),
		}, {
			Name:              "vm_tablet_mode",
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               arc.VMBootedInTabletModeAppCompat(),
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// AdobeAcrobatReader test uses library for opting into the playstore and installing app.
// Launch the app from playstore.
// Verify app is logged in.
// Verify app reached main activity page of the app.
func AdobeAcrobatReader(ctx context.Context, s *testing.State) {
	const (
		appPkgName = "com.adobe.reader"

		userButtonClassName    = "android.widget.TextView"
		openButtonClassName    = "android.widget.Button"
		openButtonRegex        = "Open|OPEN"
		signInButtonText       = "Sign in with Google"
		appAccountName         = "App Compat"
		appAccountID           = "com.google.android.gms:id/account_display_name"
		continueButtonText     = "Continue"
		continueButtonID       = "com.adobe.reader:id/continue_button"
		toolsButtonID          = "com.adobe.reader:id/fab_button"
		toolsButtonDescription = "Tools"

		defaultUITimeout = 20 * time.Second
		longUITimeout    = 5 * time.Minute
	)

	// Setup Chrome.
	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	s.Log("Enable showing ANRs")
	if err := a.Command(ctx, "settings", "put", "secure", "anr_show_background", "1").Run(testexec.DumpLogOnError); err != nil {
		s.Error("Failed to enable showing ANRs: ", err)
	}
	s.Log("Enable crash dialog")
	if err := a.Command(ctx, "settings", "put", "secure", "show_first_crash_dialog_dev_option", "1").Run(testexec.DumpLogOnError); err != nil {
		s.Error("Failed to enable crash dialog: ", err)
	}

	s.Log("Installing app")
	if err := apps.Launch(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to launch Play Store: ", err)
	}
	if err := playstore.InstallApp(ctx, a, d, appPkgName); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	must := func(err error) {
		if err != nil {
			s.Fatal(err) // NOLINT: arc/ui returns loggable errors
		}
	}

	// Launch Acrobat Reader app.
	// Click on open button.
	openButton := d.Object(ui.ClassName(openButtonClassName), ui.TextMatches(openButtonRegex))
	must(openButton.WaitForExists(ctx, longUITimeout))
	// Open button exists and click
	must(openButton.Click(ctx))

	// Click on sign in button.
	signInButton := d.Object(ui.ClassName(userButtonClassName), ui.Text(signInButtonText))
	if err := signInButton.WaitForExists(ctx, defaultUITimeout); err != nil {
		s.Log("Sign in Button doesn't exists: ", err)
	} else {
		must(signInButton.Click(ctx))
	}

	// Click on account button.
	accountButton := d.Object(ui.ID(appAccountID), ui.Text(appAccountName))
	if err := accountButton.WaitForExists(ctx, defaultUITimeout); err != nil {
		s.Log("Account Button doesn't exists: ", err)
	} else {
		must(accountButton.Click(ctx))
	}

	// Click on continue button.
	continueButton := d.Object(ui.ID(continueButtonID), ui.Text(continueButtonText))
	if err := continueButton.WaitForExists(ctx, defaultUITimeout); err != nil {
		s.Log("Continue Button doesn't exists: ", err)
	} else {
		must(continueButton.Click(ctx))
	}

	// Check for tools button in home page.
	toolsButton := d.Object(ui.ID(toolsButtonID), ui.DescriptionContains(toolsButtonDescription))
	must(toolsButton.WaitForExists(ctx, longUITimeout))
}
