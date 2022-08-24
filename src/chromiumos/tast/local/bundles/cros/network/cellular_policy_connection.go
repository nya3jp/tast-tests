// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CellularPolicyConnection,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test that managed eSIM profile can be connected and disconnected and restrict managed only cellular network works properly",
		Contacts: []string{
			"jiajunz@google.com",
			"cros-connectivity@google.com@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:cellular", "cellular_unstable", "cellular_sim_prod_esim"},
		Fixture:      "cellularWithFakeDMSEnrolled",
		Timeout:      9 * time.Minute,
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

	managedProperties, unmanagedProperties, err := getProfilesBeforeTest(ctx)
	if err != nil {
		s.Fatal("Failed to connect to each cellular network before applying policy: ", err)
	}
	managedIccid := managedProperties.Iccid
	managedProfileName := managedProperties.Name
	unmanagedProfileName := unmanagedProperties.Name

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API in clean up: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

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
				ICCID: managedIccid,
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

			regex := fmt.Sprintf(".*%s,.*Managed by your Administrator.*", managedProfileName)
			managedNetworks := nodewith.NameRegex(regexp.MustCompile(regex)).Role(role.GenericContainer)
			infos, err := ui.NodesInfo(ctx, managedNetworks)
			if err != nil {
				s.Fatal("Failed to get nodes info: ", err)
			}

			managedNetworkWithBuildingIcon := managedNetworks.Nth(len(infos))
			managedDetail := nodewith.NameContaining(managedProfileName).ClassName("subpage-arrow").Role(role.Button)

			if err := uiauto.Combine("click to connect to the managed network and verify connected",
				ui.LeftClick(managedNetworkWithBuildingIcon),
				ui.WithTimeout(90*time.Second).LeftClick(managedDetail),
				ui.WaitUntilExists(ossettings.ConnectedStatus),
			)(ctx); err != nil {
				s.Fatal("Failed to click to connect to the managed network and verify connected: ", err)
			}

			if err := ui.WaitUntilExists(ossettings.RoamingToggle)(ctx); err != nil {
				s.Log("Got back to the network subpage from the detail page")
				if err := ui.LeftClick(managedDetail)(ctx); err != nil {
					s.Fatal("Couldn't go to managed network detail page")
				}
			}

			if err := uiauto.Combine("In the managed network detail page, disconnect and go back",
				ui.EnsureGoneFor(ossettings.ConnectingStatus, 5*time.Second),
				ui.LeftClick(ossettings.DisconnectButton),
				ui.WaitUntilExists(ossettings.DisconnectedStatus),
				ui.LeftClick(ossettings.BackArrowBtn),
			)(ctx); err != nil {
				s.Fatal("Failed to disconnect and go back in the managed network detail page: ", err)
			}

			if param.allowOnlyPolicyCellularNetworks {
				// make sure "Add Cellular" button is disabled
				if err := ui.CheckRestriction(ossettings.AddCellularButton, restriction.Disabled)(ctx); err != nil {
					s.Fatal("Add cellular button is not disabled: ", err)
				}
			}

			unmanagedNetwork := nodewith.NameContaining(unmanagedProfileName).Role(role.GenericContainer)
			unmanagedNetworkDetail := nodewith.NameContaining(unmanagedProfileName).ClassName("subpage-arrow").Role(role.Button)

			if param.allowOnlyPolicyCellularNetworks {
				// Click on unmanaged cellular should not attempt to make a connection, and it should bring you to the detail page
				if err := uiauto.Combine("click on unmanaged network and verify it doesn't get connected",
					ui.LeftClick(unmanagedNetwork),
					ui.WithTimeout(5*time.Second).WaitUntilExists(ossettings.DisconnectedStatus),
					ui.EnsureGoneFor(ossettings.ConnectButton, 5*time.Second),
				)(ctx); err != nil {
					s.Fatal("Failed to click on unmanaged network and verify it doesn't get connected: ", err)
				}
			} else {
				if err := uiauto.Combine("go to the unmanaged detail page, connect, and verify connected",
					ui.LeftClick(unmanagedNetworkDetail),
					ui.LeftClick(ossettings.ConnectButton),
					ui.WaitUntilExists(ossettings.ConnectedStatus),
				)(ctx); err != nil {
					s.Fatal("Failed to go to the unmanaged detail page, connect, and verify connected: ", err)
				}

				if err := ui.WaitUntilExists(ossettings.RoamingToggle)(ctx); err != nil {
					s.Log("Got back to the network subpage from the detail page")
					if err := ui.LeftClick(unmanagedNetworkDetail)(ctx); err != nil {
						s.Fatal("Couldn't go to unmanaged network detail page")
					}
				}

				if err := uiauto.Combine("In the unmanaged network detail page, disconnect and go back",
					ui.EnsureGoneFor(ossettings.ConnectingStatus, 5*time.Second),
					ui.LeftClick(ossettings.DisconnectButton),
					ui.WaitUntilExists(ossettings.DisconnectedStatus),
				)(ctx); err != nil {
					s.Fatal("Failed to disconnect and go back in the unmanaged network detail page: ", err)
				}
			}
			if err := app.Close(ctx); err != nil {
				s.Fatal("Failed to close settings app: ", err)
			}

			// Verify that the restrict managed network also works properly from quick settings
			s.Log("Start testing cellular connection from quick settings")
			if err := quicksettings.Expand(ctx, tconn); err != nil {
				s.Fatal("Fail to open quick settings")
			}

			networkFeaturePodLabelButton := nodewith.ClassName("FeaturePodLabelButton").NameContaining("network list")
			connectManagedNetwork := nodewith.ClassName("HoverHighlightView").NameStartingWith("Connect to " + managedProfileName)
			connectUnmanagedNetwork := nodewith.ClassName("HoverHighlightView").NameStartingWith("Connect to " + unmanagedProfileName)
			connectingToManagedNetwork := nodewith.ClassName("NetworkTrayView").NameStartingWith("Connecting to " + managedProfileName)
			connectingToUnmanagedNetwork := nodewith.ClassName("NetworkTrayView").NameStartingWith("Connecting to " + unmanagedProfileName)
			openUnmanagedNetwork := nodewith.ClassName("HoverHighlightView").NameStartingWith("Open settings for " + unmanagedProfileName)
			disableImage := nodewith.NameStartingWith("This network is disabled by your administrator").Role(role.Image).Ancestor(connectUnmanagedNetwork)

			if err := uiauto.Combine("Click managed network and unmanaged network from network list in quick setting",
				ui.LeftClick(networkFeaturePodLabelButton),
				ui.LeftClick(connectManagedNetwork),
				ui.WithTimeout(time.Minute).WaitUntilGone(connectingToManagedNetwork),
				ui.LeftClick(connectUnmanagedNetwork),
				ui.WithTimeout(time.Minute).WaitUntilGone(connectingToUnmanagedNetwork),
			)(ctx); err != nil {
				s.Fatal("Failed to click managed network and unmanaged network from network list in quick setting")
			}

			if param.allowOnlyPolicyCellularNetworks {
				if err := uiauto.Combine("Verify unmanaged cellular network is not connected",
					ui.WaitUntilExists(disableImage),
					ui.EnsureGoneFor(openUnmanagedNetwork, 3*time.Second),
				)(ctx); err != nil {
					s.Fatal("Should not connect unmanaged network: ", err)
				}
			} else {
				if err := ui.WaitUntilExists(openUnmanagedNetwork)(ctx); err != nil {
					s.Fatal("Failed to connect to unmanaged network: ", err)
				}
			}
		})
	}

	s.Log("Finish managed eSIM profile connection test")
}

