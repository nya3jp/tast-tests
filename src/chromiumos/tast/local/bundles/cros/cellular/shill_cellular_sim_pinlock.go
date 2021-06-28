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
		Func:     ShillCellularSimPinLock,
		Desc:     "Verifies that Cellular Device SIM PIN lock",
		Contacts: []string{"srikanthkumar@google.com", "chromeos-cellular-team@google.com"},
		Attr:     []string{"group:cellular"},
		Fixture:  "cellular",
	})
}

// Test successfully enabling SIM lock and locking the SIM with pin-lock.
func ShillCellularSimPinLock(ctx context.Context, s *testing.State) {
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
	currentPin := 1234 //Tmp
	testing.ContextLog("Attempting to enable SIM lock with correct pin.")
	// Check if not enabled PIN and set PIN
	if helper.Device.IsSimLockEnabled(ctx) && helper.Device.IsSimPinLocked(ctx) {
		// Disable and remove PIN
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
		err = helper.Device.RequirePin(ctx, "1234", false)
		if err != nil {
			s.Fatal("Failed to disable pin lock to set dut normal", err)
		}
	}
}
