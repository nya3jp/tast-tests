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

	// Ensure that a Cellular Service is created for each Modem.SimSlots entry.
	var foundErr error
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
}
