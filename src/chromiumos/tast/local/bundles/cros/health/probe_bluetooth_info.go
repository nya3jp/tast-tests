// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"encoding/json"
	"strings"

	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ProbeBluetoothInfo,
		Desc: "Checks that cros_healthd can fetch Bluetooth info",
		Contacts: []string{
			"pmoy@google.com",
			"cros-tdm@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

type adapterInfo struct {
	Address             string `json:"address"`
	Name                string `json:"name"`
	NumConnectedDevices int    `json:"num_connected_devices"`
	Powered             bool   `json:"powered"`
}

type bluetoothInfo struct {
	Adapters []adapterInfo `json:"adapters"`
}

func ProbeBluetoothInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryBluetooth}
	rawData, err := croshealthd.RunTelem(ctx, params, s.OutDir())
	if err != nil {
		s.Fatal("Failed to get Bluetooth telemetry info: ", err)
	}

	dec := json.NewDecoder(strings.NewReader(string(rawData)))
	dec.DisallowUnknownFields()

	var info bluetoothInfo
	if err := dec.Decode(&info); err != nil {
		s.Fatalf("Failed to decode bluetooth data [%q], err [%v]", rawData, err)
	}

	// Get Bluetooth adapter values to compare to the output of cros_healthd.
	adapters, err := bluetooth.Adapters(ctx)
	if err != nil {
		s.Fatal("Unable to get Bluetooth adapters: ", err)
	}

	// If cros_healthd and D-Bus both report no adapters, there is no output to
	// verify.
	if len(info.Adapters) == 0 && len(adapters) == 0 {
		return
	}

	if len(adapters) != 1 {
		s.Fatalf("Unexpected Bluetooth adapters count: got %d; want 1", len(adapters))
	}

	if name, err := adapters[0].Name(ctx); err != nil {
		s.Fatal("Unable to get name property value: ", err)
	} else if info.Adapters[0].Name != name {
		s.Errorf("Invalid name: got %s; want %s", info.Adapters[0].Name, name)
	}

	if address, err := adapters[0].Address(ctx); err != nil {
		s.Fatal("Unable to get address property value: ", err)
	} else if info.Adapters[0].Address != address {
		s.Errorf("Invalid address: got %s; want %s", info.Adapters[0].Address, address)
	}

	if powered, err := adapters[0].Powered(ctx); err != nil {
		s.Fatal("Unable to get powered property value: ", err)
	} else if info.Adapters[0].Powered != powered {
		s.Errorf("Invalid powered value: got %v; want %v", info.Adapters[0].Powered, powered)
	}

	if info.Adapters[0].NumConnectedDevices < 0 {
		s.Errorf("Invalid num_connected_devices: got %v; want 0+", info.Adapters[0].NumConnectedDevices)
	}
}
