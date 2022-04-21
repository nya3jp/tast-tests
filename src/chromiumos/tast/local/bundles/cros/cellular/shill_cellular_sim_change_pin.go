// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCellularSimChangePin,
		Desc:     "Verifies that the cellular device SIM PIN can be changed",
		Contacts: []string{"srikanthkumar@google.com", "chromeos-cellular-team@google.com"},
		Attr:     []string{"group:cellular", "cellular_unstable", "cellular_sim_dual_active"},
		Fixture:  "cellular",
		Timeout:  4 * time.Minute,
	})
}

// ShillCellularSimChangePin tests successfully changing SIM pin.
func ShillCellularSimChangePin(ctx context.Context, s *testing.State) {
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
		s.Log("Skipped on this dut as could not find mapping PUK code for ICCID on dut")
		return
	}

	// ResetModem needed for sim power reset to reflect locked type values.
	if _, err := helper.ResetModem(ctx); err != nil {
		s.Log("Failed to reset modem: ", err)
	}

	// Shorten deadline to leave time for cleanup
	cleanupCtx := ctx
	cleanupCtx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	// Test sequence :
	// Enable Default Pin => Change Pin to Temp Pin => Check TempPin lockstates =>
	// Revert to Default Pin => Disable Default Pin.

	// Enable pin lock.
	s.Log("Calling requirepin to enable pin lock")
	if err = helper.Device.RequirePin(ctx, currentPin, true); err != nil {
		s.Fatal("Failed to enable lock: ", err)
	}

	defer func(ctx context.Context) {
		// Unlock and disable pin lock.
		if err = helper.Device.RequirePin(ctx, currentPin, false); err != nil {
			s.Fatal("Failed to disable default pin lock: ", err)
		}
	}(cleanupCtx)

	s.Log("Attempting to change enabled sim lock with temporary pin: ", tempPin)
	if err = helper.Device.ChangePin(ctx, currentPin, tempPin); err != nil {
		s.Fatal("Failed to change pin: ", err)
	}

	enabled := false
	locked := false

	defer func(ctx context.Context) {
		if enabled || locked {
			// Reverse to default pin.
			s.Log("Reverse pin to default pin from temppin")
			if err = helper.Device.ChangePin(ctx, tempPin, currentPin); err != nil {
				s.Fatal("Failed to change pin lock to default pin lock: ", err)
			}
		}
	}(cleanupCtx)

	// ResetModem needed for sim power reset to reflect locked type values.
	if _, err = helper.ResetModem(ctx); err != nil {
		s.Log("Failed to reset modem: ", err)
	}

	enabled = helper.IsSimLockEnabled(ctx)
	if !enabled {
		s.Fatal("SIM lock not enabled by temp pin: ", err)
	}
	locked = helper.IsSimPinLocked(ctx)
	if !locked {
		s.Fatal("SIM lock was not locked by temp pin: ", err)
	}
}
