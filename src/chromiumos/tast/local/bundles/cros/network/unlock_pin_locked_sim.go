// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
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
		Func:         UnlockPinLockedSim,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that a PIN locked SIM can only be unlocked by the correct PIN, and then subsequently connected to",
		Contacts: []string{
			"hsuregan@google.com",
			"cros-connectivity@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		// Run test only on cellular capable devices that only have one active SIM.
		Attr:    []string{"group:cellular", "cellular_unstable", "cellular_sim_active"},
		Fixture: "cellular",
		Timeout: 8 * time.Minute,
		Vars:    []string{"autotest_host_info_labels"},
	})
}

func UnlockPinLockedSim(ctx context.Context, s *testing.State) {
	// Gather Shill Device SIM properties.
	labels, err := cellular.GetLabelsAsStringArray(ctx, s.Var, "autotest_host_info_labels")
	if err != nil {
		s.Fatal("Failed to read autotest_host_info_labels: ", err)
	}

	// Get cellular helper, used to determine if SIM is actually PIN locked/unlocked.
	helper, err := cellular.NewHelperWithLabels(ctx, labels)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	iccid, err := helper.GetCurrentICCID(ctx)
	if err != nil {
		s.Fatal("Could not get current ICCID: ", err)
	}

	currentPin, currentPuk, err := helper.GetPINAndPUKForICCID(ctx, iccid)
	if err != nil {
		s.Fatal("Could not get PIN and PUK: ", err)
	}
	if currentPin == "" {
		// Do graceful exit, not to run tests on unknown PIN duts.
		s.Fatal("Unable to find PIN code for ICCID: ", iccid)
	}
	if currentPuk == "" {
		// Do graceful exit, not to run tests on unknown puk duts.
		s.Fatal("Unable to find PUK code for ICCID: ", iccid)
	}

	// Check if PIN enabled and locked/set.
	if helper.IsSimPinLocked(ctx) {
		s.Fatal("Precondition of an unlocked SIM was not met")
	}

	// Shorten deadline to leave time for cleanup
	cleanupCtx := ctx
	cleanupCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	s.Log("Attempting to enable sim lock with correct pin")
	if err = helper.Device.RequirePin(ctx, currentPin, true); err != nil {
		s.Fatal("Failed to enable pin, mostly default pin needs to set on dut: ", err)
	}

	defer func(ctx context.Context) {
		// Unlock and disable pin lock.
		if err = helper.Device.RequirePin(ctx, currentPin, false); err != nil {
			// Unlock and disable pin lock if failed after locking pin.
			if errNew := helper.ClearSIMLock(ctx, currentPin, currentPuk); errNew != nil {
				s.Log("Failed to clear default pin lock: ", errNew)
			}
			s.Fatal("Failed to disable default pin lock: ", err)
		}
	}(cleanupCtx)

	// ResetModem needed for sim power reset to reflect locked type values.
	if _, err := helper.ResetModem(ctx); err != nil {
		s.Log("Failed to reset modem: ", err)
	}

	// Start a new chrome session
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to create a new instance of Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Opening the mobile data page via ossettings.OpenMobileDataSubpage() will not work if the device does not support multisim and the SIM is locked.
	settings, err := ossettings.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch OS settings: ", err)
	}

	ui := uiauto.New(tconn).WithTimeout(5 * time.Minute)

	goNavigateToMobileData := func() {
		mobileDataLinkNode := nodewith.HasClass("cr-title-text").Name("Mobile data").Role(role.Heading)
		if err := settings.NavigateToPageURL(ctx, cr, "networks?type=Cellular", ui.WaitUntilExists(mobileDataLinkNode)); err != nil {
			s.Fatal("Failed to navigate to mobile data page 1")
		}
	}

	goNavigateToMobileData()
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kb.Close()

	refreshProfileText := nodewith.NameStartingWith("Refreshing profile list").Role(role.StaticText)
	if err := settings.WithTimeout(5 * time.Second).WaitUntilExists(refreshProfileText)(ctx); err == nil {
		s.Log("Wait until refresh profile finishes")
		if err := settings.WithTimeout(5 * time.Minute).WaitUntilGone(refreshProfileText)(ctx); err != nil {
			s.Fatal("Failed to wait until refresh profile complete: ", err)
		}
	}

	var incorrectPinSublabel = nodewith.NameContaining("Incorrect PIN").Role(role.StaticText)
	if err := uiauto.Combine("Incorrect PIN does not unlock the SIM",
		ui.LeftClick(ossettings.UnlockButton),
		ui.WaitUntilExists(ossettings.CancelButton),

		// Use a bad PIN.
		kb.TypeAction(currentPin+"0"),
		ui.LeftClick(ossettings.UnlockButton),
		ui.WaitUntilExists(incorrectPinSublabel),
	)(ctx); err != nil {
		s.Fatal("Unlock button can still be clicked when incorrect PIN was entered: ", err)
	}

	networkName, err := helper.GetCurrentNetworkName(ctx)
	if err != nil {
		s.Fatal("Could not get the Network name: ", err)
	}

	if err := uiauto.Combine("Correct PIN unlocks the SIM, and subsequently clicking on the network row initiates a connection",
		kb.TypeAction(currentPin),
		ui.LeftClick(ossettings.UnlockButton),
	)(ctx); err != nil {
		s.Fatal("Failed to unlock the SIM: ", err)
	}

	goNavigateToMobileData()
	var networkNameDetail = nodewith.NameContaining(networkName).Role(role.Button).ClassName("subpage-arrow").First()
	if err := uiauto.Combine("Verify network connected",
		ui.WaitUntilExists(networkNameDetail),
		ui.LeftClick(networkNameDetail),
		ui.WaitUntilExists(ossettings.ConnectedStatus),
	)(ctx); err != nil {
		s.Fatal("Failed to verify network connected via settings: ", err)
	}
}
