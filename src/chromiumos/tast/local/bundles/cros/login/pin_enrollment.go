// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PinEnrollment,
		Desc:         "Test pin enrollment",
		Contacts:     []string{"rsorokin@google.com", "chromeos-sw-engprod@google.com", "cros-oac@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

func PinEnrollment(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx,
		chrome.ExtraArgs("--force-tablet-mode=touch_view", "--oobe-trigger-sync-timeout-for-tests"),
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

	if err := oobeConn.Eval(ctx, "document.querySelector('#sync-consent').$.nonSplitSettingsDeclineButton.click()", nil); err != nil {
		s.Fatal("Failed to skip sync consent screen: ", err)
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
		for _, c := range pin {
			if err := oobeConn.Eval(ctx, fmt.Sprintf("document.querySelector('#pin-setup').$.pinKeyboard.$.pinKeyboard.$.digitButton%c.click()", c), nil); err != nil {
				s.Fatalf("Failed to click on %c: %v", c, err)
			}
		}
		if err := oobeConn.Eval(ctx, "document.querySelector('#pin-setup').$.nextButton.click()", nil); err != nil {
			s.Fatal("Failed to click on the next button: ", err)
		}
		testing.Sleep(ctx, time.Second)
	}
	if err := oobeConn.WaitForExprFailOnErr(ctx, "document.querySelector('#pin-setup').uiStep === 'done'"); err != nil {
		s.Fatal("Failed to wait for the done step: ", err)
	}

	if err := oobeConn.Eval(ctx, "document.querySelector('#pin-setup').$.doneButton.click()", nil); err != nil {
		s.Fatal("Failed to click on the done button: ", err)
	}

	cr, err = chrome.New(ctx,
		chrome.ExtraArgs("--force-tablet-mode=touch_view", "--skip-force-online-signin-for-testing"),
		chrome.NoLogin(),
		chrome.KeepState())

	testing.Sleep(ctx, time.Second*30)
}
