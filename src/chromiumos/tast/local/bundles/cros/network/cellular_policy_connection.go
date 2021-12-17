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
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CellularPolicyConnection,
		Desc: "Test that managed eSIM profile can be connected and disconnected and restrict managed only cellular network works properly",
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

func CellularPolicyConnection(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx,
		chrome.EnableFeatures("ESimPolicy"),
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment())
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	// perform clean up
	if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
		s.Fatal("Failed to clean up: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API in clean up: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := connectEachCellularBeforeTest(ctx, tconn, cr); err != nil {
		s.Fatal("Failed to connect to each cellular network before applying policy: ", err)
	}

	for _, param := range []struct {
		name                            string // subtest name.
		allowOnlyPolicyCellularNetworks bool   // Whether or not admin allow only connecting to managed cellular network
	}{
		{
			name:                            "Allow managed and unmanaged cellular network",
			allowOnlyPolicyCellularNetworks: false,
		},
		{
			name:                            "Only allow managed cellular network to connect",
			allowOnlyPolicyCellularNetworks: true,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			cellularONC := &policy.ONCCellular{
				ICCID: "8901260380728064959",
			}

			globalConfig := &policy.ONCGlobalNetworkConfiguration{
				AllowOnlyPolicyCellularNetworks: param.allowOnlyPolicyCellularNetworks,
			}

			deviceProfileServiceGUID := "Cellular-Managed"
			deviceNetworkPolicy := &policy.DeviceOpenNetworkConfiguration{
				Val: &policy.ONC{
					GlobalNetworkConfiguration: globalConfig,
					NetworkConfigurations: []*policy.ONCNetworkConfiguration{
						{
							GUID:     deviceProfileServiceGUID,
							Name:     "CellularManaged",
							Type:     "Cellular",
							Cellular: cellularONC,
						},
					},
				},
			}

			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{deviceNetworkPolicy}); err != nil {
				s.Fatal("Failed to ServeAndRefresh ONC policy: ", err)
			}
			s.Log("Applied device policy with managed cellular network configuration")

			app, err := ossettings.OpenMobileDataSubpage(ctx, tconn, cr)
			if err != nil {
				s.Fatal("Failed to open mobile data subpage: ", err)
			}
			ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

			managedCellular := nodewith.NameContaining("Managed by your Administrator").Role(role.Button).ClassName("layout horizontal center flex")
			managedCellularDetail := nodewith.ClassName("subpage-arrow").Role(role.Button).Ancestor(managedCellular)

			if err := uiauto.Combine("connect and go to the managed cellular network detail page",
				ui.LeftClick(managedCellular),
				ui.LeftClick(managedCellularDetail),
			)(ctx); err != nil {
				s.Fatal("Failed to connect and disconnect unmanaged cellular network: ", err)
			}

			if err := uiauto.Combine("verify connected status and disconnect to the managed cellular network",
				ui.WithTimeout(3*time.Second).WaitUntilExists(ossettings.ConnectedStatus),
				ui.LeftClick(ossettings.DisconnectButton),
				ui.WaitUntilExists(ossettings.DisconnectedStatus),
			)(ctx); err != nil {
				s.Fatal("Failed to verify connected status and disconnect to the managed cellular network: ", err)
			}

			if err := ui.LeftClick(ossettings.BackArrowBtn)(ctx); err != nil {
				s.Fatal("Failed to go back to mobile data page")
			}

			if param.allowOnlyPolicyCellularNetworks {
				// make sure add cellular network button is disable
				if err := ui.WithTimeout(10 * time.Second).LeftClick(ossettings.AddCellularButton); err == nil {
					s.Fatal("Should not allow adding external cellular when allow only policy cellular network is enabled")
				}
			}

			unmanagedCellular := nodewith.NameStartingWith("Network 2 of").Role(role.Button).ClassName("layout horizontal center flex")
			unmanagedCellularDetail := nodewith.ClassName("subpage-arrow").Role(role.Button).Ancestor(unmanagedCellular)

			if param.allowOnlyPolicyCellularNetworks {
				// Click on unmanaged cellular should not attempt to make a connection, and it should bring you to the detail page
				if err := uiauto.Combine("click on unmanaged network and verify it doesn't get connected",
					ui.LeftClick(unmanagedCellular),
					ui.WithTimeout(3*time.Second).WaitUntilExists(ossettings.DisconnectedStatus),
					ui.EnsureGoneFor(ossettings.ConnectButton, 5*time.Second),
				)(ctx); err != nil {
					s.Fatal("Failed to click on unmanaged network and verify it doesn't get connected: ", err)
				}
			} else {
				if err := uiauto.Combine("go to detail page, connect and disconnect unmanaged cellular network",
					ui.LeftClick(unmanagedCellularDetail),
					ui.LeftClick(ossettings.ConnectButton),
					ui.WaitUntilExists(ossettings.ConnectedStatus),
					ui.WaitUntilExists(ossettings.RoamingToggle),
					ui.LeftClick(ossettings.DisconnectButton),
					ui.WaitUntilExists(ossettings.DisconnectedStatus),
				)(ctx); err != nil {
					s.Fatal("Failed to connect and disconnect unmanaged cellular network: ", err)
				}
			}

			app.Close(ctx)
		})
	}

	s.Log("Finish managed eSIM profile connection test")
}

func connectEachCellularBeforeTest(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) error {
	settingApp, err := ossettings.OpenMobileDataSubpage(ctx, tconn, cr)
	if err != nil {
		return errors.Wrap(err, "failed to navigate to mobile data page")
	}
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)
	testing.ContextLog(ctx, "Start connecting to each cellular to make sure Shill config is created")

	cellular1 := nodewith.NameStartingWith("Network 1 of").Role(role.Button).ClassName("layout horizontal center flex")
	cellular1Detail := nodewith.ClassName("subpage-arrow").Role(role.Button).Ancestor(cellular1)
	cellular2 := nodewith.NameStartingWith("Network 2 of").Role(role.Button).ClassName("layout horizontal center flex")
	cellular2Detail := nodewith.ClassName("subpage-arrow").Role(role.Button).Ancestor(cellular2)

	if err := ui.LeftClick(cellular1Detail)(ctx); err != nil {
		return errors.Wrap(err, "failed to find or click on the first cellular network subpage arrow")
	}

	// connect to the first network if it's not connected
	if err := ui.WithTimeout(3 * time.Second).WaitUntilExists(ossettings.ConnectedStatus)(ctx); err != nil {
		if err := uiauto.Combine("connect to the first cellular network",
			ui.LeftClick(ossettings.ConnectButton),
			ui.WaitUntilExists(ossettings.ConnectedStatus),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to connect to the first cellular network")
		}
	}

	if err := ui.LeftClick(ossettings.BackArrowBtn)(ctx); err != nil {
		return errors.Wrap(err, "failed to go back to mobile data page")
	}

	if err := uiauto.Combine("connect and disconnect second cellular network",
		ui.LeftClick(cellular2Detail),
		ui.LeftClick(ossettings.ConnectButton),
		ui.WaitUntilExists(ossettings.ConnectedStatus),
		ui.WaitUntilExists(ossettings.RoamingToggle),
		ui.LeftClick(ossettings.DisconnectButton),
		ui.WaitUntilExists(ossettings.DisconnectedStatus),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to connect and disconnect second cellular network")
	}

	testing.ContextLog(ctx, "Connect to each cellular networks completed")
	settingApp.Close(ctx)
	return nil
}
