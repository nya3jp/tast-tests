// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CellularConnectedStatus,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that SIM Lock in Settings PIN locks and unlocks the SIM",
		Contacts: []string{
			"hsuregan@google.com",
			"cros-connectivity@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:cellular", "cellular_unstable", "cellular_sim_dual_active"},
		Fixture:      "cellular",
		Vars:         []string{"autotest_host_info_labels"},
	})
}

func CellularConnectedStatus(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to create a new instance of Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	mdp, err := ossettings.OpenMobileDataSubpage(ctx, tconn, cr)
	if err != nil {
		s.Fatal("Failed to open mobile data subpage: ", err)
	}
	defer mdp.Close(ctx)

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}
	firstIccid, err := helper.GetCurrentICCID(ctx)
	if err != nil {
		s.Fatal("Could not get current ICCID: ", err)
	}

	if err := verifyNetworkIsActive(ctx, tconn, firstIccid); err != nil {
		s.Fatal("Failed to verify network is active: ", err)
	}

	if err := mdp.WaitUntilExists(ossettings.NotActiveCellularBtn)(ctx); err != nil {
		s.Fatal("Failed to find not connected network(s): ", err)
	}
	if err := mdp.LeftClick(ossettings.NotActiveCellularRows.First())(ctx); err != nil {
		s.Fatal("Failed to click into not active cellular row: ", err)
	}
	if err := waitUntilRefreshProfileCompletesX(ctx, tconn); err != nil {
		s.Fatal("Failed to wait until refresh profile complete: ", err)
	}

	internetSettingsURL := "chrome://os-settings/internet"
	matcher := chrome.MatchTargetURL(internetSettingsURL)
	if _, err := cr.NewConnForTarget(ctx, matcher); err == nil {
		s.Log("Connecting to a different network caused settings to navigate to top level network settings")
		mdp, err = ossettings.OpenMobileDataSubpage(ctx, tconn, cr)
		if err != nil {
			s.Fatal("Failed to open mobile data subpage: ", err)
		}
	}

	secondIccid, err := helper.GetCurrentICCID(ctx)
	if err != nil {
		s.Fatal("Could not get current ICCID: ", err)
	}

	if firstIccid == secondIccid {
		s.Fatal("Failed to connect to a different cellular network")
	}

	if err != nil {
		s.Fatal("Failed to open mobile data subpage: ", err)
	}
	if err := verifyNetworkIsActive(ctx, tconn, secondIccid); err != nil {
		s.Fatal("Failed to verify network is active: ", err)
	}
}

func verifyNetworkIsActive(ctx context.Context, tconn *chrome.TestConn, activeIccid string) error {
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	if err := waitUntilRefreshProfileCompletesX(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait until refresh profile complete")
	}

	if err := ui.WithTimeout(90 * time.Second).WaitUntilExists(ossettings.ActiveCellularBtn)(ctx); err != nil {
		return errors.Wrap(err, "failed to find active cellular network")
	}

	activeCellularRowNodes, err := ui.NodesInfo(ctx, ossettings.ActiveCellularRows)
	if err != nil {
		return errors.Wrap(err, "failed to find node info of active cellular network")
	}
	if len(activeCellularRowNodes) > 1 {
		return errors.Wrap(err, "more than one active network displayed as active")
	}

	if err := ui.LeftClick(ossettings.ActiveCellularBtn)(ctx); err != nil {
		return errors.Wrap(err, "failed to click into active cellular networks detail view")
	}

	if err := uiauto.IfFailThen(ui.WithTimeout(10*time.Second).WaitUntilExists(ossettings.ConnectedStatus), ui.WithTimeout(10*time.Second).WaitUntilExists(ossettings.SignInToNetwork))(ctx); err != nil {
		return errors.Wrap(err, "failed to verify active cellular network in details settings page")
	}

	displayedIccid := nodewith.NameContaining(activeIccid).Role(role.StaticText)
	if err := uiauto.Combine("Verify network connected",
		ui.WithTimeout(90*time.Second).LeftClick(ossettings.CellularAdvanced),
		ui.WithTimeout(90*time.Second).WaitUntilExists(displayedIccid),
		ui.LeftClick(ossettings.BackArrowBtn),
		ui.WaitUntilExists(ossettings.ActiveCellularBtn),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to verify network iccid in network details setting page")
	}
	return nil
}

// waitUntilRefreshProfileCompletesX - Replace with ossettings.WaitUntilRefreshProfileCompletes() once crrev/c/4007037 lands
func waitUntilRefreshProfileCompletesX(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn).WithTimeout(1 * time.Minute)
	refreshProfileText := nodewith.NameContaining("This may take a few minutes").Role(role.StaticText)
	if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(refreshProfileText)(ctx); err == nil {
		if err := ui.WithTimeout(5 * time.Minute).WaitUntilGone(refreshProfileText)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait until refresh profile complete")

		}
	}
	return nil
}
