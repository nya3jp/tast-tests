// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wilco/pre"
	"chromiumos/tast/local/dbusutil"
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
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"vm_host", "wilco"},
		Pre:          pre.WilcoDtcSupportdAPI,
	})
}

func enableBluetooth(ctx context.Context, enable bool) error {
	const (
		name     = "org.bluez"
		path     = "/org/bluez/hci0"
		property = "org.bluez.Adapter1.Powered"
	)
	_, obj, err := dbusutil.Connect(ctx, name, path)
	if err != nil {
		return errors.Wrap(err, "failed to create D-Bus connection to Bluetooth adapter")
	}

	if err := dbusutil.SetProperty(ctx, obj, property, enable); err != nil {
		return err
	}
	return nil
}

func validateBluetoothData(msg *dtcpb.HandleBluetoothDataChangedRequest, enableBluetooth bool) error {
	if len(msg.Adapters) != 1 {
		return errors.Errorf("unexpected adapters array size; got %d, want 1", len(msg.Adapters))
	}

	adapter := msg.Adapters[0]
	if len(adapter.AdapterName) == 0 {
		return errors.New("received adapter with empty name")
	}
	if len(adapter.AdapterMacAddress) == 0 {
		return errors.New("received adapter with empty MAC address")
	}

	var want dtcpb.HandleBluetoothDataChangedRequest_AdapterData_CarrierStatus
	if enableBluetooth {
		want = dtcpb.HandleBluetoothDataChangedRequest_AdapterData_STATUS_UP
	} else {
		want = dtcpb.HandleBluetoothDataChangedRequest_AdapterData_STATUS_DOWN
	}

	if adapter.CarrierStatus != want {
		return errors.Errorf("unexpected carrier status; got %q, want %q", adapter.CarrierStatus, want)
	}

	return nil
}

func APIHandleBluetoothDataChanged(ctx context.Context, s *testing.State) {
	if err := enableBluetooth(ctx, false); err != nil {
		s.Error("Unable to disable Bluetooth: ", err)
	}

	rec, err := wilco.NewDPSLMessageReceiver(ctx)
	if err != nil {
		s.Fatal("Unable to create DPSL Message Receiver: ", err)
	}
	defer rec.Stop(ctx)

	for _, enable := range []bool{true, false} {
		if err := enableBluetooth(ctx, enable); err != nil {
			s.Fatalf("Unable to set Bluetooth powered property to %q: %q", enable, err)
		}

		s.Log("Waiting for Bluetooth event")
		msg := dtcpb.HandleBluetoothDataChangedRequest{}
		if err := rec.WaitForMessage(ctx, &msg); err != nil {
			s.Fatal("Unable to receive Bluetooth event: ", err)
		}
		s.Log("Received Bluetooth data: ", msg)

		if err := validateBluetoothData(&msg, enable); err != nil {
			s.Fatal("Unable to validate Bluetooth data: ", err)
		}
	}

}
