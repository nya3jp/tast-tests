// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/cellular"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
)

// This test is only run on the cellular_esim group. All boards in that group
// provide the Modem.SimSlots property and have at least two SIM slots.

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCellularSimSlots,
		Desc:     "Verifies that Shill receives SimSlot information from ModemManager and can set the primary sim slot",
		Contacts: []string{"stevenjb@google.com", "cros-network-health@google.com"},
		Attr:     []string{"group:cellular", "cellular_sim_dual_active"},
	})
}

func ShillCellularSimSlots(ctx context.Context, s *testing.State) {
	// Ensure that MM reports 2 slots.
	modem, err := modemmanager.NewModem(ctx)
	if err != nil {
		s.Fatal("Failed to create Modem: ", err)
	}
	simProperties, _, err := modem.GetSimSlots(ctx)
	if err != nil {
		s.Fatal("Failed to get SimSlots: ", err)
	}
	numSlots := len(simProperties)
	if numSlots < 2 {
		s.Fatalf("Expected at least 2 SIM slots, found: %d", numSlots)
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper")
	}

	// Ensure that a Cellular Service is created for each SIM.
	for i := 0; i < 2; i++ {
		simProps := simProperties[i]
		if simProps == nil {
			s.Errorf("No SIM properties in slot: %d", i)
			continue
		}
		iccid, err := simProps.GetString(mmconst.SimPropertySimIdentifier)
		if err != nil {
			s.Fatal("Missing Sim.SimIdentifier property: ", err)
		}
		props := map[string]interface{}{
			shillconst.ServicePropertyCellularICCID: iccid,
			shillconst.ServicePropertyType:          shillconst.TypeCellular,
		}
		_, err = helper.Manager.WaitForServiceProperties(ctx, props, 30*time.Second)
		if err != nil {
			s.Errorf("Cellular Service not found for ICCID: %s: %s", iccid, err)
		}
	}
	if s.HasError() {
		s.Log("Error(s) finding Cellular Services")
		return
	}

	// Gather Shill Device properties
	deviceProps, err := helper.Device.GetShillProperties(ctx)
	if err != nil {
		s.Fatal("Failed to get Device properties: ", err)
	}

	// Verify Device.SimSlots.
	info, err := deviceProps.Get(shillconst.DevicePropertyCellularSIMSlotInfo)
	if err != nil {
		s.Fatal("Failed to get Device.CellularSIMSlotInfo property: ", err)
	}
	simSlotInfo, ok := info.([]map[string]dbus.Variant)
	if !ok {
		s.Fatal("Invalid format for Device.CellularSIMSlotInfo")
	}
	if len(simSlotInfo) != numSlots {
		s.Fatalf("Incorrect Device.CellularSIMSlotInfo size, got %d, want %d", len(simSlotInfo), numSlots)
	}
	for i := 0; i < 2; i++ {
		simProps := simProperties[i]
		slotInfo := simSlotInfo[i]
		if err := compareSimProps(simProps, "SimIdentifier", slotInfo, "ICCID"); err != nil {
			s.Fatal("ICCID mismatch, err: ", err)
		}
		if err := compareSimProps(simProps, "Eid", slotInfo, "EID"); err != nil {
			s.Fatal("EID mismatch, err: ", err)
		}
	}
}

func compareSimProps(simProps *dbusutil.Properties, simKey string, slotInfo map[string]dbus.Variant, slotKey string) error {
	simp, err := simProps.GetString(simKey)
	if err != nil {
		return errors.Wrapf(err, "missing property: %v", simKey)
	}
	slotp := slotInfo[slotKey].Value()
	if simp != slotp {
		return errors.Errorf("property mismatch, got: %s, want: %s", slotp, simp)
	}
	return nil
}
