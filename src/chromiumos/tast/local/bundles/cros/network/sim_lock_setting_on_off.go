// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SimLockSettingOnOff,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that SIM Lock in Settings PIN locks and unlocks the SIM",
		Contacts: []string{
			"hsuregan@google.com",
			"cros-connectivity@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:cellular", "cellular_unstable", "cellular_sim_pinlock"},
		Fixture:      "cellular",
		Vars:         []string{"autotest_host_info_labels"},
	})
}

func SimLockSettingOnOff(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to create a new instance of Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Gather Shill Device sim properties.
	labels, err := cellular.GetLabelsAsStringArray(ctx, s.Var, "autotest_host_info_labels")
	if err != nil {
		s.Fatal("Failed to read autotest_host_info_labels: ", err)
	}

	// Get cellular helper, used to determine if SIM is actually PIN locked/unlocked
	helper, err := cellular.NewHelperWithLabels(ctx, labels)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	app, err := ossettings.OpenMobileDataSubpage(ctx, tconn, cr)
	if err != nil {
		s.Fatal("Failed to open mobile data subpage: ", err)
	}

	defer app.Close(ctx)

	networkName, err := helper.GetCurrentNetworkName(ctx)
	if err != nil {
		s.Fatal("Could not get name: ", err)
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

	var networkNameDetail = nodewith.NameContaining(networkName).Role(role.Button).ClassName("subpage-arrow").First()
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kb.Close()

	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)
	if err := uiauto.Combine("Toggle on the SIM Lock setting",
		ui.LeftClick(networkNameDetail),
		ui.WaitUntilExists(ossettings.ConnectedStatus),
		ui.WithTimeout(90*time.Second).LeftClick(ossettings.CellularAdvanced),
		ui.LeftClick(ossettings.LockSimToggle),
		ui.WaitUntilExists(ossettings.EnterButton),
		kb.TypeAction(currentPin),
		ui.LeftClick(ossettings.EnterButton),
		ui.WaitUntilExists(ossettings.LockSimToggle.Focusable().Focused()),
	)(ctx); err != nil {
		s.Fatal("Failed: ", err)
	}

	if !helper.IsSimLockEnabled(ctx) {
		s.Fatal("Failed to turn on PIN lock")
	}

	if err := uiauto.Combine("Toggle off the SIM Lock setting",
		ui.LeftClick(ossettings.LockSimToggle),
		ui.WaitUntilExists(ossettings.EnterButton),
		kb.TypeAction(currentPin),
		ui.LeftClick(ossettings.EnterButton),
		ui.WaitUntilExists(ossettings.LockSimToggle.Focusable().Focused()),
	)(ctx); err != nil {
		s.Fatal("Failed: ", err)
	}

	if helper.IsSimLockEnabled(ctx) {
		s.Fatal("Failed to turn off PIN lock")
	}
}
