// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/local/bundles/cros/network/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCellularSim,
		Desc:     "Verifies that Cellular Device and Service properties match ModemManager SIM properties",
		Contacts: []string{"stevenjb@google.com", "chromeos-cellular-team@google.com"},
		Attr:     []string{"group:cellular"},
		Fixture:  "cellular",
	})
}

func ShillCellularSim(ctx context.Context, s *testing.State) {
	// Gather ModemManager properties
	modem, err := modemmanager.NewModemWithSim(ctx)
	if err != nil {
		s.Fatal("Failed to create Modem: ", err)
	}
	modemProps, err := modem.GetProperties(ctx)
	if err != nil {
		s.Fatal("Failed to call GetProperties on Modem: ", err)
	}
	simPath, err := modemProps.GetObjectPath(mmconst.ModemPropertySim)
	if err != nil {
		s.Fatal("Failed to get Modem.Sim property: ", err)
	}
	s.Log("SIM path = ", simPath)
	simProps, err := modem.GetSimProperties(ctx, simPath)
	if err != nil {
		s.Fatalf("Failed to create Sim for path: %q: %s", simPath, err)
	}
	simICCID, err := simProps.GetString(mmconst.SimPropertySimIdentifier)
	if err != nil {
		s.Fatal("Failed to get Sim.SimIdentifier property: ", err)
	}

	// Gather Shill Device properties
	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper")
	}
	deviceProps, err := helper.Device.GetProperties(ctx)
	if err != nil {
		s.Fatal("Failed to get Device properties: ", err)
	}
	if simPresent, err := deviceProps.GetBool(shillconst.DevicePropertyCellularSIMPresent); err != nil {
		s.Fatal("Failed to get Device.Cellular.SIMPresent property: ", err)
	} else if !simPresent {
		s.Fatal("SIMPresent property not set")
	}
	deviceICCID, err := deviceProps.GetString(shillconst.DevicePropertyCellularICCID)
	if err != nil {
		s.Fatal("Failed to get Device.Cellular.ICCID property: ", err)
	}

	// Ensure Shill Device ICCID and ModemManager ICCID match.
	if deviceICCID != simICCID {
		s.Fatalf("Device ICCID does not match SIM, got %q, want %q", deviceICCID, simICCID)
	}

	// Ensure that Shill creates a Service matching ICCID.
	if _, err = helper.FindServiceForDevice(ctx); err != nil {
		s.Fatal("Failed to get Cellular Service for Device: ", err)
	}
}
