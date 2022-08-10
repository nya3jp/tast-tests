// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package bt contains helper functions to work with Bluetooth.
package bt

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bluetooth/bluez"
	dtcpb "chromiumos/wilco_dtc"
)

type bluetoothAdapter struct {
	name    string
	address string
	powered bool
}

type option func(*bluetoothAdapter)

// ExpectPowered returns an option that configures expected powered
// value of Bluetooth adapter for ValidateBluetoothData function.
func ExpectPowered(powered bool) option {
	return func(adapter *bluetoothAdapter) {
		adapter.powered = powered
	}
}

// ValidateBluetoothData validates whether HandleBluetoothDataChangedRequest
// contains correct Bluetooth data.
func ValidateBluetoothData(ctx context.Context, msg *dtcpb.HandleBluetoothDataChangedRequest, opts ...option) error {
	adapters, err := bluez.Adapters(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to get Bluetooth adapters")
	}

	if len(adapters) != 1 {
		return errors.Errorf("unexpected Bluetooth adapters count; got %d, want 1", len(adapters))
	}

	adapter := adapters[0]
	var btAdapter bluetoothAdapter
	btAdapter.name, err = adapter.Name(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to get name property value")
	}
	btAdapter.address, err = adapter.Address(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to get address property value")
	}
	btAdapter.powered, err = adapter.Powered(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to get powered property value")
	}

	for _, opt := range opts {
		opt(&btAdapter)
	}

	if len(msg.Adapters) != 1 {
		return errors.Errorf("unexpected adapters array size; got %d, want 1", len(msg.Adapters))
	}

	msgAdapter := msg.Adapters[0]

	if msgAdapter.AdapterName != btAdapter.name {
		return errors.Errorf("unexpected adapter name; got %s, want %s", msgAdapter.AdapterName, btAdapter.name)
	}
	if msgAdapter.AdapterMacAddress != btAdapter.address {
		return errors.Errorf("unexpected adapter address; got %s, want %s", msgAdapter.AdapterMacAddress, btAdapter.address)
	}

	expectedCarrierStatus := dtcpb.HandleBluetoothDataChangedRequest_AdapterData_STATUS_DOWN
	if btAdapter.powered {
		expectedCarrierStatus = dtcpb.HandleBluetoothDataChangedRequest_AdapterData_STATUS_UP
	}

	if msgAdapter.CarrierStatus != expectedCarrierStatus {
		return errors.Errorf("unexpected carrier status; got %s, want %s", msgAdapter.CarrierStatus, expectedCarrierStatus)
	}

	return nil
}
