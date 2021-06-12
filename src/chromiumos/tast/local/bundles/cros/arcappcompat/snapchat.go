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
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// clamshellLaunchForSnapchat launches Snapchat in clamshell mode.
var clamshellLaunchForSnapchat = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForSnapchat},
}

// touchviewLaunchForSnapchat launches Snapchat in tablet mode.
var touchviewLaunchForSnapchat = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForSnapchat},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Snapchat,
		Desc:         "Functional test for Snapchat that installs the app also verifies it is logged in and that the main page is open, checks Snapchat correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		// To skip on duffy(Chromebox) with no internal display.
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForSnapchat,
				CommonTest: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				Tests:      touchviewLaunchForSnapchat,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForSnapchat,
				CommonTest: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				Tests:      touchviewLaunchForSnapchat,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.username", "arcappcompat.password", "arcappcompat.Snapchat.username", "arcappcompat.Snapchat.password"},
	})
}

// Snapchat test uses library for opting into the playstore and installing app.
// Checks Snapchat correctly changes the window states in both clamshell and touchview mode.
func Snapchat(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.snapchat.android"
		appActivity = ".LandingPageActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForSnapchat verifies Snapchat is logged in and
// verify Snapchat reached main activity page of the app.
func launchAppForSnapchat(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText             = "ALLOW"
		cameraID                    = "com.snapchat.android:id/ngs_camera_icon_container"
		enterEmailAddressID         = "com.snapchat.android:id/username_or_email_field"
		loginButtonClassName        = "android.widget.TextView"
		loginText                   = "Log In"
		slideIconID                 = "com.snapchat.android:id/subscreen_top_left"
		signInID                    = "com.snapchat.android:id/nav_button"
		notNowID                    = "android:id/autofill_save_no"
		passwordID                  = "com.snapchat.android:id/password_field"
		profileID                   = "com.snapchat.android:id/neon_header_avatar_container"
		turnonText                  = "TURN ON"
		userNameOrEmailID           = "com.snapchat.android:id/login_username_hint"
		homeID                      = "com.bydeluxe.d3.android.program.Snapchat:id/action_home"
		whileUsingThisAppButtonText = "WHILE USING THE APP"
	)

	// Check for login button.
	loginButton := d.Object(ui.ClassName(loginButtonClassName), ui.TextMatches("(?i)"+loginText))
	if err := loginButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("LoginButton doesn't exist: ", err)
	}

	// To handle to multiple UI window for login to app.
	// Check for username or email text.
	checkForUsernameOrEmail := d.Object(ui.ID(userNameOrEmailID))
	if err := checkForUsernameOrEmail.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("checkForUsernameOrEmail doesn't exist: ", err)
		loginToSnapchat(ctx, s, tconn, a, d, appPkgName, appActivity)
	} else {
		s.Log("checkForUsernameOrEmail does exist")
		loginToSnapchatWithOtherUI(ctx, s, tconn, a, d, appPkgName, appActivity)
	}
	// Check for signin button.
	signInButton := d.Object(ui.ID(signInID))
	if err := signInButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Error("signInButton doesn't exists: ", err)
	}
	// Click on signIn Button until not now button exist.
	signInButton = d.Object(ui.ID(signInID))
	notNowButton := d.Object(ui.ID(notNowID))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := notNowButton.Exists(ctx); err != nil {
			signInButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Log("notNowButton doesn't exist: ", err)
	} else if err := notNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on notNowButton: ", err)
	}

	// Click on turnon button to save password.
	turnonButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+turnonText))
	if err := turnonButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("turnonButton doesn't exists: ", err)
	} else if err := turnonButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on turnonButton: ", err)
	}

	// Click on allow button for accessing files.
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+allowButtonText))
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("allowButton Button doesn't exists: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton Button: ", err)
	}

	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("allowButton Button doesn't exists: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton Button: ", err)
	}

	// Click on allow while using this app button to record audio.
	clickOnWhileUsingThisAppButton := d.Object(ui.TextMatches("(?i)" + whileUsingThisAppButtonText))
	if err := clickOnWhileUsingThisAppButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnWhileUsingThisApp Button doesn't exists: ", err)
	} else if err := clickOnWhileUsingThisAppButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnWhileUsingThisApp Button: ", err)
	}

	// Click on allow button for accessing videos.
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("allowButton Button doesn't exists: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton Button: ", err)
	}

	// Click on allow while using this app button to record video.
	clickOnWhileUsingThisAppButton = d.Object(ui.TextMatches("(?i)" + whileUsingThisAppButtonText))
	if err := clickOnWhileUsingThisAppButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnWhileUsingThisApp Button doesn't exists: ", err)
	} else if err := clickOnWhileUsingThisAppButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnWhileUsingThisApp Button: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)

	// Click on slideDownIcon.
	clickOnSlideDownIcon := d.Object(ui.ID(slideIconID))
	if err := clickOnSlideDownIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnSlideDownIcon Button doesn't exists: ", err)
	} else if err := clickOnSlideDownIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnSlideDownIcon Button: ", err)
	}

	// Check for profile icon.
	profileIcon := d.Object(ui.ID(profileID))
	if err := profileIcon.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Fatal("profileIcon doesn't exist: ", err)
	}
}

