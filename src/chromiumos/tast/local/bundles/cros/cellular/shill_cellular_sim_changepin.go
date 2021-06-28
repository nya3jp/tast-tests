// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCellularSimChangepin,
		Desc:     "Verifies that Cellular Device SIM PIN lock change",
		Contacts: []string{"srikanthkumar@google.com", "chromeos-cellular-team@google.com"},
		Attr:     []string{"group:cellular", "cellular_unstable", "cellular_sim_dual_active"},
		Fixture:  "cellular",
	})
}

// ShillCellularSimChangepin tests successfully changing SIM pin lock pin.
func ShillCellularSimChangepin(ctx context.Context, s *testing.State) {
	// Gather Shill Device SIM properties
	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	modem, err := modemmanager.NewModemWithSim(ctx)
	if err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}

	currentPin := mmconst.DefaultSimPin
	tempPin := mmconst.TempSimPin

	if currentPuk, err := modem.GetActiveSimPuk(ctx); err != nil {
		s.Fatal("Failed to get active sim puk: ", err)
	} else if currentPuk == "" {
		// Do graceful exit, not to run tests on unknown puk duts.
		return
	}

	// ResetModem needed for sim power reset to reflect locked type values.
	if _, err := helper.ResetModem(ctx); err != nil {
		s.Log("Failed to reset modem: ", err)
	}

	// Test sequence :
	// Enable Default Pin => Change Pin to Temp Pin => Check TempPin lockstates =>
	// Revert to Default Pin => Disable Default Pin.

	// Enable pin lock.
	s.Log("Calling requirepin to enable pin lock")
	if err = helper.Device.RequirePin(ctx, currentPin, true); err != nil {
		s.Fatal("Failed to enable lock: ", err)
	}

	s.Log("Attempting to change enabled sim lock with temporary pin: ", tempPin)
	if err = helper.Device.ChangePin(ctx, currentPin, tempPin); err != nil {
		helper.Device.RequirePin(ctx, currentPin, false)
		s.Fatal("Failed to change pin: ", err)
	}

	// ResetModem needed for sim power reset to reflect locked type values.
	if _, err = helper.ResetModem(ctx); err != nil {
		s.Log("Failed to reset modem: ", err)
	}

	enabled := helper.IsSimLockEnabled(ctx)
	if !enabled {
		s.Fatal("SIM lock not enabled by temp pin: ", err)
	}
	locked := helper.IsSimPinLocked(ctx)
	if !locked {
		s.Fatal("SIM lock was not locked by temp pin: ", err)
	}
	if enabled || locked {
		// Reverse to default pin.
		s.Log("Reverse pin to default pin from temppin")
		if err = helper.Device.ChangePin(ctx, tempPin, currentPin); err != nil {
			s.Fatal("Failed to change pin lock to default pin lock: ", err)
		}
		// Unlock and disable pin lock.
		if err = helper.Device.RequirePin(ctx, currentPin, false); err != nil {
			s.Fatal("Failed to disable default pin lock: ", err)
		}
	}
}
