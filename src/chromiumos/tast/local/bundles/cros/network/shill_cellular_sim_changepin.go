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
		Func:     ShillCellularSimChangePin,
		Desc:     "Verifies that Cellular Device SIM PIN lock change",
		Contacts: []string{"srikanthkumar@google.com", "chromeos-cellular-team@google.com"},
		Attr:     []string{"group:cellular"},
		Fixture:  "cellular",
	})
}

// ShillCellularSimChangePin tests successfully changing SIM lock pin.
func ShillCellularSimChangePin(ctx context.Context, s *testing.State) {
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
	testing.ContextLog("Attempting to enable SIM lock with correct pin.")
	// Check if not enabled PIN and set PIN
	if helper.Device.IsSimLockEnabled(ctx) && helper.Device.IsSimPinLocked(ctx) {
		// Disable and remove default PIN
		err = helper.Device.RequirePin(currentPin, false)
		if err != nil {
			s.Fatal("Failed to disable lock", err)
		}
	}
	err = helper.Device.RequirePin("1234", true)
	if err != nil {
		s.Fatal("Failed to enable PIN", err)
	}
	enabled, err = helper.Device.IsSimLockEnabled(ctx)
	if !enabled || err != nil {
		s.Fatal("SIM lock was not enabled by correct pin.", err)
	}
	err = helper.Device.ChangePin(ctx, "1234", currentPin)
	if err != nil {
		s.Fatal("Failed to change pin", err)
	}
	// Reset modem or Reset dut with sim lock
	// SIM lock should be enabled, and lock set after reset.
	// TODO: Reset using building blocks
	// cbb.ReinitDUT()
	enabled, err = helper.Device.IsSimLockEnabled(ctx)
	if !enabled || err != nil {
		s.Fatal("SIM lock not enabled by correct pin after reset.", err)
	}
	locked, err = helper.Device.IsSimPinLocked(ctx)
	if !locked || err != nil {
		s.Fatal("SIM lock was not eanbled by correct pin.", err)
	}
	if enabled || locked {
		// unlock and Disable pin lock
		err = helper.Device.RequirePin(ctx, currentPin, false)
		if err != nil {
			s.Fatal("Failed to disable pin lock to set dut normal", err)
		}
	}
}
