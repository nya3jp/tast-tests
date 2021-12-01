// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/stork"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CellularPolicy,
		Desc: "Test that eSIM profile can correctly be installed from device policy and the managed eSIM profile can not be removed or renamed",
		Contacts: []string{
			"jiajunz@google.com",
			"cros-connectivity@google.com@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:cellular", "cellular_unstable", "cellular_sim_test_esim"},
		Fixture:      fixture.FakeDMSEnrolled,
		Timeout:      5 * time.Minute,
	})
}

// networkFinder is the finder for the Network page UI in OS setting.
var networkFinder = nodewith.Name("Network").Role(role.Link).Ancestor(ossettings.WindowFinder)

// mobileButton is the finder for the Mobile Data page button UI in network page.
var mobileButton = nodewith.Name("Mobile data").Role(role.Button)

// testProfileDetailButton is the finder for the "Test Profile" detail subpage arrow button in the mobile data page UI.
var testProfileDetailButton = nodewith.NameStartingWith("Test Profile").Role(role.Button)

// tridots is the finder for the "More actions" button UI in cellular detail page.
var tridots = nodewith.Name("More actions").Role(role.Button)

// removeMenu is the finder for the Remove Profile menu item UI when click on the More actions menu.
var removeMenu = nodewith.Name("Remove Profile").Role(role.MenuItem)

// removeButton is the finder for the Remove eSIM Profile button UI when click on the "Remove Profile".
var removeButton = nodewith.NameStartingWith("Remove eSIM profile").Role(role.Button)

// renameButton is the finder for the Rename Profile menu item UI when click on the More action menu.
var renameMenu = nodewith.Name("Rename Profile").Role(role.MenuItem)

func CellularPolicy(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx,
		chrome.EnableFeatures("ESimPolicy", "UseStorkSmdsServerAddress", "CellularUseExternalEuicc"),
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment())
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui, err := openMobileDataSubpage(ctx, tconn, cr)
	if err != nil {
		s.Fatal("Failed to open mobile data subpage: ", err)
	}

	// Remove the existing stork eSIM profile if there's any.
	foundExistingTestProfile := true
	if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(testProfileDetailButton)(ctx); err != nil {
		s.Log("Did not find Test Profile, exit clean up: ", err)
		foundExistingTestProfile = false
	}

	if foundExistingTestProfile {
		s.Log("Find existing stork eSIM profile, attempt to remove it")
		// Wait for 3 seconds in case the mobile data page get reloaded.
		if err := testing.Sleep(ctx, 3*time.Second); err != nil {
			s.Fatal("Failed to sleep for 3 seconds: ", err)
		}
		if err := removeTestProfile(ctx, ui); err != nil {
			s.Fatal("Failed to remove existing Test Profile: ", err)
		}
		s.Log("Remove existing test profile completed")

		cr.Close(ctx)
		cr, err = chrome.New(ctx,
			chrome.EnableFeatures("ESimPolicy", "UseStorkSmdsServerAddress", "CellularUseExternalEuicc"),
			chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
			chrome.DMSPolicy(fdms.URL),
			chrome.KeepEnrollment())
		if err != nil {
			s.Fatal("Chrome login failed: ", err)
		}
		s.Log("Log out and back in to install stork eSIM profile")

		tconn, err = cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to connect Test API: ", err)
		}
		ui, err = openMobileDataSubpage(ctx, tconn, cr)
		if err != nil {
			s.Fatal("Failed to open mobile data subpage: ", err)
		}
	}

	activationCode, cleanupFunc, err := stork.FetchStorkProfile(ctx)
	if err != nil {
		s.Fatal("Failed to fetch Stork profile: ", err)
	}

	defer cleanupFunc(ctx)
	s.Log("Fetched Stork profile with activation code: ", activationCode)

	cellularONC := &policy.ONCCellular{
		SMDPAddress: string(activationCode),
	}

	globalConfig := &policy.ONCGlobalNetworkConfiguration{
		AllowOnlyPolicyCellularNetworks: false,
	}

	deviceProfileServiceGUID := "Cellular-Device-Policy"
	deviceNetworkPolicy := &policy.DeviceOpenNetworkConfiguration{
		Val: &policy.ONC{
			GlobalNetworkConfiguration: globalConfig,
			NetworkConfigurations: []*policy.ONCNetworkConfiguration{
				{
					GUID:     deviceProfileServiceGUID,
					Name:     "CellularDevicePolicyName",
					Type:     "Cellular",
					Cellular: cellularONC,
				},
			},
		},
	}

	s.Log("Running: restart hermes")
	output, err := testexec.CommandContext(ctx, "restart", "hermes").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Running command: restart hermes failed: ", err)
	}
	s.Log(string(output))
	// Wait 3 seconds for hermes to complete restart.
	if err := testing.Sleep(ctx, 3*time.Second); err != nil {
		s.Fatal("Failed to sleep for 3 seconds: ", err)
	}

	s.Log("Running: modem esim use_test_certs true")
	output, err = testexec.CommandContext(ctx, "modem", "esim", "use_test_certs", "true").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Running command: modem esim use_test_certs failed: ", err)
	}
	s.Log(string(output))

	// Wait 5 seconds for modem to complete set use_test_certs to true.
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep for 5 seconds: ", err)
	}

	if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{deviceNetworkPolicy}); err != nil {
		s.Fatal("Failed to ServeAndRefresh ONC policy: ", err)
	}
	s.Log("Applied device policy with cellular network configuration")

	if err := verifyTestESimProfileNotModifiable(ctx, ui); err != nil {
		s.Fatal("Failed to verify newly installed stork profile: ", err)
	}
	s.Log("Cellular policy test completed")
}

