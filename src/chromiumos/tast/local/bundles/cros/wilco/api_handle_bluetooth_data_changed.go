// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
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

const (
	btName     = "org.bluez"
	btPath     = "/org/bluez/hci0"
	btProperty = "org.bluez.Adapter1.Powered"
)

func bluetoothEnabled(ctx context.Context) (bool, error) {
	_, obj, err := dbusutil.Connect(ctx, btName, btPath)
	if err != nil {
		return false, errors.Wrap(err, "failed to create D-Bus connection to Bluetooth adapter")
	}

	v, err := dbusutil.Property(ctx, obj, btProperty)
	if err != nil {
		return false, err
	}
	powered, ok := v.(bool)
	if !ok {
		return false, errors.Errorf("received non-bool D-Bus property value: %v", v)
	}
	return powered, nil
}

func enableBluetooth(ctx context.Context, enable bool) error {
	_, obj, err := dbusutil.Connect(ctx, btName, btPath)
	if err != nil {
		return errors.Wrap(err, "failed to create D-Bus connection to Bluetooth adapter")
	}
	return dbusutil.SetProperty(ctx, obj, btProperty, enable)
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
		return errors.Errorf("unexpected carrier status; got %s, want %s", adapter.CarrierStatus, want)
	}

	return nil
}

func APIHandleBluetoothDataChanged(ctx context.Context, s *testing.State) {
	btEnable, err := bluetoothEnabled(ctx)
	if err != nil {
		s.Fatal("Unable to determine whether Bluetooth enable: ", err)
	}
	defer func(ctx context.Context) {
		if err := enableBluetooth(ctx, btEnable); err != nil {
			s.Errorf("Unable to restore Bluetooth powered property to %t: %v", btEnable, err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	if err := enableBluetooth(ctx, false); err != nil {
		s.Fatal("Unable to disable Bluetooth: ", err)
	}

	rec, err := wilco.NewDPSLMessageReceiver(ctx)
	if err != nil {
		s.Fatal("Unable to create DPSL Message Receiver: ", err)
	}
	defer rec.Stop(ctx)

	ctx, cancel = ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	for _, enable := range []bool{true, false} {
		if err := enableBluetooth(ctx, enable); err != nil {
			s.Fatalf("Unable to set Bluetooth powered property to %t: %v", enable, err)
		}

		s.Log("Waiting for Bluetooth event")
		msg := dtcpb.HandleBluetoothDataChangedRequest{}
		if err := rec.WaitForMessage(ctx, &msg); err != nil {
			s.Fatal("Unable to receive Bluetooth event: ", err)
		}

		if err := validateBluetoothData(&msg, enable); err != nil {
			s.Errorf("Unable to validate Bluetooth data %v: %v", msg, err)
		}
	}
}
