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

// This test run on all carriers including esim. Slot switch tested on boards with Modem,
// SimSlots property and have at least two SIM slots and one active profile on each slot.

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
	s.Log("Precheck Modem SIM properties")
	primary, modem, err := checkModemSimProperties(ctx)
	// Do graceful exit if dut has only one sim slot or no slot
	if err == errorSingleSlot {
		s.Log("Can not run this test as: ", err)
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
	if modem.Call(ctx, "SetPrimarySimSlot", newslot); err != nil {
		s.Fatal("Failed at primary SIM Slot switch: ", err)
	}
	defer modem.Call(ctx, "SetPrimarySimSlot", primary)

	if err = testing.Poll(ctx, func(ctx context.Context) error {
		modem1, err := modemmanager.NewModem(ctx)
		if err != nil {
			return err
		}
		if modem.String() == modem1.String() {
			return errors.New("Old modem still exists")
		}
		s.Logf("Modem paths are Old: %s, New: %s ", modem, modem1)
		return nil
	}, &testing.PollOptions{Timeout: 60 * time.Second}); err != nil {
		s.Fatal("Modem or its properties not up after switching SIM slot")
	}
	s.Log("Postcheck Modem SIM properties")
	primary, modem, err = checkModemSimProperties(ctx)
	if err != nil || primary != newslot {
		s.Fatal("Failed to get primary slot properties after switching primary slot: ", err)
	}
}

// errorSingleSlot thrown to do greaceful Exit from this test if DUT got single SIM slot error
var errorSingleSlot = errors.New("Single/No SIM slot on DUT, Need two slots to run this test")

// checkModemSimProperties checks modem properties and returns primary SIM slot,
// associated new modem object, and error for DBus failures & prerequisite failures
func checkModemSimProperties(ctx context.Context) (uint32, *modemmanager.Modem, error) {
	modem, err := modemmanager.NewModem(ctx)
	if err != nil {
		return 0, nil, errors.Wrap(err, "failed to create modem")
	}
	props, err := modem.GetDBusProperties(ctx)
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
