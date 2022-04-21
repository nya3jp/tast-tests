// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ModemmanagerSARInterfaceVerification,
		Desc:     "Verifies that modemmanager SAR interface enable, disable succeeds",
		Contacts: []string{"madhavadas@google.com", "chromeos-cellular-team@google.com"},
		Attr:     []string{"group:cellular", "cellular_sim_active"},
		Fixture:  "cellular",
		Timeout:  5 * time.Minute,
	})
}

// ModemmanagerSARInterfaceVerification Test
func ModemmanagerSARInterfaceVerification(ctx context.Context, s *testing.State) {

	// Check if device uses modemmanager/sw sar
	mmSAREnabled, err := crosconfig.Get(ctx, "/power", "use-modemmanager-for-dynamic-sar")
	if err != nil && !crosconfig.IsNotFound(err) {
		s.Fatal("cros_config /power use-modemmanager-for-dynamic-sar failed: ", err)
	}

	s.Log("mmSAREnabled :", mmSAREnabled)

	if mmSAREnabled != "1" {
		return
	}

	modem, err := modemmanager.NewModemWithSim(ctx)
	if err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}

	if err := modem.Call(ctx, mmconst.ModemEnable, true).Err; err != nil {
		s.Fatal("Modem enable failed with: ", err)
	}

	if err := modemmanager.EnsureEnabled(ctx, modem); err != nil {
		s.Fatal("Modem not enabled: ", err)
	}

	sar, err := modem.GetSARInterface(ctx)
	if err != nil {
		s.Fatal("Failed to read SAR interface: ", err)
	}

	// Check Enable SAR
	if err := updateAndCheckSARState(ctx, sar, true); err != nil {
		s.Fatal("Failed to enable SAR: ", err)
	} else {
		s.Log("Enabled SAR")
	}

	// Check Disable SAR
	if err := updateAndCheckSARState(ctx, sar, false); err != nil {
		s.Fatal("Failed to disable SAR: ", err)
	} else {
		s.Log("Disabled SAR")
	}

	// Re-enable SAR
	if err := updateAndCheckSARState(ctx, sar, true); err != nil {
		s.Fatal("Failed to re-enable SAR: ", err)
	} else {
		s.Log("Re-enabled SAR")
	}

}

func updateAndCheckSARState(ctx context.Context, modem *modemmanager.Modem, state bool) error {
	if err := modem.EnableSAR(ctx, state); err != nil {
		return errors.Wrap(err, "failed to call EnableSAR()")
	}

	enabled, err := modem.IsSAREnabled(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to read SARState")
	}
	testing.ContextLog(ctx, "SARState :", enabled)

	if enabled != state {
		return errors.New("failed to set SAR State")
	}

	return nil
}
