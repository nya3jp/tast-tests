// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCellularSimFailedEnablePinLock,
		Desc:     "Verifies that cellular device SIM lock can't be enabled with incorrect PIN",
		Contacts: []string{"srikanthkumar@google.com", "chromeos-cellular-team@google.com"},
		Attr:     []string{"group:cellular", "cellular_unstable", "cellular_sim_pinlock"},
		Fixture:  "cellular",
		Timeout:  5 * time.Minute,
	})
}

// ShillCellularSimFailedEnablePinLock checks sim lock can not be enabled with incorrect PIN.
func ShillCellularSimFailedEnablePinLock(ctx context.Context, s *testing.State) {
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

	s.Log("Attempting to enable SIM lock with incorrect pin")
	// Check if pin is not enabled and try to set incorrect pin.
	if helper.IsSimLockEnabled(ctx) && helper.IsSimPinLocked(ctx) {
		// Disable and remove pin.
		if err = helper.Device.RequirePin(ctx, currentPin, false); err != nil {
			s.Fatal("Failed to disable lock: ", err)
		}
	}

	badPin, err := helper.BadPin(ctx, currentPin)
	if err != nil {
		s.Fatal("Failed to generate random pin based on current pin")
	}
	// Shorten deadline to leave time for cleanup
	cleanupCtx := ctx
	cleanupCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	err = helper.Device.RequirePin(ctx, badPin, true)
	if err == nil {
		defer func(ctx context.Context) {
			// Unlock and disable bad pin lock.
			if err = helper.Device.RequirePin(ctx, badPin, false); err != nil {
				s.Fatal("Failed to disable bad pin lock: ", err)
			}
		}(cleanupCtx)
		s.Fatal("Failed as able to enable pin with bad pin")
	}
	s.Log("Bad pin used to lock device: ", badPin)

	if strings.Contains(err.Error(), shillconst.ErrorIncorrectPin) ||
		strings.Contains(err.Error(), shillconst.ErrorPinFailure) {
		s.Log("Got expected pin lock error for incorrect pin")
	} else {
		// Unlock dut and raise error.
		helper.Device.RequirePin(ctx, badPin, false)
		s.Fatal("Failed to get expected error with incorrect pin: ", err)
	}

	enabled := helper.IsSimLockEnabled(ctx)
	if enabled {
		s.Log("SIM lock got enabled by incorrect pin: ", badPin)
	}

	// ResetModem needed for sim power reset to reflect locked type values.
	if _, err := helper.ResetModem(ctx); err != nil {
		s.Log("Failed to reset modem: ", err)
	}

	// Reverse pin lock status with badpin if still locked.
	locked := helper.IsSimPinLocked(ctx)
	pukLocked := helper.IsSimPukLocked(ctx)
	if enabled || locked || pukLocked {
		// Disable pin lock and unlock.
		err = helper.Device.RequirePin(ctx, badPin, false)
		s.Fatal("Cellular device able to get locked by an incorrect pin: ", err)
	}
}
