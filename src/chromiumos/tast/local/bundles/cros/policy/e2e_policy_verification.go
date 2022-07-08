// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

type systemE2eTestCase struct {
	// username of the OTA.
	username string
	// password of the OTA.
	password string
	// dmserverOption specifies what DM server to use. Use chrome.ProdPolicy()
	// or chrome.DMSPolicy(dmServerURL) to target custom instance.
	dmserverOption chrome.Option
	// policies and their values that are expected on the account. Those should
	// be related to the OTA you configured for the test.
	policies []policy.Policy
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         E2ePolicyVerification,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Test enrolls and signs in with owned test account configured in a way to have specific policies set",
		Contacts: []string{
			"kamilszarek@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "reboot"},
		Fixture:      fixture.CleanOwnership,
		VarDeps: []string{
			"policy.E2e_autoupdate_username",
			"policy.E2e_autoupdate_password",
			"policy.E2e_devicesignin_username",
			"policy.E2e_devicesignin_password",
		},
		Timeout: 2 * time.Minute,
		Params: []testing.Param{
			// do NOT add more tests as TPM will not be clear between execution.
			// Once the fixture will be able to execute Reset cleaning ownerhip
			// this test can be expanded with other accounts checks.
			{
				Name: "autoupdate",
				Val: systemE2eTestCase{
					username:       "policy.E2e_autoupdate_username",
					password:       "policy.E2e_autoupdate_password",
					dmserverOption: chrome.ProdPolicy(),
					policies: []policy.Policy{ // Put only policies related to auto update.
						&policy.ChromeOsReleaseChannelDelegated{Val: true},
						&policy.DeviceAutoUpdateDisabled{Val: true},
						&policy.DeviceRollbackToTargetVersion{Val: 3},
						&policy.DeviceUpdateHttpDownloadsEnabled{Val: true},
						&policy.DeviceUpdateScatterFactor{Val: 0},
						&policy.RebootAfterUpdate{Val: true},
						&policy.DeviceGuestModeEnabled{Val: true},
					},
				},
			},
			{
				Name: "device_signin",
				Val: systemE2eTestCase{
					username:       "policy.E2e_devicesignin_username",
					password:       "policy.E2e_devicesignin_password",
					dmserverOption: chrome.ProdPolicy(),
					policies: []policy.Policy{ // Put only policies related to Device settings -> Sign in section.
						&policy.DeviceGuestModeEnabled{Val: false},
						&policy.DeviceAllowNewUsers{Val: false, Stat: policy.StatusDefault},
						&policy.DeviceLoginScreenDomainAutoComplete{Val: "", Stat: policy.StatusDefault},
						&policy.DeviceLoginScreenDefaultHighContrastEnabled{Val: false},
						&policy.DeviceLoginScreenDefaultLargeCursorEnabled{Val: false},
						&policy.DeviceLoginScreenDefaultScreenMagnifierType{Val: 0},
						&policy.DeviceLoginScreenDefaultSpokenFeedbackEnabled{Val: false},
					},
				},
			},
		},
	})
}

func E2ePolicyVerification(ctx context.Context, s *testing.State) {
	tc := s.Param().(systemE2eTestCase)
	username := s.RequiredVar(tc.username)
	password := s.RequiredVar(tc.password)

	cr, err := chrome.New(ctx,
		chrome.GAIAEnterpriseEnroll(chrome.Creds{User: username, Pass: password}),
		chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
		tc.dmserverOption,
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	// Ensure chrome://policy shows correct policy values.
	if err := policyutil.Verify(ctx, tconn, tc.policies); err != nil {
		s.Error("Wrong policy value: ", err)
	}
}
