// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/local/bundles/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCellularSimFailedEnablePinLock,
		Desc:     "Verifies that Cellular Device SIM lock not enabled with incorrect PIN",
		Contacts: []string{"srikanthkumar@google.com", "chromeos-cellular-team@google.com"},
		Attr:     []string{"group:cellular", "cellular_unstable", "cellular_sim_dual_active"},
		Fixture:  "cellular",
	})
}

// ShillCellularSimFailedEnablePinLock checks unsuccessful enabling of SIM lock
func ShillCellularSimFailedEnablePinLock(ctx context.Context, s *testing.State) {
	// Gather Shill Device SIM properties
	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper")
	}
	deviceProps, err := helper.Device.GetProperties(ctx)
	if err != nil {
		s.Fatal("Failed to get Device properties: ", err)
	}
	if simPresent, err := deviceProps.GetBool(shillconst.DevicePropertyCellularSIMPresent); err != nil {
		s.Fatal("Failed to get Device.Cellular.SIMPresent property: ", err)
	} else if !simPresent {
		s.Fatal("SIMPresent property not set")
	}
	modem, err := modemmanager.NewModemWithSim(ctx)
	if err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}
	currentPin := mmconst.DefaultSimPin
	currentPuk := modem.GetActiveSimPuk(ctx)
	testing.ContextLog("Attempting to enable SIM lock with incorrect pin.")
	// Check if PIN is not enabled and try to set incorrect PIN
	if helper.IsSimLockEnabled(ctx) && helper.IsSimPinLocked(ctx) {
		// Disable and remove PIN
		err = helper.Device.RequirePin(currentPin, false)
		if err != nil {
			s.Fatal("Failed to disable lock: ", err)
		}
	}

	badPin, err := helper.BadPin(ctx, currentPin)
	err = helper.Device.RequirePin(badPin, true)
	if err != shillconst.ErrorIncorrectPin {
		s.Fatal("Failed to get expected error with incorrect pin: ", err)
	} else {
		testing.ContextLog("Got expected error pin lock enable failed with incorrect pin")
	}

	enabled := helper.IsSimLockEnabled(ctx)
	if enabled {
		s.Fatal("SIM lock got enabled by incorrect pin: ", err)
	}
	// Reset modem or Reset dut if needed, by this time those bugs must be resolved
	enabled = helper.IsSimLockEnabled(ctx)

	locked := helper.IsSimPinLocked(ctx)
	pukLocked := helper.IsSimPukLocked(ctx)
	if enabled || locked || pukLocked {
		// Disable pin lock and unlock
		err = helper.Device.RequirePin(ctx, badPin, false)
		s.Fatal("Cellular device able to get locked by an incorrect pin: ", err)
	} else {
		testing.ContextLog("After reset: Got expected error pin lock enable failed with incorrect pin")
	}
}
