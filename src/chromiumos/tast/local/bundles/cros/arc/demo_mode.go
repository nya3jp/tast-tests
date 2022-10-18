// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/demomode/fixture"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DemoMode,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Basic test for ARC++ in Demo Mode from OOBE",
		Contacts:     []string{"arc-commercial@google.com"},
		Fixture:      fixture.PostDemoModeOOBE,
		Attr:         []string{"group:mainline", "informational"},
		// Demo Mode uses Zero Touch Enrollment for enterprise enrollment, which
		// requires a real TPM.
		// We require "arc" and "chrome_internal" because the ARC TOS screen
		// is only shown for chrome-branded builds when the device is ARC-capable.
		SoftwareDeps: []string{"chrome", "chrome_internal", "arc", "tpm", "play_store"},
	})
}

// DemoMode goes through the basic flow of entering Demo Mode from OOBE, open
// Play Store and verify the "Install" button is not enabled.
func DemoMode(ctx context.Context, s *testing.State) {
	const (
		installButtonText = "Install"
	)

	cr, err := chrome.New(ctx,
		chrome.NoLogin(),
		chrome.ARCSupported(),
		chrome.KeepEnrollment(),
		// Force devtools on regardless of policy (devtools is disabled in
		// Demo Mode policy) to support connecting to the test API extension.
		chrome.ExtraArgs("--force-devtools-available"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	clearUpCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	defer cr.Close(clearUpCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create the test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(clearUpCtx, s.OutDir(), s.HasError, tconn)

	uia := uiauto.New(tconn)

	// Wait for ARC to start and ADB to be setup, which would take a bit long.
	arc, err := arc.NewWithTimeout(ctx, s.OutDir(), 2*time.Minute)
	if err != nil {
		s.Fatal("Failed to get ARC: ", err)
	}

	if err = uia.WithTimeout(5 * time.Second).WaitUntilExists(nodewith.Name(apps.PlayStore.Name).ClassName("ash/ShelfAppButton"))(ctx); err != nil {
		s.Fatal("Failed to wait for Play Store icon on shelf: ", err)
	}

	if err := apps.Launch(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to launch Play Store: ", err)
	}

	// ARC opt-in may take a long time to finish.
	classNameRegexp := regexp.MustCompile(`^ExoShellSurface(-\d+)?$`)
	playStoreUI := nodewith.Name("Play Store").Role(role.Window).ClassNameRegex(classNameRegexp)
	if err := uia.WithTimeout(10 * time.Second).WaitUntilExists(playStoreUI)(ctx); err != nil {
		s.Fatal("Failed to see ARC Opt-In UI: ", err)
	}

	// Find the Calculator app in Play Store.
	playStoreAppPageURI := "https://play.google.com/store/apps/details?id=" + "com.google.android.calculator"
	intentActionView := "android.intent.action.VIEW"
	if err := arc.SendIntentCommand(ctx, intentActionView, playStoreAppPageURI).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to send intent to open the Play Store: ", err)
	}

	// Between Chrome and Play Store, always open with Play Store: Choose Play
	// Store and click "Always".
	d, err := arc.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)
	openWith := d.Object(ui.Text("Play Store"))
	if err := openWith.WaitForExists(ctx, 10*time.Second); err == nil {
		// If we get a prompt, click to always use Play Store.
		if err := openWith.Click(ctx); err != nil {
			s.Error("Failed to click 'Open with Play Store': ", err)
		}

		alwaysLink := d.Object(ui.Text("Always"))
		if err := alwaysLink.Click(ctx); err != nil {
			s.Error("Failed to click 'Always': ", err)
		}
	} // If we didn't get a prompt, the Play Store *might* be open, so keep going anyway.

	// Ensure that the "Install" button is disabled.
	opButton := d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches(installButtonText), ui.Enabled(false))
	if err := opButton.WaitForExists(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to find greyed Install button in Play Store")
	}
}
