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
		Func:     ShillCellularSimPinLock,
		Desc:     "Verifies that cellular device SIM PIN lock",
		Contacts: []string{"srikanthkumar@google.com", "chromeos-cellular-team@google.com"},
		Attr:     []string{"group:cellular", "cellular_unstable", "cellular_sim_pinlock"},
		Fixture:  "cellular",
		Timeout:  5 * time.Minute,
	})
}

// ShillCellularSimPinLock tests enabling sim lock and locking the sim with pin-lock.
func ShillCellularSimPinLock(ctx context.Context, s *testing.State) {
	// Gather Shill Device sim properties.
	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	modem, err := modemmanager.NewModemWithSim(ctx)
	if err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}

	currentPin := mmconst.DefaultSimPin
	if currentPuk, err := modem.GetActiveSimPuk(ctx); err != nil {
		s.Fatal("Failed to get active sim puk: ", err)
	} else if currentPuk == "" {
		// Do graceful exit, not to run tests on unknown puk duts.
		s.Log("Skipped on this dut as could not find mapping PUK code for ICCID on dut")
		return
	}

	// Check if pin enabled and locked/set.
	if helper.IsSimLockEnabled(ctx) || helper.IsSimPinLocked(ctx) {
		// Disable pin.
		if err = helper.Device.RequirePin(ctx, currentPin, false); err != nil {
			s.Fatal("Failed to disable lock: ", err)
		}
	}

	// Shorten deadline to leave time for cleanup
	cleanupCtx := ctx
	cleanupCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	s.Log("Attempting to enable sim lock with correct pin")
	if err = helper.Device.RequirePin(ctx, currentPin, true); err != nil {
		s.Fatal("Failed to enable pin, mostly default pin needs to set on dut: ", err)
	}

	defer func(ctx context.Context) {
		// Unlock and disable pin lock.
		if err = helper.Device.RequirePin(ctx, currentPin, false); err != nil {
			s.Fatal("Failed to disable default pin lock: ", err)
		}
	}(cleanupCtx)

	enabled := helper.IsSimLockEnabled(ctx)
	if !enabled {
		s.Fatal("SIM lock not enabled by correct pin: ", err)
	}

	// ResetModem needed for sim power reset to reflect locked type values.
	if _, err := helper.ResetModem(ctx); err != nil {
		s.Log("Failed to reset modem: ", err)
	}

	locked := helper.IsSimPinLocked(ctx)
	if !locked {
		s.Log("SIM lock was not locked by pin: ", mmconst.DefaultSimPin)
	}
	if enabled || locked {
		if err := helper.Device.EnterPin(ctx, currentPin); err != nil {
			s.Log("Failed to enterpin")
		}
	}
}
