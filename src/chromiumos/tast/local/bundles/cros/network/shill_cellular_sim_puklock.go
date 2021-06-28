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
		Func:     ShillCellularSimPUKLock,
		Desc:     "Verifies that Cellular Device SIM PUK lock",
		Contacts: []string{"srikanthkumar@google.com", "chromeos-cellular-team@google.com"},
		Attr:     []string{"group:cellular"},
		Fixture:  "cellular",
	})
}

// ShillCellularSimPUKLock tests successfully enabling SIM lock and locking the SIM with puk-lock.
func ShillCellularSimPUKLock(ctx context.Context, s *testing.State) {
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
	testing.ContextLog("Attempting to enable SIM lock and set in PUK lock state.")
	err = helper.Device.PukLockSim(ctx, "123456")
	if err != nil {
		s.Fatal("Failed to enable PUK lock", err)
	}

	// Reset modem or Reset dut with sim lock
	// SIM lock should be enabled, and lock set after reset.
	// TODO: Reset using building blocks
	// cbb.ReinitDUT()
	retriesLeft, err = helper.Device.GetRetriesLeft(ctx)
	if err != nil {
		s.Fatal("Could not get pin retries left", err)
	}
	// get default puk, pin from initialize
	err = helper.Device.UnblockPUK(ctx, currentPuk, currentPin)
	if err != nil {
		s.Fatal("Could not unlock puk", err)
	}
	locked, err = helper.Device.IsSimPukLocked(ctx)
	if locked || err != nil {
		s.Fatal("Failed to unlock a puk-locked SIM with correct puk.", err)
	}
	locked, err = helper.Device.IsSimPinLocked(ctx)
	if locked || err != nil {
		s.Fatal("pin-lock got unlocked while unlocking the puk-lock.", err)
	}
	enabled, err = helper.Device.IsSimLockEnabled(ctx)
	if !enabled || err != nil {
		s.Fatal("SIM lock got disabled when attemping to unlock a pin-locked SIM.", err)
	}

	// Reverse to default state
	enabled, err = helper.Device.IsSimLockEnabled(ctx)
	if err != nil {
		s.Fatal("SIM lock not enabled by correct pin after reset.", err)
	}
	locked, err = helper.Device.IsSimPinLocked(ctx)
	if err != nil {
		s.Fatal("SIM lock was not enabled by correct pin.", err)
	}
	if enabled || locked {
		// unlock and Disable pin lock
		err = helper.Device.RequirePin(ctx, currentPin, false)
		if err != nil {
			s.Fatal("Failed to disable pin lock to set dut normal", err)
		}
	}
}