func disconnect(ctx context.Context) error {
	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create cellular helper")
	}

	service, err := helper.FindService(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get connectable cellular service")
	}

	if connected, err := service.IsConnected(ctx); err != nil {
		return errors.Wrap(err, "failed to get the cellular service connected status")
	} else if connected {
		if _, err := helper.Disconnect(ctx); err != nil {
			return errors.Wrap(err, "failed to disconnect from cellular service")
		}
	}

	return nil
}

func getProfilesBeforeTest(ctx context.Context) (*network.CellularNetworkProperties, *network.CellularNetworkProperties, error) {
	testing.ContextLog(ctx, "disconnecting cellular connection")
	if err := disconnect(ctx); err != nil {
		return nil, nil, errors.Wrap(err, "failed to disconnect from cellular service")
	}

	networkProvider, err := network.NewCellularNetworkProvider(ctx, false)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to fetch cellular network")
	}

	testing.ContextLog(ctx, "renaming eSIM profiles")
	if err := networkProvider.RenameESimProfiles(ctx, "CellularNetwork"); err != nil {
		return nil, nil, errors.Wrap(err, "failed to rename all eSIM profiles")
	}

	networks, err := networkProvider.ESimNetworks(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to fetch eSIM networks")
	}

	if len(networks) < 2 {
		return nil, nil, errors.Errorf("Not enough eSIM profiles, expected: 2 got %d", len(networks))
	}

	managedNetwork := networks[0]
	managedProperties, err := managedNetwork.Properties(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to fetch managed network properties")
	}

	unmanagedNetwork := networks[1]
	unmanagedProperties, err := unmanagedNetwork.Properties(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to fetch unmanaged network properties")
	}

	testing.ContextLogf(ctx, "using unmanaged profile: %q, managed profile: %q", managedProperties.Name, unmanagedProperties.Name)
	return managedProperties, unmanagedProperties, nil
}
