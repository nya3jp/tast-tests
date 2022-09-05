// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterpriseconnectors

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

type userParam struct {
	username      string
	loginPossible bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DeviceTrust,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that Device Trust is working on Login Screen with a fake IDP",
		Contacts: []string{
			"lmasopust@google.com",
			"rodmartin@google.com",
			"cbe-device-trust-eng@google.com",
		},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
		},
		Attr: []string{
			"group:mainline", "informational",
		},
		Fixture: fixture.FakeDMSEnrolled,
		VarDeps: []string{
			"enterpriseconnectors.devicetrustusername1",
			"accountmanager.managedusername",
			"accountmanager.samlusername",
			"ui.signinProfileTestExtensionManifestKey",
		},
		Params: []testing.Param{{
			Val: userParam{
				username:      "enterpriseconnectors.devicetrustusername1",
				loginPossible: true,
			},
		}, {
			Name: "host_not_allowed",
			Val: userParam{
				username:      "accountmanager.managedusername",
				loginPossible: false,
			},
		}, {
			Name: "verified_access_also_specified",
			Val: userParam{
				username:      "accountmanager.managedusername",
				loginPossible: false,
			},
		}},
		Timeout: chrome.GAIALoginTimeout + time.Minute,
	})
}

const defaultUITimeout = 20 * time.Second

func activateDeviceTrustViaSettings(ctx context.Context, ui *uiauto.Context) error {
	root := nodewith.Name("IdP Settings").Role(role.RootWebArea)

	radioButtonYes := nodewith.Role(role.RadioButton).Ancestor(root).Nth(1)
	if err := uiauto.Combine("Click on yes",
		ui.WaitUntilExists(radioButtonYes),
		ui.LeftClick(radioButtonYes),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to click on yes in the IdP settings")
	}

	invalidateCheckbox := nodewith.Role(role.CheckBox).Ancestor(root).Focusable()
	if err := uiauto.Combine("Click on Invalidate",
		ui.WaitUntilExists(invalidateCheckbox),
		ui.LeftClick(invalidateCheckbox),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to click on Invalidate in the IdP settings")
	}

	saveButton := nodewith.Name("Save").Role(role.Button).Ancestor(root).Focusable()
	if err := uiauto.Combine("Click on Save",
		ui.WaitUntilExists(saveButton),
		ui.LeftClick(saveButton),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to click on Save in the IdP settings")
	}

	homeLink := nodewith.Name("Home").Role(role.Link).Ancestor(root).Focusable()
	if err := uiauto.Combine("Click on Home",
		ui.WaitUntilExists(homeLink),
		ui.LeftClick(homeLink),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to click on Home in the IdP settings")
	}

	return nil
}

func loginPossible(ctx context.Context, ui *uiauto.Context) (bool, error) {
	root := nodewith.Name("Sample Login page").Role(role.RootWebArea)
	loginButton := nodewith.Name("Login").Role(role.Button).Ancestor(root).Focusable()
	unsuccesfulText := nodewith.Name("Please login from a trusted device.").Role(role.StaticText).Ancestor(root)

	result := false
	err := testing.Poll(ctx, func(ctx context.Context) error {
		err := ui.Exists(loginButton)(ctx)
		if err == nil {
			result = true
			return nil
		}
		err = ui.Exists(unsuccesfulText)(ctx)
		if err == nil {
			result = false
			return nil
		}
		return errors.Wrap(err, " found neither the login button nor the text \"Please login from a trusted device\"")
	}, &testing.PollOptions{Interval: 300 * time.Millisecond,
		Timeout: defaultUITimeout})

	if err != nil {
		return false, err
	}

	return result, nil
}

func testFakeIDP(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	ui := uiauto.New(tconn).WithTimeout(defaultUITimeout)
	root := nodewith.Name("Enterprise NTP").Role(role.RootWebArea)

	//Activate device trust requirement
	settingsButton := nodewith.Name("Login Settings").Role(role.Link).Ancestor(root).Focusable()
	if err := uiauto.Combine("Go to settings",
		ui.WaitUntilExists(settingsButton),
		ui.LeftClick(settingsButton),
	)(ctx); err != nil {
		return false, errors.Wrap(err, "failed to go to settings")
	}
	if err := activateDeviceTrustViaSettings(ctx, ui); err != nil {
		return false, errors.Wrap(err, " failed to set settings")
	}

	testButton := nodewith.Name("App 1").Role(role.Link).Ancestor(root).Focusable()
	if err := uiauto.Combine("Click on OK and proceed",
		ui.WaitUntilExists(testButton),
		ui.LeftClick(testButton),
	)(ctx); err != nil {
		return false, errors.Wrap(err, "failed to click OK. Is Account addition dialog open?")
	}

	loginPossible, err := loginPossible(ctx, ui)
	if err != nil {
		return false, errors.Wrap(err, " failed to load results")
	}

	return loginPossible, nil
}

func DeviceTrust(ctx context.Context, s *testing.State) {
	fdms, ok := s.FixtValue().(*fakedms.FakeDMS)
	if !ok {
		s.Fatal("Parent is not a FakeDMS fixture")
	}

	param := s.Param().(userParam)
	username := s.RequiredVar(param.username)

	s.Log(username)

	var samlCreds chrome.Creds
	samlCreds.User = username

	pb := policy.NewBlob()
	pb.PolicyUser = username
	pb.DeviceAffiliationIds = []string{"default_affiliation_id"}
	pb.UserAffiliationIds = []string{"default_affiliation_id"}

	//set the policy
	deviceTrustPolicy := &policy.DeviceLoginScreenContextAwareAccessSignalsAllowlist{Val: []string{"https://cbe-integrationtesting-sandbox.uc.r.appspot.com/"}}
	pb.AddPolicy(deviceTrustPolicy)

	// We have to update fake DMS policy user and affiliation IDs before starting Chrome.
	if err := fdms.WritePolicyBlob(pb); err != nil {
		s.Fatal("Failed to write policy blob before starting Chrome: ", err)
	}

	cr, err := chrome.New(
		ctx,
		chrome.KeepEnrollment(),
		chrome.DMSPolicy(fdms.URL),
		chrome.DontSkipOOBEAfterLogin(),
		chrome.DontWaitForCryptohome(),
		chrome.RemoveNotification(false),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
		chrome.SAMLLogin(samlCreds),
		chrome.EnableFeatures("DeviceTrustConnectorEnabled"),
	)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating login test API connection failed: ", err)
	}

	if err := policyutil.Verify(ctx, tconn, []policy.Policy{deviceTrustPolicy}); err != nil {
		s.Fatal("Failed to refresh policies: ", err)
	}

	loginPossible, err := testFakeIDP(ctx, tconn)
	if err != nil {
		s.Fatal("Device Trust failed with error: ", err)
	}

	if loginPossible != param.loginPossible {
		s.Errorf("Unexpected value for loginPossible: %t, expected %t", loginPossible, param.loginPossible)
	}

	defer cr.Close(ctx)
}
