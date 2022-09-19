// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCellularSimChangePin,
		Desc:     "Verifies that the cellular device SIM PIN can be changed",
		Contacts: []string{"srikanthkumar@google.com", "chromeos-cellular-team@google.com"},
		Attr:     []string{"group:cellular", "cellular_unstable", "cellular_sim_pinlock"},
		Fixture:  "cellular",
		Timeout:  5 * time.Minute,
		Vars:     []string{"autotest_host_info_labels"},
	})
}

// ShillCellularSimChangePin tests successfully changing SIM pin.
func ShillCellularSimChangePin(ctx context.Context, s *testing.State) {
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

	tempPin := mmconst.TempSimPin

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
