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
