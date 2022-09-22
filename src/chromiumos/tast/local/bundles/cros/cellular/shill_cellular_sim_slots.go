// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/hermes"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
)

// This test is only run on the cellular_esim group. All boards in that group
// provide the Modem.SimSlots property and have at least two SIM slots.

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCellularSimSlots,
		Desc:     "Verifies that Shill receives SimSlot information from ModemManager and can set the primary sim slot",
		Contacts: []string{"stevenjb@google.com", "cros-network-health@google.com", "chromeos-cellular-team@google.com"},
		Attr:     []string{"group:cellular", "cellular_sim_dual_active", "cellular_ota_avl"},
		Fixture:  "cellular",
	})
}

func ShillCellularSimSlots(ctx context.Context, s *testing.State) {
	// The test only checks that shill creates a cellular service for the pSIM when the eSIM slot is active.
	// Shill doesn't create a service for the eSIM if it is on the inactive slot since the service is not usable. See go/dual-sim-mbim
	// If Chrome wishes to create a service for the inactive eSIM, Chrome calls Hermes to switch slots, enables any profiles, before shill creates a service for the eSIM.
	euicc, _, err := hermes.GetEUICC(ctx, false)
	if err != nil {
		s.Fatal("Unable to get Hermes euicc: ", err)
	}
	p, err := euicc.EnabledProfile(ctx)
	if err != nil {
		s.Fatal("Could not read profile status: ", err)
	}
	if p == nil {
		s.Fatal("Expected a profile to be enabled on the eSIM")
	}

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
		if iccid == "" {
			// Shill represents unknown pSIM iccid = "" as "unknown-iccid".
			iccid = shillconst.UnknownICCID
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
		if err := compareSimProps(simProps, mmconst.SimPropertySimIdentifier, slotInfo, "ICCID"); err != nil {
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
	if simKey == mmconst.SimPropertySimIdentifier && simp == "" {
		// Shill represents MM iccid = "" as "unknown-iccid".
		simp = shillconst.UnknownICCID
	}
	slotp := slotInfo[slotKey].Value()
	if simp != slotp {
		return errors.Errorf("property mismatch, got: %s, want: %s", slotp, simp)
	}
	return nil
}
