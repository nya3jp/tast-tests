// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bluetooth/bluez"
	"chromiumos/tast/local/bundles/cros/wilco/bt"
	"chromiumos/tast/local/bundles/cros/wilco/pre"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: APIHandleBluetoothDataChanged,
		Desc: "Tests that the Wilco DTC VM receives Bluetooth events using the DPSL",
		Contacts: []string{
			"lamzin@google.com", // Test author and wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"vm_host", "wilco"},
		Pre:          pre.WilcoDtcSupportdAPI,
	})
}

func APIHandleBluetoothDataChanged(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	adapters, err := bluez.Adapters(ctx)
	if err != nil {
		s.Fatal("Unable to get Bluetooth adapters: ", err)
	}

	if len(adapters) != 1 {
		s.Fatalf("Unexpected Bluetooth adapters count; got %d, want 1", len(adapters))
	}

	adapter := adapters[0]
	powered, err := adapter.Powered(ctx)
	if err != nil {
		s.Fatal("Unable to get powered property value: ", err)
	}

	if err := adapter.SetPowered(ctx, false); err != nil {
		s.Fatal("Unable to disable Bluetooth adapter: ", err)
	}

	// Put Bluetooth adapter to the same state as it was before test run.
	defer adapter.SetPowered(cleanupCtx, powered)

	rec, err := wilco.NewDPSLMessageReceiver(ctx)
	if err != nil {
		s.Fatal("Unable to create DPSL Message Receiver: ", err)
	}
	defer rec.Stop(cleanupCtx)

	// Repeat tests to make sure they're not influenced by system events.
	for i := 0; i < 10; i++ {
		for _, enable := range []bool{true, false} {
			if err := adapter.SetPowered(ctx, enable); err != nil {
				s.Fatalf("Unable to set Bluetooth powered property to %t: %v", enable, err)
			}

			for {
				s.Log("Waiting for Bluetooth event")
				msg := &dtcpb.HandleBluetoothDataChangedRequest{}
				if err := rec.WaitForMessage(ctx, msg); err != nil {
					s.Fatal("Unable to receive Bluetooth event: ", err)
				}

				if err := bt.ValidateBluetoothData(ctx, msg, bt.ExpectPowered(enable)); err != nil {
					s.Logf("Unable to validate Bluetooth data %v: %v", msg, err)
				} else {
					break
				}
			}
		}
	}
}
