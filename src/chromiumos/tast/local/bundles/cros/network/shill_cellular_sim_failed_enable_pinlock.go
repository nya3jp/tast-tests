// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/local/bundles/cros/network/cellular"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCellularSimFailedEnablePinLock,
		Desc:     "Verifies that Cellular Device SIM lock not enabled with incorrect PIN",
		Contacts: []string{"srikanthkumar@google.com", "chromeos-cellular-team@google.com"},
		Attr:     []string{"group:cellular"},
		Fixture:  "cellular",
	})
}

// ShillCellularSimFailedEnablePinLock checks unsuccessful enabling of SIM lock
func ShillCellularSimFailedEnablePinLock(ctx context.Context, s *testing.State) {
	// TODO:
	// cbb.InitDUT()
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
    // TODO: Get current pin and puk of the dut from cellular building blocks init
	// currentPin = cbb.dutPin
	// currentPuk = cbb.dutPuk
	currentPin := 1234 //Tmp
	testing.ContextLog("Attempting to enable SIM lock with incorrect pin.")
	// Check if PIN is not enabled and try to set incorrect PIN
	if (helper.Device.IsSimLockEnabled(ctx) && helper.Device.IsSimPinLocked(ctx)) {}
	    // Disable and remove PIN
		err = helper.Device.RequirePin(currentPin, false)
		if err != nil {
			s.Fatal("Failed to disable lock", err)
		}
	}

	badPin, err = helper.Device.BadPin(ctx, currentPin)
	err = helper.Device.RequirePin(badPin, true)
	if err != shillconst.ErrorIncorrectPin {
		s.Fatal("Failed to get expected error with incorrect pin", err)
	}
	else {
		testing.ContextLog("Got expected error pin lock enable failed with incorrect pin")
	}

	enabled, err = helper.Device.IsSimLockEnabled(ctx)
	if enabled {
		s.Fatal("SIM lock got enabled by incorrect pin.", err)
	}
	// Reset modem or Reset dut
	// SIM lock should not be enabled and lock should not set after reset.
	// TODO: Reset using building blocks
	// cbb.ReinitDUT()
	enabled, err = helper.Device.IsSimLockEnabled(ctx)

	locked, err = helper.Device.IsSimPinLocked(ctx)
    pukLocked, err = helper.Device.IsSimPukLocked(ctx)
	if enabled || locked || pukLocked {
		// Disable pin lock and unlock
		err = helper.Device.RequirePin(ctx, badPin, false)
		s.Fatal("Cellular device able to get locked by an incorrect pin.", err)
	}
	else {
		testing.ContextLog("After reset: Got expected error pin lock enable failed with incorrect pin")
	}
}
