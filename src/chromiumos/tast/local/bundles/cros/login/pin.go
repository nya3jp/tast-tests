// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type testParam struct {
	OobeEnroll bool
	Autosubmit bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Pin,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test pin enrollment, pin unlock and pin login",
		Contacts:     []string{"rsorokin@google.com", "chromeos-sw-engprod@google.com", "cros-oac@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		Timeout:      2*chrome.LoginTimeout + 25*time.Second,
		Params: []testing.Param{{
			Name: "settings_enroll",
			Val:  testParam{false, false},
		}, {
			Name: "settings_enroll_autosubmit",
			Val:  testParam{false, true},
		}, {
			Name: "oobe_enroll",
			Val:  testParam{true, false},
		}, {
			Name: "oobe_enroll_autosubmit",
			Val:  testParam{true, true},
		}},
	})
}

func Pin(ctx context.Context, s *testing.State) {
	oobeEnroll := s.Param().(testParam).OobeEnroll
	autosubmit := s.Param().(testParam).Autosubmit
	pin := "1234566543210"
	if autosubmit {
		// autosubmit works for pins with len<=12 only
		pin = "654321"
	}
	func() {
		var cr *chrome.Chrome
		var err error
		var tconn *chrome.TestConn
		if oobeEnroll {
			cr, err = chrome.New(ctx,
				chrome.ExtraArgs(
					// Force pin screen during OOBE.
					"--force-tablet-mode=touch_view",
					"--vmodule=wizard_controller=1",
					// Disable VK so it does not get in the way of the pin pad.
					"--disable-virtual-keyboard"),
				chrome.DontSkipOOBEAfterLogin())

			if err != nil {
				s.Fatal("Chrome login failed: ", err)
			}
			defer cr.Close(ctx)
			oobeConn, err := cr.WaitForOOBEConnection(ctx)
			if err != nil {
				s.Fatal("Failed to create OOBE connection: ", err)
			}
			defer oobeConn.Close()

			if err := oobeConn.Eval(ctx, "OobeAPI.advanceToScreen('pin-setup')", nil); err != nil {
				s.Fatal("Failed to advance to the pin screen: ", err)
			}

			if err := oobeConn.WaitForExprFailOnErr(ctx, "!document.querySelector('#pin-setup').hidden"); err != nil {
				s.Fatal("Failed to wait for the pin screen: ", err)
			}

			for _, step := range []string{"start", "confirm"} {
				if err := oobeConn.WaitForExprFailOnErr(ctx, fmt.Sprintf("document.querySelector('#pin-setup').uiStep === '%s'", step)); err != nil {
					s.Fatalf("Failed to wait for %s step: %v", step, err)
				}
				if err := oobeConn.Eval(ctx, fmt.Sprintf("document.querySelector('#pin-setup').$.pinKeyboard.$.pinKeyboard.$.pinInput.value = '%s'", pin), nil); err != nil {
					s.Fatal("Failed to enter pin: ", err)
				}

				if err := oobeConn.Eval(ctx, "document.querySelector('#pin-setup').$.nextButton.click()", nil); err != nil {
					s.Fatal("Failed to click on the next button: ", err)
				}
			}
			if err := oobeConn.WaitForExprFailOnErr(ctx, "document.querySelector('#pin-setup').uiStep === 'done'"); err != nil {
				s.Fatal("Failed to wait for the done step: ", err)
			}

			if err := oobeConn.Eval(ctx, "OobeAPI.skipPostLoginScreens()", nil); err != nil {
				// This is not fatal because sometimes it fails because Oobe shutdowns too fast after the call - which produces error.
				s.Log("Failed to call skip post login screens: ", err)
			}

			if err := cr.WaitForOOBEConnectionToBeDismissed(ctx); err != nil {
				s.Fatal("Failed to wait for OOBE to be dismissed: ", err)
			}

			tconn, err = cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Getting test API connection failed: ", err)
			}
			defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
		} else {
			// Setup pin from the settings.
			// Disable VK so it does not get in the way of the pin pad.
			cr, err = chrome.New(ctx, chrome.ExtraArgs("--disable-virtual-keyboard"))
			if err != nil {
				s.Fatal("Chrome login failed: ", err)
			}
			defer cr.Close(ctx)

			tconn, err = cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Getting test API connection failed: ", err)
			}
			defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

			// Set up PIN through a connection to the Settings page.
			settings, err := ossettings.Launch(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to launch Settings app: ", err)
			}

			if err := settings.EnablePINUnlock(cr, cr.Creds().Pass, pin, autosubmit)(ctx); err != nil {
				s.Fatal("Failed to enable PIN unlock: ", err)
			}
		}

		// Lock the screen.
		if err := lockscreen.Lock(ctx, tconn); err != nil {
			s.Fatal("Failed to lock the screen: ", err)
		}

		if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, 30*time.Second); err != nil {
			s.Fatalf("Waiting for screen to be locked failed: %v (last status %+v)", err, st)
		}

		// Enter and submit the PIN to unlock the DUT.
		if err := lockscreen.EnterPIN(ctx, tconn, pin); err != nil {
			s.Fatal("Failed to enter in PIN: ", err)
		}

		if !autosubmit {
			if err := lockscreen.SubmitPIN(ctx, tconn); err != nil {
				s.Fatal("Failed to submit PIN: ", err)
			}
		}

		if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return !st.Locked }, 30*time.Second); err != nil {
			s.Fatalf("Waiting for screen to be unlocked failed: %v (last status %+v)", err, st)
		}
	}()

	cmdRunner := hwseclocal.NewCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)
	supportsLE, err := cryptohome.SupportsLECredentials(ctx)
	if err != nil {
		s.Fatal("Failed to get supported policies: ", err)
	}

	options := []chrome.Option{
		chrome.ExtraArgs("--skip-force-online-signin-for-testing"),
		chrome.NoLogin(),
		chrome.KeepState(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
	}
	if supportsLE {
		// Disable VK so it does not get in the way of the pin pad.
		options = append(options, chrome.ExtraArgs("--disable-virtual-keyboard"))
	}
	cr, err := chrome.New(ctx, options...)

	if err != nil {
		s.Fatal("Chrome start failed: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting test Signin Profile API connection failed: ", err)
	}

	if !supportsLE {
		if err := lockscreen.WaitForPasswordField(ctx, tconn, cr.Creds().User, 20*time.Second); err != nil {
			s.Fatal("Failed to wait for the password field: ", err)
		}
		keyboard, err := input.VirtualKeyboard(ctx)
		if err != nil {
			s.Fatal("Failed to get virtual keyboard: ", err)
		}
		defer keyboard.Close()
		if err = lockscreen.EnterPassword(ctx, tconn, cr.Creds().User, cr.Creds().Pass, keyboard); err != nil {
			s.Fatal("Failed to enter password: ", err)
		}
	} else {
		// Enter and submit the PIN to unlock the DUT.
		if err := lockscreen.EnterPIN(ctx, tconn, pin); err != nil {
			s.Fatal("Failed to enter in PIN: ", err)
		}

		if !autosubmit {
			if err := lockscreen.SubmitPIN(ctx, tconn); err != nil {
				s.Fatal("Failed to submit PIN: ", err)
			}
		}
	}

	if err := lockscreen.WaitForLoggedIn(ctx, tconn, chrome.LoginTimeout); err != nil {
		s.Fatal("Failed to login: ", err)
	}
}
