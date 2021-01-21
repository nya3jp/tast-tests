// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/local/bundles/cros/network/cellular"
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
		Attr:     []string{"group:cellular_esim"},
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
	if numSlots < 2 {
		s.Fatalf("Expected at least 2 SIM slots, found: %d", numSlots)
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper")
	}

	// Ensure that a Cellular Service is created for each Modem.SimSlots entry.
	for i := 0; i < 2; i++ {
		simPath := simSlots[i]
		simProps, err := modem.GetSimProperties(ctx, simPath)
		if err != nil {
			s.Fatal("Failed to call Sim.GetProperties: ", err)
		}
		iccid, err := simProps.GetString("SimIdentifier")
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
}