func openMobileDataSubpage(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) (*uiauto.Context, error) {
	ui := uiauto.New(tconn)

	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "Network", ui.Exists(networkFinder)); err != nil {
		return nil, errors.Wrap(err, "failed to launch settings page")
	}

	if err := uiauto.Combine("Go to mobile data page",
		ui.LeftClick(networkFinder),
		ui.LeftClick(mobileButton),
	)(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to go to mobile data page")
	}
	return ui, nil
}

func removeTestProfile(ctx context.Context, ui *uiauto.Context) error {
	if err := ui.LeftClick(testProfileDetailButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to left click Test Profile detail button")
	}

	if err := testing.Sleep(ctx, 3*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep for 3 seconds to load tridot")
	}

	if err := uiauto.Combine("Remove Test Profile",
		ui.LeftClick(tridots),
		ui.LeftClick(removeMenu),
		ui.LeftClick(removeButton),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to find or click on Remove Test Profile button")
	}

	// Wait for a minute to complete removal
	if err := testing.Sleep(ctx, time.Minute); err != nil {
		return errors.Wrap(err, "failed to sleep for one minute to make sure the eSIM Profile is removed")
	}
	return nil
}

func verifyTestESimProfileNotModifiable(ctx context.Context, ui *uiauto.Context) error {
	if err := ui.WithTimeout(time.Minute).LeftClick(testProfileDetailButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to left click Test Profile detail button")
	}

	if err := testing.Sleep(ctx, 3*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep for 3 seconds to load tridots")
	}

	if err := ui.LeftClick(tridots)(ctx); err != nil {
		return errors.Wrap(err, "failed to left click tridots button")
	}

	if err := ui.WithTimeout(3 * time.Second).WaitUntilExists(removeMenu)(ctx); err == nil {
		return errors.Wrap(err, "should not show Remove profile in tridot menu")
	}

	if err := ui.WithTimeout(3 * time.Second).WaitUntilExists(renameMenu)(ctx); err == nil {
		return errors.Wrap(err, "should not show Rename profile in tridot menu")
	}
	return nil
}
