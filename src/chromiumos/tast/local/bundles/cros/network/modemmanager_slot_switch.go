// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
)

// This test is only run on the cellular_esim group. All boards in that group
// provide the Modem.SimSlots property and have at least two SIM slots.
func init() {
	testing.AddTest(&testing.Test{
		Func:     ModemmanagerSlotSwitch,
		Desc:     "Verifies that modemmanager switches SIM slot",
		Contacts: []string{"srikanthkumar@google.com", "cros-network-health@google.com"},
		Attr:     []string{"group:cellular"},
		Fixture:  "cellular",
	})
}

// ModemmanagerSlotSwitch Test
func ModemmanagerSlotSwitch(ctx context.Context, s *testing.State) {
        s.Log("Precheck shill SIM services")
        primary, modem, err := checkShillSimServices(ctx)
	// Switch slot
	var newslot uint32 = 2
	if primary == newslot {
	        newslot = 1
	}
	s.Log("Switching slot from:", primary, " to:",  newslot)
	c := modem.Call(ctx, "SetPrimarySimSlot", newslot)
	if c.Err != nil {
		s.Fatal(c.Err, "Failed to switch Primary SIM Slot")
	}
	// Wait for modem, as slot switch causes modem and MM reload
	testing.Sleep(ctx, 30*time.Second)
        s.Log("Postcheck shill SIM services")
        primary, modem, err = checkShillSimServices(ctx)
        if err != nil || primary <= 0 {
		s.Fatal("Failed to get primary slot properties after switching primary slot", err)
	}
}

// checks cellular service created for each SIM slot
func checkShillSimServices(ctx context.Context) (uint32, *modemmanager.Modem, error) {
	modem, err := modemmanager.NewModem(ctx)
	if err != nil {
		return 0, nil, errors.Wrap(err, "Failed to create Modem")
	}
	props, err := modem.GetDBusProperties(ctx)
	if err != nil {
		return 0, modem, errors.Wrap(err, "Failed to call GetProperties on Modem")
	}
	sim, err := props.GetObjectPath(mmconst.ModemPropertySim)
	if err != nil {
		return 0, modem, errors.Wrap(err, "Missing Sim property")
	}
	simSlots, err := props.GetObjectPaths(mmconst.ModemPropertySimSlots)
	if err != nil {
		return 0, modem, errors.Wrap(err, "Failed to get SimSlots property")
	}
	numSlots := len(simSlots)
	if numSlots < 2 {
		return 0, modem, errors.Errorf("Expected at least 2 SIM slots, found: %d", numSlots)
	}
	primary, err := props.GetUint32(mmconst.ModemPropertyPrimarySimSlot)
	if err != nil {
		return 0, modem, errors.Wrap(err, "Missing PrimarySimSlot property")
	}
	if int(primary) > len(simSlots) {
		return primary, modem, errors.Errorf("Invalid PrimarySimSlot: %d", primary)
	}
	if sim != simSlots[primary-1] {
		return primary, modem, errors.Errorf("Sim property mismatch: %s", sim)
	}
	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		return primary, modem, errors.Wrap(err, "Failed to create cellular.Helper")
	}

	// Ensure that a Cellular Service is created for each Modem.SimSlots entry.
	for i := 0; i < 2; i++ {
		simPath := simSlots[i]
		simProps, err := modem.GetSimProperties(ctx, simPath)
		if err != nil {
			return primary, modem, errors.Wrap(err, "Failed to call Sim.GetProperties")
		}
		iccid, err := simProps.GetString("SimIdentifier")
		if err != nil {
			return primary, modem, errors.Wrap(err, "Missing Sim.SimIdentifier property")
		}
		props := map[string]interface{}{
			shillconst.ServicePropertyCellularICCID: iccid,
			shillconst.ServicePropertyType:          shillconst.TypeCellular,
		}
		_, err = helper.Manager.WaitForServiceProperties(ctx, props, 30*time.Second)
		if err != nil {
			return primary, modem, errors.Errorf("Cellular Service not found for ICCID: %s: %s", iccid, err)
		}
	}
        return primary, modem, nil
}
