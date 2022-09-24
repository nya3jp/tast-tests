// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testutil contains functionality shared by tast tests for android apps on Chromebooks.
package testutil

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/testing"
)

// LoginToApp verifies login to app.
func LoginToApp(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {

	const (
		appPageClassName    = "android.widget.FrameLayout"
		enterEmailAddressID = "i0116"
		nextButtonText      = "Next"
		passwordID          = "i0118"
		signInText          = "Sign in"
		notNowID            = "android:id/autofill_save_no"
	)

	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	t, ok := arc.Type()
	if !ok {
		s.Fatal("Unable to determine arc type")
	}

	// Enter email address.
	enterEmailAddress(ctx, s, tconn, a, d, appPkgName, appActivity)

	// Click on next button
	nextButton := d.Object(ui.ClassName(AndroidButtonClassName), ui.TextMatches("(?i)"+nextButtonText))
	if err := nextButton.WaitForExists(ctx, DefaultUITimeout); err != nil {
		s.Log("Next Button doesn't exists: ", err)
		// Press enter key to click on next button.
		if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
			s.Fatal("Failed to enter KEYCODE_ENTER: ", err)
		} else {
			s.Log("Entered KEYCODE_ENTER")
		}
	} else if err := nextButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on nextButton: ", err)
	}

	// Enter password.
	enterPassword(ctx, s, tconn, a, d, appPkgName, appActivity)

	if tabletModeEnabled && t == arc.VM {
		s.Log("Device is ARC-R and in tablet mode")
		// Press enter key to click on sign in button.
		if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
			s.Fatal("Failed to enter KEYCODE_ENTER: ", err)
		} else {
			s.Log("Entered KEYCODE_ENTER")
		}
	} else {
		// Click on Sign in button.
		signInButton := d.Object(ui.ClassName(AndroidButtonClassName), ui.TextMatches("(?i)"+signInText))
		if err := signInButton.WaitForExists(ctx, DefaultUITimeout); err != nil {
			s.Fatal("SignInButton doesn't exists: ", err)
		}
		// Click on signIn Button until not now button exist.
		signInButton = d.Object(ui.ClassName(AndroidButtonClassName), ui.TextMatches("(?i)"+signInText))
		notNowButton := d.Object(ui.ID(notNowID))
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := notNowButton.Exists(ctx); err != nil {
				signInButton.Click(ctx)
				return err
			}
			return nil
		}, &testing.PollOptions{Timeout: DefaultUITimeout}); err != nil {
			s.Log("notNowButton doesn't exist: ", err)
		} else if err := notNowButton.Click(ctx); err != nil {
			s.Fatal("Failed to click on notNowButton: ", err)
		}
	}
}

// enterEmailAddress func verifies email address can be entered successfully.
func enterEmailAddress(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		appPageClassName    = "android.widget.FrameLayout"
		enterEmailAddressID = "i0116"
	)
	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	t, ok := arc.Type()
	if !ok {
		s.Fatal("Unable to determine arc type")
	}

	enterEmailAddress := d.Object(ui.ID(enterEmailAddressID))
	// Wait for the existence of enterEmailAddress field and then click on it.
	if err := enterEmailAddress.WaitForExists(ctx, LongUITimeout); err != nil {
		s.Fatal("EnterEmailAddress doesn't exists: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	}

	// If device is ARC-P and is in clamshell mode.
	if t == arc.Container || (!tabletModeEnabled && t == arc.VM) {
		s.Log("Device is in clamshell mode")
		// For arc-P devices, click on emailid text field until the emailid text field is focused.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if emailIDFocused, err := enterEmailAddress.IsFocused(ctx); err != nil {
				return errors.New("email text field not focused yet")
			} else if !emailIDFocused {
				enterEmailAddress.Click(ctx)
				return errors.New("email text field not focused yet")
			}
			return nil
		}, &testing.PollOptions{Timeout: DefaultUITimeout}); err != nil {
			s.Log("Failed to focus EmailId: ", err)
		}
	}
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()
	emailID := s.RequiredVar("arcappcompat.Minecraft.emailid")
	if err := kb.Type(ctx, emailID); err != nil {
		s.Fatal("Failed to enter emailID: ", err)
	}
	s.Log("Entered EmailAddress")
}

