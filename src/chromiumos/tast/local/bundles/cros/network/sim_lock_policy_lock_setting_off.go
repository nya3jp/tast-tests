// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SimLockPolicyLockSettingOff,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test that SIM PIN locking is disabled when the SIM lock policy is applied",
		Contacts: []string{
			"hsuregan@google.com",
			"cros-connectivity@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:cellular", "cellular_unstable"},
		Fixture:      fixture.FakeDMSEnrolled,
		Timeout:      9 * time.Minute,
		Vars:         []string{"autotest_host_info_labels"},
	})
}

func SimLockPolicyLockSettingOff(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx,
		chrome.EnableFeatures("SimLockPolicy"),
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment())
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	// Perform clean up
	if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
		s.Fatal("Failed to clean up: ", err)
	}

	// Gather Shill Device sim properties.
	labels, err := cellular.GetLabelsAsStringArray(ctx, s.Var, "autotest_host_info_labels")
	if err != nil {
		s.Fatal("Failed to read autotest_host_info_labels: ", err)
	}

	helper, err := cellular.NewHelperWithLabels(ctx, labels)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	// Enable and get service to set autoconnect based on test parameters.
	if _, err := helper.Enable(ctx); err != nil {
		s.Fatal("Failed to enable modem")
	}

	iccid, err := helper.GetCurrentICCID(ctx)
	if err != nil {
		s.Fatal("Could not get current ICCID: ", err)
	}

	currentPin, currentPuk, err := helper.GetPINAndPUKForICCID(ctx, iccid)
	if err != nil {
		s.Fatal("Could not get Pin and Puk: ", err)
	}
	if currentPuk == "" {
		// Do graceful exit, not to run tests on unknown puk duts.
		s.Fatalf("Unable to find PUK code for ICCID : %s, skipping the test", iccid)
	}

	// Check if pin enabled and locked/set.
	if helper.IsSimLockEnabled(ctx) || helper.IsSimPinLocked(ctx) {
		// Disable pin.
		if err = helper.Device.RequirePin(ctx, currentPin, false); err != nil {
			s.Fatal("Failed to disable lock: ", err)
		}
	}

	networkName, err := helper.GetCurrentNetworkName(ctx)
	if err != nil {
		s.Fatal("Could not get name: ", err)
	}

	connectedProfile := nodewith.NameRegex(regexp.MustCompile("^Network [0-9] of [0-9],.*Details"))
	var networkNameDetail = nodewith.NameContaining(networkName).ClassName("subpage-arrow").Role(role.Button).Ancestor(connectedProfile.First())

	if err != nil {
		s.Fatal("Failed to ensure sim unlocked: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API in clean up: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	for _, param := range []struct {
		name                 string // subtest name.
		allowCellularSimLock bool   // Whether or not admin allow SIM lock
	}{
		{
			name:                 "Allow SIM lock",
			allowCellularSimLock: true,
		},
		{
			name:                 "Prohibit SIM lock",
			allowCellularSimLock: false,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			globalConfig := &policy.ONCGlobalNetworkConfiguration{
				AllowCellularSimLock: param.allowCellularSimLock,
			}

			deviceNetworkPolicy := &policy.DeviceOpenNetworkConfiguration{
				Val: &policy.ONC{
					GlobalNetworkConfiguration: globalConfig,
					NetworkConfigurations:      []*policy.ONCNetworkConfiguration{},
				},
			}

			// Apply Global Network Configuration.
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{deviceNetworkPolicy}); err != nil {
				s.Fatal("Failed to ServeAndRefresh ONC policy: ", err)
			}
			app, err := ossettings.OpenMobileDataSubpage(ctx, tconn, cr)
			if err != nil {
				s.Fatal("Failed to open mobile data subpage: ", err)
			}
			ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

			refreshProfileText := nodewith.NameStartingWith("Refreshing profile list").Role(role.StaticText)
			if err := app.WithTimeout(5 * time.Second).WaitUntilExists(refreshProfileText)(ctx); err == nil {
				s.Log("Wait until refresh profile finishes")
				if err := app.WithTimeout(5 * time.Minute).WaitUntilGone(refreshProfileText)(ctx); err != nil {
					s.Fatal("Failed to wait until refresh profile complete: ", err)
				}
			}

			if err := ui.WithTimeout(3 * time.Minute).WaitUntilExists(networkNameDetail)(ctx); err != nil {
				s.Fatal("Could not find connected mobile network: ", err)
			}

			if err := uiauto.Combine("Go to detail page and expand advanced section",
				ui.LeftClick(networkNameDetail),
				ui.LeftClick(ossettings.CellularAdvanced),
			)(ctx); err != nil {
				s.Fatal("Failed: ", err)
			}

			if param.allowCellularSimLock {
				if err := ui.CheckRestriction(ossettings.LockSimToggle, restriction.None)(ctx); err != nil {
					s.Fatal("Lock SIM card setting is disabled: ", err)
				}
			} else {
				if err := ui.CheckRestriction(ossettings.LockSimToggle, restriction.Disabled)(ctx); err != nil {
					s.Fatal("Lock SIM card setting is not disabled: ", err)
				}
			}

			if err := app.Close(ctx); err != nil {
				s.Fatal("Failed to close settings app: ", err)
			}
		})
	}
}
