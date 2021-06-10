// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/errors"
	//	"chromiumos/tast/ctxutil"

	//	"chromiumos/tast/errors"
//	"chromiumos/tast/local/bundles/cros/network/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// This test runs on all carriers including esim. Slot switch is tested on boards with Modem,
// SimSlots property and at least two SIM slots and one active profile on each slot.

func init() {
	testing.AddTest(&testing.Test{
		Func:     ModemmanagerConnect,
		Desc:     "Verifies that modemmanager switches SIM slot",
		Contacts: []string{"srikanthkumar@google.com", "cros-network-health@google.com"},
		Attr:     []string{"group:cellular", "cellular_unstable"},
		Fixture:  "cellular",
		Timeout:  5 * time.Minute,
	})
}

// ModemmanagerConnect Test
func ModemmanagerConnect(ctx context.Context, s *testing.State) {

//	helper, err := cellular.NewHelper(ctx)
//	if err != nil {
//		s.Fatal("Failed to create cellular.Helper: ", err)
//	}

	upstart.StopJob(ctx, "shill")
	defer upstart.StartJob(ctx,"shill")
//	// Disable cellular if present and defer re-enabling.
//	if _, err := helper.Manager.DisableTechnologyForTesting(ctx, shill.TechnologyCellular); err != nil {
//		s.Fatal("Unable to disable cellular: ", err)
//	} 
//	//else if enableFunc != nil {
//		// newCtx, cancel := ctxutil.Shorten(ctx, shill.EnableWaitTime)
//		// defer cancel()
//		// defer enableFunc(ctx)
//		// ctx = newCtx
////	}

	modem, err := modemmanager.NewModem(ctx)
	if err != nil {
		s.Fatal("Failed to create modem: ", err)
	}

	for i := 0; i < 3; i++ {
    s.Logf("Disabling modem")
	if modem.Call(ctx, "Enable", false); err != nil {
		s.Fatal("Failed to disable modem", err)
	}

	if modem.Call(ctx, "SetPowerState", uint32(2)); err != nil {
		s.Fatal("Failed to put modem into low power state", err)
	}

    s.Logf("Enabling modem")
	if modem.Call(ctx, "Enable", true); err != nil {
		s.Fatal("Failed to enable modem", err)
	}
    
	props, err := modem.GetProperties(ctx)
	if err := testing.Poll(ctx, func (ctx context.Context) error {
	val, err := props.GetInt32(mmconst.ModemPropertyState)
	if err != nil {
		// You may shortcut the polling if an unexpected error happens.
		s.Log("early error")
	}
	if val != 9 {
		// The error will return when the poll timeout.
		return errors.Errorf("Val %v, want %v", val, 9)
	}
	// Successful. Polling ends.
	return nil
}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed waiting for registered", err)
}

	s.Logf("Connecting to modem")
	modemSimple, err := modem.GetSimple(ctx) 
	if err != nil {
		s.Fatal("Could not get MM's simple interface")
	}
	simpleProperties := map[string]interface{}{
		"apn" : "broadband",
	}
	modemSimple.Call(ctx,"Connect",simpleProperties)
	}

	props, err := modem.GetProperties(ctx)
	if err != nil {
		s.Fatal("Failed to call GetProperties on Modem: ", err)
	}

	simSlots, err := props.GetObjectPaths(mmconst.ModemPropertySimSlots)
	if err != nil {
		s.Fatal("Failed to get SimSlots property: ", err)
	}
	if len(simSlots) == 0 {
		s.Log("No SimSlots for device, ending test")
	} else {
		s.Log("simSlots: ", simSlots)
	}

}
