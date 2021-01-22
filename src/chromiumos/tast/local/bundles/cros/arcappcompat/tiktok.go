// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// ClamshellTests are placed here.
var clamshellTestsForTiktok = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForTiktok},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForTiktok = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForTiktok},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Tiktok,
		Desc:         "Functional test for Tiktok that installs the app also verifies it is logged in and that the main page is open, checks Tiktok correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForTiktok,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForTiktok,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForTiktok,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForTiktok,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// Tiktok test uses library for opting into the playstore and installing app.
// Checks Tiktok correctly changes the window states in both clamshell and touchview mode.
func Tiktok(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.zhiliaoapp.musically"
		appActivity = "com.ss.android.ugc.aweme.splash.SplashActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForTiktok verifies Tiktok is logged in and
// verify Tiktok reached main activity page of the app.
func launchAppForTiktok(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		loginText                      = "Already have an account? Log in"
		loginWithGoogleButtonClassName = "android.view.ViewGroup"
		loginWithPreviousDeviceText    = "Log in with previous device"
		emailAddressID                 = "com.google.android.gms:id/container"
		homeText                       = "Home"
		textviewClassName              = "android.widget.TextView"
		skipText                       = "Skip"
		startWatchingText              = "Start watching"
		signUpButtonID                 = "com.zhiliaoapp.musically:id/ak"
	)
	var (
		loginWithGoogleIndex       = 4
		loginWithGoogleButtonIndex = 3
		emailAddressIndex          = 0
	)
	// Press until KEYCODE_TAB until signup button is focused.
	signUpButton := d.Object(ui.ID(signUpButtonID))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if signUpBtnFocused, err := signUpButton.IsFocused(ctx); err != nil {
			return errors.New("signUpButton not focused yet")
		} else if !signUpBtnFocused {
			d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0)
			return errors.New("signUp button not focused yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Log("Failed to focus signup button: ", err)
	}

	// Check for signup button.
	signUpButton = d.Object(ui.ID(signUpButtonID))
	if err := signUpButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("signUpButton doesn't exist: ", err)
	} else if err := signUpButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on signUpButton: ", err)
	}

	// Press until KEYCODE_TAB until login button is focused.
	loginButton := d.Object(ui.TextMatches("(?i)" + loginText))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if loginBtnFocused, err := loginButton.IsFocused(ctx); err != nil {
			return errors.New("login button not focused yet")
		} else if !loginBtnFocused {
			d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0)
			return errors.New("login button not focused yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Log("Failed to focus login button: ", err)
	}

	// Check for login button.
	loginButton = d.Object(ui.TextMatches("(?i)" + loginText))
	if err := loginButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("LoginButton doesn't exist: ", err)
	} else if err := loginButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on loginButton: ", err)
	}

	// check for log in with previous device.
	checkForLoginWithPreviousDevice := d.Object(ui.ClassName(textviewClassName), ui.TextMatches("(?i)"+loginWithPreviousDeviceText))
	emailAddress := d.Object(ui.ID(emailAddressID), ui.Index(emailAddressIndex))
	if err := checkForLoginWithPreviousDevice.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("checkForLoginWithPreviousDevice doesn't exist: ", err)
		continueWithGoogle := d.Object(ui.ClassName(loginWithGoogleButtonClassName), ui.Index(loginWithGoogleButtonIndex))
		// Click on continue with Google button until EmailAddress exist.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := emailAddress.Exists(ctx); err != nil {
				continueWithGoogle.Click(ctx)
				return err
			}
			return nil
		}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
			s.Log("emailAddress doesn't exist: ", err)
		}

	} else {
		s.Log("checkForLoginWithPreviousDevice does exist")
		loginWithGoogleButton := d.Object(ui.ClassName(loginWithGoogleButtonClassName), ui.Index(loginWithGoogleIndex))
		// Click on login with Google Button until EmailAddress exist.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := emailAddress.Exists(ctx); err != nil {
				loginWithGoogleButton.Click(ctx)
				return err
			}
			return nil
		}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
			s.Log("emailAddress doesn't exist: ", err)
		}
	}

	skipButton := d.Object(ui.ClassName(textviewClassName), ui.TextMatches("(?i)"+skipText))
	emailAddress = d.Object(ui.ID(emailAddressID), ui.Index(emailAddressIndex))
	// Click on EmailAddress until skipButton exist.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := skipButton.Exists(ctx); err != nil {
			emailAddress.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Log("skipButton doesn't exist: ", err)
	} else if err := skipButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on skipButton: ", err)
	}

	// Click on start watching button.
	startWatchingButton := d.Object(ui.ClassName(textviewClassName), ui.TextMatches("(?i)"+startWatchingText))
	if err := startWatchingButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("startWatchingButton doesn't exist: ", err)
	} else if err := startWatchingButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on startWatchingButton: ", err)
	}

	// Check for home icon.
	homeIcon := d.Object(ui.TextMatches("(?i)" + homeText))
	if err := homeIcon.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		testutil.DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)
		s.Error("homeIcon doesn't exist: ", err)
	}
}
