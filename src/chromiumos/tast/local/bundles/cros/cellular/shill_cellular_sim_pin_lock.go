// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cellular"
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
		Vars:     []string{"autotest_host_info_labels"},
	})
}

// ShillCellularSimPinLock tests enabling sim lock and locking the sim with pin-lock.
func ShillCellularSimPinLock(ctx context.Context, s *testing.State) {
	// Gather Shill Device sim properties.
	labels, err := cellular.GetLabelsAsStringArray(ctx, s.Var, "autotest_host_info_labels")
	if err != nil {
		s.Fatal("Failed to read autotest_host_info_labels: ", err)
	}

	helper, err := cellular.NewHelperWithLabels(ctx, labels)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	iccid, err := helper.GetCurrentICCID(ctx)
	if err != nil {
		s.Fatal("Could not get current ICCID: ", err)
	}
	currentPin, currentPuk, err := helper.GetPINAndPUKForICCID(ctx, iccid)
	if err != nil {
		s.Fatal("Could not get Pin and Puk : ", err)
	}
	if currentPuk == "" {
		s.Fatal("Unable to find PUK code for ICCID : ", iccid)
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
		s.Log("SIM lock was not locked by pin: ", currentPin)
	}
	if enabled || locked {
		if err := helper.Device.EnterPin(ctx, currentPin); err != nil {
			s.Log("Failed to enterpin")
		}
	}
}
