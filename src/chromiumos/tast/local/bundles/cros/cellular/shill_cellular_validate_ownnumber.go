// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"strings"

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
		Vars:     []string{"autotest_host_info_labels"},
	})
}

func ShillCellularValidateOwnnumber(ctx context.Context, s *testing.State) {

	// Gather Device properties from host info store labels.
	labels, err := cellular.GetLabelsAsStringArray(ctx, s.Var, "autotest_host_info_labels")
	if err != nil {
		s.Fatal("Failed to read autotest_host_info_labels: ", err)
	}
	// Gather ModemManager properties
	modem, err := modemmanager.NewModemWithSim(ctx)
	if err != nil {
		s.Fatal("Failed to create Modem: ", err)
	}
	modemProps, err := modem.GetProperties(ctx)
	if err != nil {
		s.Fatal("Failed to call GetProperties on Modem: ", err)
	}

	// Skip test on duts not having phone number/own number(amarisoft connected)
	helper, err := cellular.NewHelperWithLabels(ctx, labels)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	labelOwnNumber := helper.modemInfo.OwnNumber
	if labelOwnNumber == mmconst.StaticModemOwnNumber {
		s.Log("Skip test as its no OwnNumber dut")
		return
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
	if !strings.Contains(modemOwnNumber, deviceOwnNumber) {
		s.Fatalf("Shill Device OwnNumber does not match modem, got %q, want %q", deviceOwnNumber, modemOwnNumber)
	}

}
