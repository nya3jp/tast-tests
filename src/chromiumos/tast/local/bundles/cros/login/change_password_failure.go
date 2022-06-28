// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/bundles/cros/login/userutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChangePasswordFailure,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks cryptohome password change flow when user does not remember the old password",
		Contacts: []string{
			"emaamari@google.com",
			"cros-lurs@google.com",
			"chromeos-sw-engprod@google.com",
		},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
		},
		Attr: []string{"group:mainline", "informational"},
		VarDeps: []string{
			"ui.gaiaPoolDefault",
			"ui.signinProfileTestExtensionManifestKey",
		},
		Timeout: chrome.GAIALoginTimeout + 2*chrome.LoginTimeout + userutil.TakingOwnershipTimeout + time.Minute,
	})
}
func ChangePasswordFailure(ctx context.Context, s *testing.State) {
	var fakeCreds chrome.Creds
	var gaiaCreds chrome.Creds

	// Isolate the step to leverage `defer` pattern.
	func() {
		var err error
		gaiaCreds, err = chrome.PickRandomCreds(s.RequiredVar("ui.gaiaPoolDefault"))
		if err != nil {
			s.Fatal("Failed to parse creds: ", err)
		}

		fakeCreds = gaiaCreds
		// Add something to the password so when user logs in again - password change would be detected.
		fakeCreds.Pass = "fake" + fakeCreds.Pass
		cr, err := chrome.New(
			ctx,
			chrome.FakeLogin(fakeCreds))
		if err != nil {
			s.Fatal("Failed to create a user: ", err)
		}
		defer cr.Close(ctx)
		// This is needed for reven tests, as login flow there relies on the existence of a device setting.
		if err := userutil.WaitForOwnership(ctx, cr); err != nil {
			s.Fatal("User did not become device owner: ", err)
		}
	}()

	// Isolate the step to leverage `defer` pattern.
	func() {
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
		if err := oobeConn.Eval(ctx, "document.querySelector('#gaia-password-changed').$.forgotPasswordLink.click()", nil); err != nil {
			s.Fatal("Failed to click on forgot password link: ", err)
		}
		if err := oobeConn.Eval(ctx, "document.querySelector('#gaia-password-changed').$.proceedAnyway.click()", nil); err != nil {
			s.Fatal("Failed to click proceed anyway button: ", err)
		}
		if err := cr.WaitForOOBEConnectionToBeDismissed(ctx); err != nil {
			s.Fatal("Failed to wait for OOBE to be dismissed: ", err)
		}
	}()
}
