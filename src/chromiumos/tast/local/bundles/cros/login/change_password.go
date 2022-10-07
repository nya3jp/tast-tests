// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/local/bundles/cros/login/userutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	testFile = "test_file"
	testData = "test that data persisted on the password change"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChangePassword,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks cryptohome password change flow",
		Contacts: []string{
			"anastasiian@google.com",
			"bohdanty@google.com",
			"cros-oac@google.com",
			"chromeos-sw-engprod@google.com",
		},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
		},
		Attr: []string{"group:mainline"},
		VarDeps: []string{
			"ui.gaiaPoolDefault",
			"ui.signinProfileTestExtensionManifestKey",
		},
		Timeout: chrome.GAIALoginTimeout + 2*chrome.LoginTimeout + userutil.TakingOwnershipTimeout + time.Minute,
		SearchFlags: []*testing.StringPair{{
			Key: "feature_id",
			// Credentials sync - successful password change.
			Value: "screenplay-1b766b3e-874a-49dd-be9d-5c63994970e3",
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

func ChangePassword(ctx context.Context, s *testing.State) {
	var fakeCreds chrome.Creds
	var gaiaCreds chrome.Creds
	var normalizedUser string

	testParamOpts := s.Param().([]chrome.Option)

	cmdRunner := hwseclocal.NewCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)

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
		normalizedUser = cr.NormalizedUser()
		if err := hwsec.WriteUserTestContent(ctx, cryptohome, cmdRunner, normalizedUser, testFile, testData); err != nil {
			s.Fatal("Failed to write a user test file: ", err)
		}
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
		if err := oobeConn.Eval(ctx, "document.querySelector('#gaia-password-changed').$.next.click()", nil); err != nil {
			s.Fatal("Failed to click on next button: ", err)
		}
		if err := cr.WaitForOOBEConnectionToBeDismissed(ctx); err != nil {
			s.Fatal("Failed to wait for OOBE to be dismissed: ", err)
		}

		// Write test file to check that data persisted on password change.
		if content, err := hwsec.ReadUserTestContent(ctx, cryptohome, cmdRunner, normalizedUser, testFile); err != nil {
			s.Fatal("Failed to read a user test file: ", err)
		} else if !bytes.Equal(content, []byte(testData)) {
			s.Fatalf("Unexpected test file content: got %q, want %q", content, testData)
		}
	}()

	// Login again with the updated password.
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

	if err = lockscreen.WaitForPasswordField(ctx, tconn, gaiaCreds.User, 10*time.Second); err != nil {
		s.Fatal("Fail to wait for password: ", err)
	}

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get virtual keyboard: ", err)
	}
	defer keyboard.Close()
	if err = lockscreen.EnterPassword(ctx, tconn, gaiaCreds.User, gaiaCreds.Pass, keyboard); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}

	if err := lockscreen.WaitForLoggedIn(ctx, tconn, chrome.LoginTimeout); err != nil {
		s.Fatal("Failed to login: ", err)
	}

	if content, err := hwsec.ReadUserTestContent(ctx, cryptohome, cmdRunner, normalizedUser, testFile); err != nil {
		s.Fatal("Failed to read a user test file: ", err)
	} else if !bytes.Equal(content, []byte(testData)) {
		s.Fatalf("Unexpected test file content: got %q, want %q", content, testData)
	}
}
