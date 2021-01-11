// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ModemmanagerSlots,
		Desc:     "Verifies that modemmanager reports multiple SIM slots",
		Contacts: []string{"stevenjb@google.com", "cros-network-health@google.com"},
		Attr:     []string{"group:cellular"},
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
	simSlots, err := props.GetObjectPaths("SimSlots")
	if err != nil {
		s.Fatal("Missing SimSlots property: ", err)
	}
	primary, err := props.GetUInt32("PrimarySimSlot")
	if err != nil {
		s.Fatal("Missing PrimarySimSlot property: ", err)
	}
	if int(primary) > len(simSlots) {
		s.Fatalf("Invalid PrimarySimSlot: %d", primary)
	}
	sim, err := props.GetObjectPath("Sim")
	if err != nil {
		s.Fatal("Missing Sim property: ", err)
	}
	if sim != simSlots[primary-1] {
		s.Fatalf("Sim property mismatch: %s", sim)
	}
}
