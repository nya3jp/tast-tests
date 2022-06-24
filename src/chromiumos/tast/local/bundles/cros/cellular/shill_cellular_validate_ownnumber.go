// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCellularValidateOwnnumber,
		Desc:     "Verifies that Cellular Device could fetch same OwnNumbers property from modemmanager and shill device",
		Contacts: []string{"srikanthkumar@google.com", "chromeos-cellular-team@google.com"},
		Attr:     []string{"group:cellular", "cellular_unstable", "cellular_sim_active"},
		Fixture:  "cellular",
	})
}

func ShillCellularValidateOwnnumber(ctx context.Context, s *testing.State) {
	// Gather ModemManager properties
	modem, err := modemmanager.NewModemWithSim(ctx)
	if err != nil {
		s.Fatal("Failed to create Modem: ", err)
	}
	modemProps, err := modem.GetProperties(ctx)
	if err != nil {
		s.Fatal("Failed to call GetProperties on Modem: ", err)
	}

	// Read modem property OwnNumbers from ModemManager.
	value, err := modemProps.Get(mmconst.ModemPropertyOwnNumbers)
	if err != nil {
		s.Fatal("Failed to read OwnNumbers property: ", err)
	}
	if value == nil {
		s.Fatal("OwnNumbers property does not exist")
	}

	phoneNumbers, ok := value.([]string)
	if !ok {
		s.Fatal("OwnNumbers property type conversion failed")
	}
	if len(phoneNumbers) < 1 {
		s.Fatal("Empty OwnNumbers property")
	}
	modemOwnNumber := phoneNumbers[0]
	s.Logf("OwnNumber on modem: %s", modemOwnNumber)

	// Gather Shill Device properties
	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.simOwnNumberHelper")
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
	deviceOwnNumber, err := deviceProps.GetString(shillconst.DevicePropertyCellularMDN)
	if err != nil {
		s.Fatal("Failed to get Device.Cellular.MDN property: ", err)
	}
	s.Logf("OwnNumber on shill device: %s", deviceOwnNumber)

	// Ensure Shill Device OwnNumber and ModemManager OwnNumber match.
	if deviceOwnNumber != modemOwnNumber {
		s.Fatalf("Shill Device OwnNumber does not match modem, got %q, want %q", deviceOwnNumber, modemOwnNumber)
	}

}
