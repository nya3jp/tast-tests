// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCellularSimPUKLock,
		Desc:     "Verifies that Cellular Device SIM PUK lock",
		Contacts: []string{"srikanthkumar@google.com", "chromeos-cellular-team@google.com"},
		Attr:     []string{"group:cellular", "cellular_unstable", "cellular_sim_dual_active"},
		Fixture:  "cellular",
	})
}

// ShillCellularSimPUKLock tests successfully enabling SIM lock and locking the SIM with puk-lock.
func ShillCellularSimPUKLock(ctx context.Context, s *testing.State) {
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
	testing.ContextLog("Attempting to enable SIM lock and set in PUK lock state.")
	err = helper.PukLockSim(ctx, currentPin)
	if err != nil {
		s.Fatal("Failed to enable PUK lock: ", err)
	}

	// Reset modem or Reset dut with sim lock if not working in below way
	retriesLeft, err := helper.GetRetriesLeft(ctx)
	if err != nil {
		s.Fatal("Could not get pin retries left: ", err)
	}
	if retriesLeft <= 0 {
		s.Fatal("No retries left to try Error state after puk locked")
	}
	// get default puk, pin from initialize
	err = helper.Device.UnblockPUK(ctx, currentPuk, currentPin)
	if err != nil {
		s.Fatal("Could not unlock puk: ", err)
	}
	locked := helper.IsSimPukLocked(ctx)
	if locked {
		s.Fatal("Failed to unlock a puk-locked SIM with correct puk: ", err)
	}
	pinLocked := helper.IsSimPinLocked(ctx)
	if pinLocked {
		s.Fatal("pin-lock got unlocked while unlocking the puk-lock: ", err)
	}
	enabled := helper.IsSimLockEnabled(ctx)
	if !enabled {
		s.Fatal("SIM lock got disabled when attemping to unlock a pin-locked SIM: ", err)
	}

	// Reverse to default state
	enabled = helper.IsSimLockEnabled(ctx)
	locked = helper.IsSimPinLocked(ctx)
	if enabled || locked {
		// unlock and Disable pin lock
		err = helper.Device.RequirePin(ctx, currentPin, false)
		if err != nil {
			s.Fatal("Failed to disable pin lock to set dut normal: ", err)
		}
	}
}
