// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
)

// This test runs on all carriers including esim. Slot switch is tested on boards with Modem,
// SimSlots property and at least two SIM slots and one active profile on each slot.

func init() {
	testing.AddTest(&testing.Test{
		Func:     ModemmanagerSlotSwitch,
		Desc:     "Verifies that modemmanager switches SIM slot",
		Contacts: []string{"srikanthkumar@google.com", "cros-network-health@google.com"},
		Attr:     []string{"group:cellular_multisim", "cellular_multisim_unstable"},
		Fixture:  "cellular",
		Timeout:  5 * time.Minute,
	})
}

// ModemmanagerSlotSwitch Test
func ModemmanagerSlotSwitch(ctx context.Context, s *testing.State) {
	primary, _, err := checkModemSimProperties(ctx)

	// Do graceful exit if dut has only one sim slot or no slot
	if err == errorSingleSlot {
		s.Log("Can not run this test: ", err)
		return
	} else if err != nil {
		s.Fatal("Failed at checkModemSimProperties: ", err)
	}

	// Switch slot
	var newslot uint32 = 2
	if primary == newslot {
		newslot = 1
	}
	s.Logf("Switching slot from: %d to %d", primary, newslot)
	if err = setPrimarySimSlot(ctx, newslot); err != nil {
		s.Fatal("Switch failed in 1st run: ", err)
	}
	slot, _, err := checkModemSimProperties(ctx)
	if err != nil {
		s.Fatal("Failed to get primary slot properties in 1st run: ", err)
	}
	if slot != newslot {
		s.Fatal("Failed to switch SIM slot")
	}

	// Call setPrimarySimSlot again to set slot to original primary slot
	s.Logf("Switching slot from: %d to %d", newslot, primary)
	if err = setPrimarySimSlot(ctx, primary); err != nil {
		s.Fatal("Switch failed in 2nd run: ", err)
	}
	slot, _, err = checkModemSimProperties(ctx)
	if err != nil {
		s.Fatal("Failed to get primary slot properties in 2nd run: ", err)
	}
	if slot != primary {
		s.Fatal("Failed to switch SIM slot")
	}
}

// errorSingleSlot thrown to do greaceful Exit from this test if DUT got single SIM slot error
var errorSingleSlot = errors.New("Test requires two slots, exiting")

// setPrimarySimSlot switches primary SIM slot to a given slot
func setPrimarySimSlot(ctx context.Context, primary uint32) error {
	modem, err := modemmanager.NewModem(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create modem")
	}
	if c := modem.Call(ctx, "SetPrimarySimSlot", primary); c.Err != nil {
		return errors.Wrapf(c.Err, "failed while switching the primary SIM slot to %d", primary)
	}
	if _, err = modemmanager.PollModem(ctx, modem.String()); err != nil {
		return errors.Wrapf(err, "could not find modem after switching the primary slot to %d", primary)
	}
	return nil
}

// checkModemSimProperties checks modem properties and returns primary SIM slot,
// associated new modem object, and error for DBus failures & prerequisite failures
func checkModemSimProperties(ctx context.Context) (uint32, *modemmanager.Modem, error) {
	modem, err := modemmanager.NewModem(ctx)
	if err != nil {
		return 0, nil, errors.Wrap(err, "failed to create modem")
	}
	props, err := modem.GetProperties(ctx)
	if err != nil {
		return 0, modem, errors.Wrap(err, "failed to call GetProperties on modem")
	}
	sim, err := props.GetObjectPath(mmconst.ModemPropertySim)
	if err != nil {
		return 0, modem, errors.Wrap(err, "missing sim property")
	}
	simSlots, err := props.GetObjectPaths(mmconst.ModemPropertySimSlots)
	if err != nil {
		return 0, modem, errors.Wrap(err, "failed to get simslots property")
	}
	numSlots := len(simSlots)
	testing.ContextLogf(ctx, "Number of slots on dut: %d", numSlots)
	if numSlots < 2 {
		return 0, modem, errorSingleSlot
	}
	primary, err := props.GetUint32(mmconst.ModemPropertyPrimarySimSlot)
	if err != nil {
		return 0, modem, errors.Wrap(err, "missing PrimarySimSlot property")
	}
	if int(primary) > len(simSlots) {
		return primary, modem, errors.Errorf("invalid PrimarySimSlot: %d", primary)
	}
	if sim != simSlots[primary-1] {
		return primary, modem, errors.Errorf("Sim property mismatch: %s", sim)
	}
	testing.ContextLogf(ctx, "Modem Sim property: %s, PrimarySimSlot: %d", sim, primary)
	return primary, modem, nil
}
