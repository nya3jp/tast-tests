// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
		Func:     ShillCellularSimPukLock,
		Desc:     "Verifies that cellular device SIM PUK lock",
		Contacts: []string{"srikanthkumar@google.com", "chromeos-cellular-team@google.com"},
		Attr:     []string{"group:cellular", "cellular_unstable", "cellular_sim_pinlock"},
		Fixture:  "cellular",
		Timeout:  5 * time.Minute,
		Vars:     []string{"autotest_host_info_labels"},
	})
}

// ShillCellularSimPukLock tests successfully enabling sim lock and locking the sim with puk-lock.
func ShillCellularSimPukLock(ctx context.Context, s *testing.State) {
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
		// Do graceful exit, not to run tests on unknown puk duts.
		s.Logf("Unable to find PUK code for ICCID : %s, skipping the test", iccid)
		return
	}

	s.Log("Attempting to enable sim pin lock and set in puk lock state")
	if err = helper.PukLockSim(ctx, currentPin); err != nil {
		// Unlock and disable pin lock if failed after locking pin.
		if errNew := helper.ClearSIMLock(ctx, currentPin, currentPuk); errNew != nil {
			s.Log("Failed to clear default pin lock in puklocksim: ", errNew)
		}
		s.Fatal("Failed to enable puk lock: ", err)
	}

	// Shorten deadline to leave time for cleanup
	cleanupCtx := ctx
	cleanupCtx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	defer func(ctx context.Context) {
		// Unlock and disable pin lock.
		if err = helper.ClearSIMLock(ctx, currentPin, currentPuk); err != nil {
			s.Fatal("Failed to clear default pin lock: ", err)
		}
	}(cleanupCtx)

	if retriesLeft, err := helper.GetRetriesLeft(ctx); err != nil {
		s.Fatal("Could not get pin retries left: ", err)
	} else if retriesLeft <= 1 {
		s.Fatal("No retries left to try error state after puk locked")
	}

	// Get default puk, pin from initialize and unlock using puk.
	if err = helper.Device.UnblockPin(ctx, currentPuk, currentPin); err != nil {
		s.Fatal("Could not unlock puk: ", err)
	}

	locked := helper.IsSimPukLocked(ctx)
	if locked {
		s.Fatal("Failed to do puk unlock-manual repair needed on dut: ", err)
	}
	pinLocked := helper.IsSimPinLocked(ctx)
	if pinLocked {
		s.Log("Pin-lock got unlocked while unlocking the puk-lock: ", err)
	}
	enabled := helper.IsSimLockEnabled(ctx)
	if !enabled {
		s.Fatal("SIM lock got disabled when attemping to unlock a pin-locked sim: ", err)
	}
}
