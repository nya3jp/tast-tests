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
		Attr:     []string{"group:cellular", "cellular_unstable"},
		Fixture:  "cellular",
		Timeout:  5 * time.Minute,
	})
}

// ModemmanagerSlotSwitch Test
func ModemmanagerSlotSwitch(ctx context.Context, s *testing.State) {
	primary, modem, err := checkModemSimProperties(ctx)

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
	if err = pollModem(ctx, modem.String()); err != nil {
		s.Fatal("Poll failed 1st run: ", err)
	}
	slot, modem, err := checkModemSimProperties(ctx)
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
	if err = pollModem(ctx, modem.String()); err != nil {
		s.Fatal("Poll failed in 2nd run: ", err)
	}
	slot, modem, err = checkModemSimProperties(ctx)
	if err != nil {
		s.Fatal("Failed to get primary slot properties in 2nd run: ", err)
	}
	if slot != primary {
		s.Fatal("Failed to switch SIM slot")
	}
}

// errorSingleSlot thrown to do greaceful Exit from this test if DUT got single SIM slot error
var errorSingleSlot = errors.New("Test requires two slots, exiting")

// setPrimarySimSlot switches primary sim slot to given slot
func setPrimarySimSlot(ctx context.Context, primary uint32) error {
	modem, err := modemmanager.NewModem(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Modem")
	}
	if modem.Call(ctx, "SetPrimarySimSlot", primary); err != nil {
		return errors.Wrap(err, "failed at primary SIM Slot switch")
	}
	return nil
}

// pollModem waits for modem to load after every slot switch operation
func pollModem(ctx context.Context, oldModem string) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		newModem, err := modemmanager.NewModem(ctx)
		if err != nil {
			return err
		}
		if oldModem == newModem.String() {
			return errors.New("Old modem still exists")
		}
		testing.ContextLogf(ctx, "Modem paths are Old: %s, New: %s ", oldModem, newModem)
		return nil
	}, &testing.PollOptions{Timeout: mmconst.ModemPollTime}); err != nil {
		return errors.Wrap(err, "modem or its properties not up after switching SIM slot")
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