// loginToSnapchat login to app.
func loginToSnapchat(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		enterEmailAddressID  = "com.snapchat.android:id/username_or_email_field"
		loginButtonClassName = "android.widget.TextView"
		loginText            = "Log In"
		passwordID           = "com.snapchat.android:id/password_field"
	)

	// click on login button until emailAddress exists.
	loginButton := d.Object(ui.ClassName(loginButtonClassName), ui.TextMatches("(?i)"+loginText))
	enterEmailAddress := d.Object(ui.ID(enterEmailAddressID))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := enterEmailAddress.Exists(ctx); err != nil {
			loginButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Error("enterEmailAddress button doesn't exists: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	}

	// Click on emailid text field until the emailid text field is focused.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if emailIDFocused, err := enterEmailAddress.IsFocused(ctx); err != nil {
			return errors.New("email text field not focused yet")
		} else if !emailIDFocused {
			enterEmailAddress.Click(ctx)
			return errors.New("email text field not focused yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Fatal("Failed to focus EmailId: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	username := s.RequiredVar("arcappcompat.Snapchat.username")
	if err := kb.Type(ctx, username); err != nil {
		s.Fatal("Failed to enter username: ", err)
	}
	s.Log("Entered username")

	// Enter password.
	enterPassword := d.Object(ui.ID(passwordID))
	if err := enterPassword.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Error("EnterPassword does not exist: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	}
	// Click on password text field until the password text field is focused.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if pwdFocused, err := enterPassword.IsFocused(ctx); err != nil {
			return errors.New("password text field not focused yet")
		} else if !pwdFocused {
			enterPassword.Click(ctx)
			return errors.New("password text field not focused yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Fatal("Failed to focus password: ", err)
	}

	password := s.RequiredVar("arcappcompat.Snapchat.password")
	if err := kb.Type(ctx, password); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}
	s.Log("Entered password")
}

// loginToSnapchatWithOtherUI login to app.
func loginToSnapchatWithOtherUI(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		editTextClassName    = "com.snapchat.android:id/input_field_edit_text"
		loginButtonClassName = "android.widget.TextView"
		loginText            = "Log In"
	)

	// click on login button until emailAddress exists.
	loginButton := d.Object(ui.ClassName(loginButtonClassName), ui.TextMatches("(?i)"+loginText))
	enterEmailAddress := d.Object(ui.ClassName(editTextClassName))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := enterEmailAddress.Exists(ctx); err != nil {
			loginButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Error("enterEmailAddress button doesn't exists: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	}

	// Click on emailid text field until the emailid text field is focused.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if emailIDFocused, err := enterEmailAddress.IsFocused(ctx); err != nil {
			return errors.New("email text field not focused yet")
		} else if !emailIDFocused {
			enterEmailAddress.Click(ctx)
			return errors.New("email text field not focused yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Fatal("Failed to focus EmailId: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	username := s.RequiredVar("arcappcompat.Snapchat.username")
	if err := kb.Type(ctx, username); err != nil {
		s.Fatal("Failed to enter username: ", err)
	}
	s.Log("Entered username")

	// Enter password.
	enterPassword := d.Object(ui.ClassName(editTextClassName))
	if err := enterPassword.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Error("EnterPassword does not exist: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	}
	// Click on password text field until the password text field is focused.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if pwdFocused, err := enterPassword.IsFocused(ctx); err != nil {
			return errors.New("password text field not focused yet")
		} else if !pwdFocused {
			enterPassword.Click(ctx)
			return errors.New("password text field not focused yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Fatal("Failed to focus password: ", err)
	}

	// Press tab to click on enter password.
	if err := d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0); err != nil {
		s.Log("Failed to enter KEYCODE_TAB: ", err)
	}
	password := s.RequiredVar("arcappcompat.Snapchat.password")
	if err := kb.Type(ctx, password); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}
	s.Log("Entered password")
}
