// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cellular"
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
		Vars:     []string{"autotest_host_info_labels"},
	})
}

// ShillCellularSimFailedEnablePinLock checks sim lock can not be enabled with incorrect PIN.
func ShillCellularSimFailedEnablePinLock(ctx context.Context, s *testing.State) {
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

	// Check if SIM is locked
	if helper.IsSimLockEnabled(ctx) || helper.IsSimPukLocked(ctx) {
		// Unlock and disable pin lock.
		if err = helper.ClearSIMLock(ctx, currentPin, currentPuk); err != nil {
			s.Fatal("Failed to clear PIN/PUK lock: ", err)
		}
	}

	s.Log("Attempting to enable SIM lock with incorrect pin")
	badPin, err := helper.BadPin(ctx, currentPin)
	if err != nil {
		s.Fatal("Failed to generate random pin based on current pin")
	}
	// Shorten deadline to leave time for cleanup
	cleanupCtx := ctx
	cleanupCtx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
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

	defer func(ctx context.Context) {
		// Unlock and disable pin lock.
		if err = helper.ClearSIMLock(ctx, currentPin, currentPuk); err != nil {
			s.Fatal("Failed to clear PIN/PUK lock: ", err)
		}
	}(cleanupCtx)

	if strings.Contains(err.Error(), shillconst.ErrorPinFailure) {
		s.Log("Got expected pin lock error for incorrect pin: ", err)
	} else {
		// Unlock dut and raise error.
		helper.Device.RequirePin(ctx, badPin, false)
		s.Fatal("Failed to get expected error with incorrect pin: ", err)
	}

	// ResetModem needed on Fibocom modems to read SIM lock status
	if _, err := helper.ResetModem(ctx); err != nil {
		s.Log("Failed to reset modem: ", err)
	}

	enabled := helper.IsSimPukLocked(ctx)
	if enabled {
		s.Log("SIM got PUK locked by incorrect pin: ", badPin)
	}
}
