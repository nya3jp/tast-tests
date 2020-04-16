// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bluetooth"
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
			"pmoy@chromium.org", // wilco_dtc_supportd author
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"vm_host", "wilco"},
		Pre:          pre.WilcoDtcSupportdAPI,
	})
}

func validateBluetoothData(msg *dtcpb.HandleBluetoothDataChangedRequest, adapterName, adapterAddress string, enableBluetooth bool) error {
	if len(msg.Adapters) != 1 {
		return errors.Errorf("unexpected adapters array size; got %d, want 1", len(msg.Adapters))
	}

	adapter := msg.Adapters[0]
	if adapter.AdapterName != adapterName {
		return errors.Errorf("unexpected adapter name; got %s, want %s", adapter.AdapterName, adapterName)
	}
	if adapter.AdapterMacAddress != adapterAddress {
		return errors.Errorf("unexpected adapter address; got %s, want %s", adapter.AdapterMacAddress, adapterAddress)
	}

	var want dtcpb.HandleBluetoothDataChangedRequest_AdapterData_CarrierStatus
	if enableBluetooth {
		want = dtcpb.HandleBluetoothDataChangedRequest_AdapterData_STATUS_UP
	} else {
		want = dtcpb.HandleBluetoothDataChangedRequest_AdapterData_STATUS_DOWN
	}

	if adapter.CarrierStatus != want {
		return errors.Errorf("unexpected carrier status; got %s, want %s", adapter.CarrierStatus, want)
	}

	return nil
}

func APIHandleBluetoothDataChanged(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	adapters, err := bluetooth.Adapters(ctx)
	if err != nil {
		s.Fatal("Unable to get Bluetooth adapters: ", err)
	}

	if len(adapters) != 1 {
		s.Fatalf("Unexpected Bluetooth adapters count; got %d, want 1", len(adapters))
	}

	adapter := adapters[0]

	name, err := adapter.Name(ctx)
	if err != nil {
		s.Fatal("Unable to get name property value: ", err)
	}

	address, err := adapter.Address(ctx)
	if err != nil {
		s.Fatal("Unable to get address property value: ", err)
	}

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
				msg := dtcpb.HandleBluetoothDataChangedRequest{}
				if err := rec.WaitForMessage(ctx, &msg); err != nil {
					s.Fatal("Unable to receive Bluetooth event: ", err)
				}

				if err := validateBluetoothData(&msg, name, address, enable); err != nil {
					s.Logf("Unable to validate Bluetooth data %v: %v", msg, err)
				} else {
					break
				}
			}
		}
	}
}
