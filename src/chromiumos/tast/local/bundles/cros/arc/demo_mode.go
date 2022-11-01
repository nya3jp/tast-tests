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
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/demomode/fixture"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DemoMode,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Enter Demo Mode from OOBE, open Play Store and verify the Install button is disabled",
		Contacts: []string{
			"yaohuali@google.com",
			"arc-commercial@google.com"},
		Fixture: fixture.PostDemoModeOOBE,
		Attr:    []string{"group:mainline", "informational"},
		// Demo Mode uses Zero Touch Enrollment for enterprise enrollment, which
		// requires a real TPM.
		// We require "arc" and "chrome_internal" because the ARC TOS screen
		// is only shown for chrome-branded builds when the device is ARC-capable.
		SoftwareDeps: []string{"chrome", "arc", "tpm", "play_store"},
		Timeout:      15 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "no_tablet_form_factor"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm", "no_tablet_form_factor"},
		}},
	})
}

func DemoMode(ctx context.Context, s *testing.State) {
	const (
		installButtonText = "Install"
		maxAttempts       = 2
	)

	attempts := 1

	// Indicates a failure in the core feature under test so the polling should stop.
	exit := func(desc string, err error) error {
		s.Fatalf("Failed to %s: %v", desc, err)
		return nil
	}

	// Indicates that the error is retryable and unrelated to core feature under test.
	retry := func(desc string, err error) error {
		if attempts < maxAttempts {
			attempts++
			err = errors.Wrap(err, "failed to "+desc)
			s.Logf("%s. Retrying", err)
			return err
		}
		return exit(desc, err)
	}

	// In Demo Mode, the session would restart if idling for 60 seconds, which would distupt the test. However, some statement below may block for over 60 seconds.
	// To solve this problem, periodically move mouse on a separate goroutine, to prevent idling.
	stop := make(chan struct{})
	go shakeMouse(ctx, s, stop)
	defer func() { stop <- struct{}{} }()

	if err := testing.Poll(ctx, func(ctx context.Context) (retErr error) {

		cr, err := chrome.New(ctx,
			chrome.NoLogin(),
			chrome.ARCSupported(),
			chrome.KeepEnrollment(),
			// Force devtools on regardless of policy (devtools is disabled in
			// Demo Mode policy) to support connecting to the test API extension.
			chrome.ExtraArgs("--force-devtools-available"))
		if err != nil {
			return retry("Failed to start Chrome: ", err)
		}
		clearupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
		defer cancel()
		defer cr.Close(clearupCtx)

		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			return retry("Failed to create the test API connection: ", err)
		}
		defer faillog.DumpUITreeOnError(clearupCtx, s.OutDir(), s.HasError, tconn)

		uia := uiauto.New(tconn)

		// Wait for ARC to start and ADB to be setup, which would take a bit long.
		arc, err := arc.NewWithTimeout(ctx, s.OutDir(), 5*time.Minute)
		if err != nil {
			return retry("Failed to get ARC: ", err)
		}
		defer arc.Close(clearupCtx)

		if err := apps.Launch(ctx, tconn, apps.PlayStore.ID); err != nil {
			return retry("Failed to launch Play Store: ", err)
		}

		// Verify that Play Store window shows up.
		classNameRegexp := regexp.MustCompile(`^ExoShellSurface(-\d+)?$`)
		playStoreUI := nodewith.Name("Play Store").Role(role.Window).ClassNameRegex(classNameRegexp)
		if err := uia.WithTimeout(30 * time.Second).WaitUntilExists(playStoreUI)(ctx); err != nil {
			return retry("Failed to see Play Store window: ", err)
		}

		// Find the Calculator app in Play Store.
		playStoreAppPageURI := "https://play.google.com/store/apps/details?id=" + "com.google.android.calculator"
		intentActionView := "android.intent.action.VIEW"
		if err := arc.SendIntentCommand(ctx, intentActionView, playStoreAppPageURI).Run(testexec.DumpLogOnError); err != nil {
			return retry("Failed to send intent to open the Play Store: ", err)
		}

		// Between Chrome and Play Store, always open with Play Store: Choose Play
		// Store and click "Always".
		d, err := arc.NewUIDevice(ctx)
		if err != nil {
			return retry("Failed initializing UI Automator: ", err)
		}
		defer d.Close(clearupCtx)
		// TODO(yaohuali): Narrow down on the target UI element, in case other element with same text is selected and clicked.
		openWith := d.Object(ui.Text("Play Store"))
		if err := openWith.WaitForExists(ctx, 1*time.Minute); err != nil {
			// If we didn't get a prompt, the Play Store might be open. If this happens, adjust test expectation accordingly.
			return retry("Failed to see the prompt to choose between Chrome and Play Store: ", err)
		}

		// If we get a prompt, click to always use Play Store.
		if err := openWith.Click(ctx); err != nil {
			return retry("Failed to click 'Open with Play Store': ", err)
		}
		// TODO(yaohuali): Narrow down on the target UI element, in case other element with same text is selected and clicked.
		alwaysLink := d.Object(ui.Text("Always"))
		if err := alwaysLink.Click(ctx); err != nil {
			return retry("Failed to click 'Always': ", err)
		}

		// Ensure that the "Install" button is disabled.
		opButton := d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches(installButtonText), ui.Enabled(false))
		if err := opButton.WaitForExists(ctx, 50*time.Second); err != nil {
			return retry("Failed to find greyed Install button in Play Store: ", err)
		}

		return nil
	}, nil); err != nil {
		s.Fatal("Demo mode test failed: ", err)
	}
}

func shakeMouse(ctx context.Context, s *testing.State, stop chan struct{}) {
	mouse, err := input.Mouse(ctx)
	if err != nil {
		s.Fatal("Failed to get the mouse: ", err)
	}
	defer mouse.Close()
	for {
		select {
		case <-stop:
		default:
			mouse.Move(5, 5)
			mouse.Move(-5, -5)
			testing.Sleep(ctx, 5*time.Second)
		}
	}
}
