// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/bundles/cros/login/userutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/input"
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
		SearchFlags: []*testing.StringPair{{
			Key: "feature_id",
			// Credentials sync - password change failure.
			Value: "screenplay-e48269d3-5309-4db0-aafa-ffdce9a79dbf",
		}},
		Params: []testing.Param{{
			Name: "auth_factor_experiment_on",
			Val:  []chrome.Option{chrome.EnableFeatures("UseAuthFactors")},
		}, {
			Name: "auth_factor_experiment_off",
			Val:  []chrome.Option{chrome.DisableFeatures("UseAuthFactors")},
		}},
	})
}

func ChangePasswordFailure(ctx context.Context, s *testing.State) {
	var fakeCreds chrome.Creds
	var gaiaCreds chrome.Creds

	testParamOpts := s.Param().([]chrome.Option)

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
			append(testParamOpts, chrome.FakeLogin(fakeCreds))...)
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
			append(testParamOpts, chrome.GAIALogin(gaiaCreds),
				chrome.DontWaitForCryptohome(),
				chrome.KeepState(),
				chrome.RemoveNotification(false), // By default it waits for the user session.
				chrome.DontSkipOOBEAfterLogin())...)
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

	// Verify we can not login with the old password.
	loginWithCreds(ctx, s, testParamOpts, fakeCreds, false)

	// Verify we can login with the new password.
	loginWithCreds(ctx, s, testParamOpts, gaiaCreds, true)
}

func loginWithCreds(ctx context.Context, s *testing.State, testParamOpts []chrome.Option, creds chrome.Creds, successExpected bool) {
	cr, err := chrome.New(
		ctx,
		append(testParamOpts, chrome.NoLogin(),
			chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
			chrome.KeepState())...,
	)
	if err != nil {
		s.Fatal("Chrome start failed: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting signing test API connection failed: ", err)
	}

	if err = lockscreen.WaitForPasswordField(ctx, tconn, creds.User, 10*time.Second); err != nil {
		s.Fatal("Fail to wait for password: ", err)
	}

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get virtual keyboard: ", err)
	}
	defer keyboard.Close()
	if err = lockscreen.EnterPassword(ctx, tconn, creds.User, creds.Pass, keyboard); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}

	err = lockscreen.WaitForLoggedIn(ctx, tconn, chrome.LoginTimeout)
	if (err != nil && successExpected) || (err == nil && !successExpected) {
		s.Fatal("Login result does not satisfy expecation ")
	}
}
