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

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCellularSimSlots,
		Desc:     "Verifies that shill receives sim slot information and can set the primary sim slot",
		Contacts: []string{"stevenjb@google.com", "cros-network-health@google.com"},
		Attr:     []string{"group:cellular"},
	})
}

func ShillCellularSimSlots(ctx context.Context, s *testing.State) {
	// Ensure that MM reports 2 slots.
	modem, err := modemmanager.NewModem(ctx)
	if err != nil {
		s.Fatal("Failed to create Modem: ", err)
	}
	modemProps, err := modem.GetDBusProperties(ctx)
	if err != nil {
		s.Fatal("Failed to call Modem.GetProperties: ", err)
	}
	simSlots, err := modemProps.GetObjectPaths(mmconst.ModemPropertySimSlots)
	if err != nil {
		s.Fatal("Missing Modem.SimSlots property: ", err)
	}
	numSlots := len(simSlots)
	if numSlots != 2 {
		s.Fatalf("Expected 2 SIM slots, found: %d", numSlots)
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper")
	}

	// Gather Properties for each Modem.SimSlots entry.
	var simProperties []*dbusutil.Properties
	for i := 0; i < 2; i++ {
		simPath := simSlots[i]
		p, err := modem.GetSimProperties(ctx, simPath)
		if err != nil {
			s.Fatal("Failed to call Sim.GetProperties: ", err)
		}
		simProperties = append(simProperties, p)
	}

	// Ensure that a Cellular Service is created for each SIM.
	var foundErr error
	for i := 0; i < 2; i++ {
		simProps := simProperties[i]
		if err != nil {
			s.Fatal("Failed to call Sim.GetProperties: ", err)
		}
		iccid, err := simProps.GetString("SimIdentifier")
		if err != nil {
			s.Fatal("Missing Sim.SimIdentifier property: ", err)
		}
		props := map[string]interface{}{
			shillconst.ServicePropertyCellularICCID: iccid,
			shillconst.ServicePropertyConnectable:   true,
			shillconst.ServicePropertyType:          shillconst.TypeCellular,
		}
		_, err = helper.Manager.WaitForServiceProperties(ctx, props, 5*time.Second)
		if err != nil {
			errors.Wrapf(foundErr, "Cellular Service not found for ICCID: %s", iccid)
		}
	}
	if foundErr != nil {
		s.Fatal("Error(s) finding Cellular Services: ", foundErr)
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
	simSlotInfo := info.([]map[string]dbus.Variant)
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
