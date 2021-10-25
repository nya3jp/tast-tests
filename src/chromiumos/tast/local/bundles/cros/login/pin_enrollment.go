// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PinEnrollment,
		Desc:         "Test pin enrollment",
		Contacts:     []string{"rsorokin@google.com", "chromeos-sw-engprod@google.com", "cros-oac@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
	})
}

func PinEnrollment(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx,
		chrome.ExtraArgs("--force-tablet-mode=touch_view"),
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

	if err := oobeConn.Eval(ctx, "OobeAPI.setCurrentScreen('pin-setup')", nil); err != nil {
		s.Fatal("Failed to click welcome page next button: ", err)
	}

	if err := oobeConn.WaitForExprFailOnErr(ctx, "!document.querySelector('#pin-setup').hidden"); err != nil {
		s.Fatal("Failed to wait for the pin screen: ", err)
	}

	const pin = "654321"

	for i := 0; i < 2; i++ {
		var step string
		if i == 0 {
			step = "start"
		} else {
			step = "confirm"
		}

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

	if err := oobeConn.Eval(ctx, "OobeAPI.finishOobe()", nil); err != nil {
		s.Fatal("Failed to call OobeAPI.finishOobe(): ", err)
	}

	cr.WaitForOOBEConnectionToBeDismissed(ctx)

	cr, err = chrome.New(ctx,
		chrome.ExtraArgs("--force-tablet-mode=touch_view", "--skip-force-online-signin-for-testing"),
		chrome.NoLogin(),
		chrome.KeepState(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")))

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting test Signin Profile API connection failed: ", err)
	}
	// Enter and submit the PIN to unlock the DUT.
	if err := lockscreen.EnterPIN(ctx, tconn, pin); err != nil {
		s.Fatal("Failed to enter in PIN: ", err)
	}

	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.LoggedIn }, 30*time.Second); err != nil {
		s.Fatalf("Waiting for login failed: %v (last status %+v)", err, st)
	}
}
