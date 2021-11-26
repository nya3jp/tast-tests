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
		Func: ChangePassword,
		Desc: "Checks cryptohome password change flow",
		Contacts: []string{
			"rsorokin@google.com",
			"cros-oac@google.com",
			"chromeos-sw-engprod@google.com",
		},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
		},
		Attr: []string{"group:mainline", "informational"},
		VarDeps: []string{
			"ui.gaiaPoolDefault",
		},
		Timeout: chrome.GAIALoginTimeout + chrome.LoginTimeout + time.Minute,
	})
}

func ChangePassword(ctx context.Context, s *testing.State) {
	var gaiaCreds chrome.Creds
	var fakeCreds chrome.Creds
	func() {
		// We only need this to get the Gaia creds.
		cr, err := chrome.New(
			ctx,
			chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
			chrome.DeferLogin())
		if err != nil {
			s.Fatal("Failed to get config: ", err)
		}
		gaiaCreds = cr.Creds()
		cr.Close(ctx)
	}()

	func() {
		fakeCreds = gaiaCreds
		// Add something to the password so when user logs in again - password change would be detected.
		fakeCreds.Pass = "fake" + fakeCreds.Pass
		cr, err := chrome.New(
			ctx,
			chrome.FakeLogin(fakeCreds))
		if err != nil {
			s.Fatal("Failed to create a user: ", err)
		}
		cr.Close(ctx)
	}()
	cr, err := chrome.New(
		ctx,
		chrome.GAIALogin(gaiaCreds),
		chrome.DontWaitForCryptohome(),
		chrome.KeepState(),
		chrome.RemoveNotification(false), // By default it waits for the user session.
		chrome.DontSkipOOBEAfterLogin())
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)
	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		s.Fatal("Failed to wait for OOBE connection: ", err)
	}
	defer oobeConn.Close()
	if err := oobeConn.WaitForExprFailOnErrWithTimeout(ctx, "!document.querySelector('#gaia-password-changed').hidden", 10*time.Second); err != nil {
		s.Fatal("Failed to wait for the gaia password changed screen: ", err)
	}
	if err := oobeConn.Eval(ctx, fmt.Sprintf("document.querySelector('#gaia-password-changed').$.oldPasswordInput.value = '%s'", fakeCreds.Pass), nil); err != nil {
		s.Fatal("Failed to enter old password: ", err)
	}
	if err := oobeConn.Eval(ctx, "document.querySelector('#gaia-password-changed').$.next.click()", nil); err != nil {
		s.Fatal("Failed to click on next button: ", err)
	}
	if err := cr.WaitForOOBEConnectionToBeDismissed(ctx); err != nil {
		s.Fatal("Failed to wait for OOBE to be dismissed: ", err)
	}
}
