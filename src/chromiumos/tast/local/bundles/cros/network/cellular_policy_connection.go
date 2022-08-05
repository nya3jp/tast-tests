// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/common/hermesconst"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/hermes"
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

const managedProfileName = "ManagedProfile"
const unmanagedProfileName = "UnmanagedProfile"

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

	managedIccid, err := getManagedProfileIccidBeforeTest(ctx)
	if err != nil {
		s.Fatal("Failed to connect to each cellular network before applying policy: ", err)
	}

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

			managedNetworks := nodewith.NameRegex(regexp.MustCompile(".*ManagedProfile,.*Managed by your Administrator.*")).Role(role.GenericContainer)
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
					s.Fatal("Add cellular button is not disabled")
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

// getManagedProfileIccidBeforeTest picks one of the profile's ICCID as the managed profile
// for the following test to use and disables all profiles in the euicc.
func getManagedProfileIccidBeforeTest(ctx context.Context) (managedIccid string, e error) {
	const prodSimSlotNum = 0
	euicc, err := hermes.NewEUICC(ctx, prodSimSlotNum)
	if err != nil {
		return "", errors.Wrap(err, "Unable to get Hermes euicc")
	}

	testing.ContextLog(ctx, "Looking for installed profile")
	profiles, err := euicc.InstalledProfiles(ctx, false)
	if err != nil {
		return "", errors.Wrap(err, "failed to get installed profiles")
	}
	if len(profiles) < 2 {
		return "", errors.Wrap(err, "no profiles found on euicc, expected atleast two installed profiles")
	}

	findUnmanagedProfile := false
	for _, profile := range profiles {
		props, err := dbusutil.NewDBusProperties(ctx, profile.DBusObject)
		iccid, err := props.GetString(hermesconst.ProfilePropertyIccid)
		if err != nil {
			return "", errors.Wrapf(err, "failed to read profile %s iccid", profile.String())
		}

		nickName, err := props.GetString(hermesconst.ProfilePropertyNickname)
		if err != nil {
			return "", errors.Wrapf(err, "failed to read profile %s nickname", profile.String())
		}

		state, err := props.GetInt32(hermesconst.ProfilePropertyState)
		if err != nil {
			return "", errors.Wrapf(err, "failed to read profile %s property state", profile.String())
		}

		if managedIccid == "" {
			managedIccid = iccid
			testing.ContextLogf(ctx, "Using managed profile iccid: %s", managedIccid)

			if state == hermesconst.ProfileStateEnabled {
				if err := profile.Call(ctx, hermesconst.ProfileMethodDisable).Err; err != nil {
					return "", errors.Wrapf(err, "failed to disable profile %s", profile.String())
				}
			}

			if nickName != managedProfileName {
				testing.ContextLogf(ctx, "Renaming profile %s to ManagedProfile", profile.String())
				if err := profile.Call(ctx, "Rename", managedProfileName).Err; err != nil {
					return "", errors.Wrapf(err, "failed to rename profile: %s", profile.String())
				}
			}
		} else if !findUnmanagedProfile {
			if nickName != unmanagedProfileName {
				testing.ContextLogf(ctx, "Renaming profile %s to UnmanagedProfile", profile.String())
				if err := profile.Call(ctx, "Rename", unmanagedProfileName).Err; err != nil {
					return "", errors.Wrapf(err, "failed to rename profile: %s", profile.String())
				}
			}
			testing.ContextLogf(ctx, "Using unmanaged profile iccid: %s", iccid)
			findUnmanagedProfile = true
		}
	}
	if !findUnmanagedProfile {
		return "", errors.Wrap(nil, "didn't find unmanaged profile")
	}

	return managedIccid, nil
}
