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
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/hermes"
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
	euicc, err := hermes.GetTestEUICC(ctx)
	if err != nil {
		s.Fatal("Failed to get test euicc: ", err)
	}

	if err := euicc.ResetMemory(ctx); err != nil {
		s.Fatal("Failed to reset test euicc: ", err)
	}
	s.Log("Reset test euicc completed")

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

	if err := euicc.UseTestCerts(ctx); err != nil {
		s.Fatal("Failed to set use test cert on test euicc: ", err)
	}
	// Wait 3 seconds for modem to complete set use_test_certs to true,
	// otherwise, the installation will fail.
	if err := testing.Sleep(ctx, 3*time.Second); err != nil {
		s.Fatal("Failed to wait for 3 seconds: ", err)
	}

	if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{deviceNetworkPolicy}); err != nil {
		s.Fatal("Failed to ServeAndRefresh ONC policy: ", err)
	}
	s.Log("Applied device policy with cellular network configuration")
	defer euicc.ResetMemory(ctx)

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