func enterPassword(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		passwordID = "i0118"
		// The inputs are not immediately active after being clicked
		// so wait a moment for the engine to make the input active before interacting with it.
		waitForActiveInputTime = time.Second * 10
	)

	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	t, ok := arc.Type()
	if !ok {
		s.Fatal("Unable to determine arc type")
	}

	ud := uidetection.NewDefault(tconn).WithTimeout(time.Minute).WithScreenshotStrategy(uidetection.ImmediateScreenshot)
	enterPassword := d.Object(ui.ID(passwordID))
	enterPWD := uidetection.TextBlock([]string{"Enter", "password"})
	pwdEditField := uidetection.TextBlock([]string{"password"}).Nth(0).Below(enterPWD)

	if tabletModeEnabled && t == arc.VM { // If device is ARC-R and is in tablet mode.
		s.Log("Device is ARC-R and in tablet mode")
		// For arc-vm devices and for tablet mode, wait for the existence of enterpassword field and then click on it.
		if err := uiauto.Combine("Find password edit field",
			ud.WaitUntilExists(pwdEditField),
			ud.Tap(pwdEditField),
			action.Sleep(waitForActiveInputTime),
		)(ctx); err != nil {
			s.Fatal("Failed to find password edit field: ", err)
		}
	} else {
		s.Log("Device is in clamshell")
		// Enter password.
		if err := enterPassword.WaitForExists(ctx, LongUITimeout); err != nil {
			s.Log("EnterPassword doesn't exists: ", err)
		} else if err := enterPassword.Click(ctx); err != nil {
			s.Fatal("Failed to click on enterPassword: ", err)
		}

		// In clamshell mode for both ARC-P and ARC-R,
		// click on password text field until the password text field is focused.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if pwdFocused, err := enterPassword.IsFocused(ctx); err != nil {
				return errors.New("password text field not focused yet")
			} else if !pwdFocused {
				enterPassword.Click(ctx)
				return errors.New("password text field not focused yet")
			}
			return nil
		}, &testing.PollOptions{Timeout: DefaultUITimeout}); err != nil {
			s.Fatal("Failed to focus password: ", err)
		}
	}
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()
	password := s.RequiredVar("arcappcompat.Minecraft.password")
	if err := kb.Type(ctx, password); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}
	s.Log("Entered password")
}

// ClickUntilFocused func click on specified element until it is focused.
func ClickUntilFocused(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, checkElement *ui.Object) {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if uiElementIsFocused, err := checkElement.IsFocused(ctx); err != nil {
			return errors.New("Ui element field not focused yet")
		} else if !uiElementIsFocused {
			checkElement.Click(ctx)
			return errors.New("Ui element field not focused yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: ShortUITimeout}); err != nil {
		s.Fatal("Failed to focus Ui element: ", err)
	}
}

// ClickUntilButtonExists func click on until specified button exists.
func ClickUntilButtonExists(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, checkElement, targetElement *ui.Object) {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := targetElement.Exists(ctx); err != nil {
			checkElement.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: ShortUITimeout}); err != nil {
		s.Log("targetElement doesn't exist: ", err)
	}
}

// CloseAndRelaunchApp to skip ads.
func CloseAndRelaunchApp(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	if err := a.Command(ctx, "am", "force-stop", appPkgName).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to stop app: ", err)
	}
	act, err := arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create new app activity: ", err)
	}
	defer act.Close()
	// Launch the app.
	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		s.Fatal("Failed to start app: ", err)
	}
	s.Log("Closed and relaunch the app successfully")
}
