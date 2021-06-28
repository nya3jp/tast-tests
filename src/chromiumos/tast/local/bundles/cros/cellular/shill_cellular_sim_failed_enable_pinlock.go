// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"strings"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCellularSimFailedEnablePinlock,
		Desc:     "Verifies that Cellular Device SIM lock not enabled with incorrect PIN",
		Contacts: []string{"srikanthkumar@google.com", "chromeos-cellular-team@google.com"},
		Attr:     []string{"group:cellular", "cellular_unstable", "cellular_sim_dual_active"},
		Fixture:  "cellular",
	})
}

// ShillCellularSimFailedEnablePinlock checks unsuccessful enabling of SIM lock
func ShillCellularSimFailedEnablePinlock(ctx context.Context, s *testing.State) {
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
	currentPuk, err := modem.GetActiveSimPuk(ctx)
	if err != nil {
		s.Fatal("Failed to get active sim puk")
	}
	if currentPuk == "" {
		// Do graceful exit, not to run tests on unknow puk duts
		return
	}
	s.Log("Attempting to enable SIM lock with incorrect pin")
	// Check if PIN is not enabled and try to set incorrect PIN
	if helper.IsSimLockEnabled(ctx) && helper.IsSimPinLocked(ctx) {
		// Disable and remove PIN
		err = helper.Device.RequirePin(ctx, currentPin, false)
		if err != nil {
			s.Fatal("Failed to disable lock: ", err)
		}
	}

	badPin, err := helper.BadPin(ctx, currentPin)
	err = helper.Device.RequirePin(ctx, badPin, true)
	s.Log("Badpin used to lock device: ", badPin)
	if !strings.Contains(err.Error(), shillconst.ErrorIncorrectPin) {
		// Unlock dut and raise error
		helper.Device.RequirePin(ctx, badPin, false)
		s.Fatal("Failed to get expected error with incorrect pin: ", err)
	} else {
		s.Log("Got expected pin lock error for incorrect pin")
	}

	enabled := helper.IsSimLockEnabled(ctx)
	if enabled {
		s.Log("SIM lock got enabled by incorrect pin: ", badPin)
	}

	// Reverse pin lock status with badpin if still locked
	locked := helper.IsSimPinLocked(ctx)
	pukLocked := helper.IsSimPukLocked(ctx)
	if enabled || locked || pukLocked {
		// Disable pin lock and unlock
		err = helper.Device.RequirePin(ctx, badPin, false)
		s.Fatal("Cellular device able to get locked by an incorrect pin: ", err)
	}
}
