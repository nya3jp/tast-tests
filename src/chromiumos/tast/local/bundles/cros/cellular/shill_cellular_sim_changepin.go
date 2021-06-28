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
		Func:     ShillCellularSimChangePin,
		Desc:     "Verifies that Cellular Device SIM PIN lock change",
		Contacts: []string{"srikanthkumar@google.com", "chromeos-cellular-team@google.com"},
		Attr:     []string{"group:cellular", "cellular_unstable", "cellular_sim_dual_active"},
		Fixture:  "cellular",
	})
}

// ShillCellularSimChangePin tests successfully changing SIM lock pin.
func ShillCellularSimChangePin(ctx context.Context, s *testing.State) {
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
	tempPin := "1111"
	currentPuk := modem.GetActiveSimPuk(ctx)
	testing.ContextLog("Attempting to enable SIM lock with correct pin.")
	// Check if not enabled PIN and set PIN
	if helper.IsSimLockEnabled(ctx) && helper.IsSimPinLocked(ctx) {
		// Disable and remove default PIN
		err = helper.Device.RequirePin(currentPin, false)
		if err != nil {
			s.Fatal("Failed to disable lock: ", err)
		}
	}
	err = helper.Device.RequirePin(tempPin, true)
	if err != nil {
		s.Fatal("Failed to enable PIN: ", err)
	}
	enabled := helper.IsSimLockEnabled(ctx)
	if !enabled {
		s.Fatal("SIM lock was not enabled by temp pin: ", err)
	}
	err = helper.Device.ChangePin(ctx, tempPin, currentPin)
	if err != nil {
		s.Fatal("Failed to change pin: ", err)
	}
	// Reset modem or Reset dut with sim lock if needed
	enabled = helper.IsSimLockEnabled(ctx)
	if !enabled {
		s.Fatal("SIM lock not enabled by correct pin after reset: ", err)
	}
	locked, err := helper.IsSimPinLocked(ctx)
	if !locked {
		s.Fatal("SIM lock was not eanbled by correct pin: ", err)
	}
	if enabled || locked {
		// unlock and Disable pin lock
		err = helper.Device.RequirePin(ctx, currentPin, false)
		if err != nil {
			s.Fatal("Failed to disable pin lock to set dut normal: ", err)
		}
	}
}
