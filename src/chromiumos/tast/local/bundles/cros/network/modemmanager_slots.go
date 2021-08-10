// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ModemmanagerSlots,
		Desc:     "Verifies that modemmanager reports multiple SIM slots",
		Contacts: []string{"stevenjb@google.com", "cros-network-health@google.com"},
		Attr:     []string{"group:cellular", "group:cellular-cq"},
		Fixture:  "cellular",
	})
}

// ModemmanagerSlots Test
func ModemmanagerSlots(ctx context.Context, s *testing.State) {
	modem, err := modemmanager.NewModem(ctx)
	if err != nil {
		s.Fatal("Failed to create Modem: ", err)
	}
	props, err := modem.GetProperties(ctx)
	if err != nil {
		s.Fatal("Failed to call GetProperties on Modem: ", err)
	}
	sim, err := props.GetObjectPath(mmconst.ModemPropertySim)
	if err != nil {
		s.Fatal("Missing Sim property: ", err)
	}
	simSlots, err := props.GetObjectPaths(mmconst.ModemPropertySimSlots)
	if err != nil {
		s.Fatal("Failed to get SimSlots property: ", err)
	}
	if len(simSlots) == 0 {
		s.Log("No SimSlots for device, ending test")
		return
	}
	primary, err := props.GetUint32(mmconst.ModemPropertyPrimarySimSlot)
	if err != nil {
		s.Fatal("Missing PrimarySimSlot property: ", err)
	}
	s.Log("Primary SIM slot: ", primary)
	if int(primary) > len(simSlots) {
		s.Fatal("Invalid PrimarySimSlot: ", primary)
	}
	if sim != simSlots[primary-1] {
		s.Fatalf("Sim property mismatch: %s", sim)
	}
}
