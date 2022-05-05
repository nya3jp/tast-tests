// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/local/jsontypes"
	"chromiumos/tast/local/set"
	"chromiumos/tast/testing"
)

type deviceInfo struct {
	Address           string   `json:"address"`
	Name              string   `json:"name"`
	Type              string   `json:"type"`
	Appearance        uint16   `json:"appearance"`
	Modalias          string   `json:"modalias"`
	MTU               int16    `json:"mtu"`
	RSSI              uint16   `json:"rssi"`
	UUIDs             []string `json:"uuids"`
	BatteryPercentage []string `json:"battery_percentage"`
}

type capabilitiesInfo struct {
	MaxAdvLen    uint8 `json:"max_adv_len"`
	MaxScnRspLen uint8 `json:"max_scn_rsp_len"`
	MaxTxPower   int16 `json:"max_tx_power"`
	MinTxPower   int16 `json:"min_tx_power"`
}

type adapterInfo struct {
	Address               string           `json:"address"`
	Name                  string           `json:"name"`
	NumConnectedDevices   jsontypes.Uint32 `json:"num_connected_devices"`
	Powered               bool             `json:"powered"`
	ConnectedDevices      []deviceInfo     `json:"connected_devices"`
	Discoverable          bool             `json:"discoverable"`
	Discovering           bool             `json:"discovering"`
	UUIDs                 []string         `json:"uuids"`
	Modalias              string           `json:"modalias"`
	ServiceAllowList      []string         `json:"service_allow_list"`
	SupportedCapabilities capabilitiesInfo `json:"supported_capabilities"`
}

type bluetoothInfo struct {
	Adapters []adapterInfo `json:"adapters"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ProbeBluetoothInfo,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that cros_healthd can fetch Bluetooth info",
		Contacts:     []string{"cros-tdm-tpe-eng@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

func validateBluetoothAdapterData(ctx context.Context, s *testing.State, info *bluetoothInfo) error {
	// Get Bluetooth adapter values to compare to the output of cros_healthd.
	adapters, err := bluetooth.Adapters(ctx)
	if err != nil {
		return err
	}

	// If cros_healthd and D-Bus both report no adapters, there is no output to
	// verify.
	if len(info.Adapters) == 0 && len(adapters) == 0 {
		return nil
	}

	if len(adapters) != 1 {
		return errors.Errorf("unexpected Bluetooth adapters count: got %d; want 1", len(adapters))
	}

	if len(info.Adapters[0].ConnectedDevices) != int(info.Adapters[0].NumConnectedDevices) {
		return errors.Errorf("inconsistent number of connected Bluetooth devices: got %d; want %d",
			len(info.Adapters[0].ConnectedDevices),
			info.Adapters[0].NumConnectedDevices)
	}

	if name, err := adapters[0].Name(ctx); err != nil {
		return err
	} else if info.Adapters[0].Name != name {
		return errors.Errorf("invalid name: got %s; want %s", info.Adapters[0].Name, name)
	}

	if address, err := adapters[0].Address(ctx); err != nil {
		return err
	} else if info.Adapters[0].Address != address {
		return errors.Errorf("invalid address: got %s; want %s", info.Adapters[0].Address, address)
	}

	if powered, err := adapters[0].Powered(ctx); err != nil {
		return err
	} else if info.Adapters[0].Powered != powered {
		return errors.Errorf("invalid powered value: got %v; want %v", info.Adapters[0].Powered, powered)
	}

	if discoverable, err := adapters[0].Discoverable(ctx); err != nil {
		return err
	} else if info.Adapters[0].Discoverable != discoverable {
		return errors.Errorf("invalid discoverable value: got %v; want %v", info.Adapters[0].Discoverable, discoverable)
	}

	if discovering, err := adapters[0].Discovering(ctx); err != nil {
		return err
	} else if info.Adapters[0].Discovering != discovering {
		return errors.Errorf("invalid discovering value: got %v; want %v", info.Adapters[0].Discovering, discovering)
	}

	if uuids, err := adapters[0].UUIDs(ctx); err != nil {
		return err
	} else if len(set.DiffStringSlice(info.Adapters[0].UUIDs, uuids)) != 0 {
		return errors.Errorf("invalid uuids value: got %v; want %v", info.Adapters[0].UUIDs, uuids)
	}

	if modalias, err := adapters[0].Modalias(ctx); err != nil {
		return err
	} else if info.Adapters[0].Modalias != modalias {
		return errors.Errorf("invalid modalias value: got %v; want %v", info.Adapters[0].Modalias, modalias)
	}

	if err := validateAdminPolicy(ctx, info, adapters[0]); err != nil {
		return err
	}

	if err := validateAdvertising(ctx, info, adapters[0]); err != nil {
		return err
	}

	return nil
}

func validateAdminPolicy(ctx context.Context, info *bluetoothInfo, adapter *bluetooth.Adapter) error {
	// Clear allowed services.
	if b, err := testexec.CommandContext(ctx, "bluetoothctl", "admin.allow", "clear").Output(testexec.DumpLogOnError); err != nil {
		return errors.Errorf("clear allowed service failed: %v", b)
	}
	// Set the allowed services for validation.
	if b, err := testexec.CommandContext(ctx, "bluetoothctl", "admin.allow", "110d", "110c", "110b").Output(testexec.DumpLogOnError); err != nil {
		return errors.Errorf("set allowed service failed: %v", b)
	}

	if serviceAllowList, err := adapter.ServiceAllowList(ctx); err != nil {
		return err
	} else if len(serviceAllowList) != 3 {
		return errors.Errorf("unexpected allowed services count: got %d; want 3", len(serviceAllowList))
	} else if len(set.DiffStringSlice(info.Adapters[0].ServiceAllowList, serviceAllowList)) != 0 {
		return errors.Errorf("invalid serviceAllowList value: got %v; want %v", info.Adapters[0].ServiceAllowList, serviceAllowList)
	}

	return nil
}

func validateAdvertising(ctx context.Context, info *bluetoothInfo, adapter *bluetooth.Adapter) error {
	if supportedCapabilities, err := adapter.SupportedCapabilities(ctx); err != nil {
		return err
	} else if info.Adapters[0].SupportedCapabilities.MaxAdvLen != supportedCapabilities.MaxAdvLen ||
		info.Adapters[0].SupportedCapabilities.MaxScnRspLen != supportedCapabilities.MaxScnRspLen ||
		info.Adapters[0].SupportedCapabilities.MaxTxPower != supportedCapabilities.MaxTxPower ||
		info.Adapters[0].SupportedCapabilities.MinTxPower != supportedCapabilities.MinTxPower {
		return errors.Errorf("invalid supportedCapabilities value: got %v; want %v", info.Adapters[0].SupportedCapabilities, supportedCapabilities)
	}

	return nil
}

func ProbeBluetoothInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryBluetooth}
	var info bluetoothInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &info); err != nil {
		s.Fatal("Failed to get Bluetooth telemetry info: ", err)
	}

	if err := validateBluetoothAdapterData(ctx, s, &info); err != nil {
		s.Fatalf("Failed to validate bluetooth adapter data, err [%v]", err)
	}
}
