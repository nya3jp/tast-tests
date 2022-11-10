// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bluetooth/bluez"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/local/jsontypes"
	"chromiumos/tast/local/set"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
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
	Class             uint32   `json:"bluetooth_class"`
}

type capabilitiesInfo struct {
	MaxAdvLen    uint8 `json:"max_adv_len"`
	MaxScnRspLen uint8 `json:"max_scn_rsp_len"`
	MaxTxPower   int16 `json:"max_tx_power"`
	MinTxPower   int16 `json:"min_tx_power"`
}

type adapterInfo struct {
	Address               string            `json:"address"`
	Name                  string            `json:"name"`
	NumConnectedDevices   jsontypes.Uint32  `json:"num_connected_devices"`
	Powered               bool              `json:"powered"`
	ConnectedDevices      []deviceInfo      `json:"connected_devices"`
	Discoverable          bool              `json:"discoverable"`
	Discovering           bool              `json:"discovering"`
	UUIDs                 []string          `json:"uuids"`
	Modalias              string            `json:"modalias"`
	ServiceAllowList      []string          `json:"service_allow_list"`
	SupportedCapabilities *capabilitiesInfo `json:"supported_capabilities"`
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
		HardwareDeps: hwdep.D(hwdep.Bluetooth()),
	})
}

var targetAllowedServices = []string{"110d", "110c", "110b"}

// resetBluetoothAdapterData clean the preset properties in adapter.
func resetBluetoothAdapterData(ctx context.Context) error {
	// Clear allowed services.
	if b, err := testexec.CommandContext(ctx, "bluetoothctl", "admin.allow", "clear").Output(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "clear allowed service failed: %s", string(b))
	}
	return nil
}

// initiateBluetoothAdapterData setup the required data we want to validate.
// Because serviceAllowList is always an empty list in lab device, we set the
// policy before the testing.
func initiateBluetoothAdapterData(ctx context.Context) error {
	if err := resetBluetoothAdapterData(ctx); err != nil {
		return err
	}
	// Set the allowed services for validation.
	args := append([]string{"admin.allow"}, targetAllowedServices...)
	if b, err := testexec.CommandContext(ctx, "bluetoothctl", args...).Output(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "set allowed service failed: %s", string(b))
	}
	return nil
}

func validateBluetoothAdapterData(ctx context.Context, info *bluetoothInfo) error {
	// Get Bluetooth adapter values to compare to the output of cros_healthd.
	adapters, err := bluez.Adapters(ctx)
	if err != nil {
		return err
	}

	if len(adapters) != 1 {
		return errors.Errorf("unexpected Bluetooth adapters count: got %d; want 1", len(adapters))
	}

	if len(info.Adapters[0].ConnectedDevices) != int(info.Adapters[0].NumConnectedDevices) {
		return errors.Errorf("inconsistent number of connected Bluetooth devices: got %d; want %d",
			len(info.Adapters[0].ConnectedDevices),
			info.Adapters[0].NumConnectedDevices)
	}

	if err := validateAdapter(ctx, info, adapters[0]); err != nil {
		return err
	}

	if err := validateAdminPolicy(ctx, info, adapters[0]); err != nil {
		return err
	}

	if err := validateAdvertising(ctx, info, adapters[0]); err != nil {
		return err
	}

	return nil
}

// validateAdapter validate the data from Adapter1 interface.
func validateAdapter(ctx context.Context, info *bluetoothInfo, adapter *bluez.Adapter) error {
	if name, err := adapter.Name(ctx); err != nil {
		return err
	} else if info.Adapters[0].Name != name {
		return errors.Errorf("invalid name: got %s; want %s", info.Adapters[0].Name, name)
	}

	if address, err := adapter.Address(ctx); err != nil {
		return err
	} else if info.Adapters[0].Address != address {
		return errors.Errorf("invalid address: got %s; want %s", info.Adapters[0].Address, address)
	}

	if powered, err := adapter.Powered(ctx); err != nil {
		return err
	} else if info.Adapters[0].Powered != powered {
		return errors.Errorf("invalid powered value: got %v; want %v", info.Adapters[0].Powered, powered)
	}

	if discoverable, err := adapter.Discoverable(ctx); err != nil {
		return err
	} else if info.Adapters[0].Discoverable != discoverable {
		return errors.Errorf("invalid discoverable value: got %v; want %v", info.Adapters[0].Discoverable, discoverable)
	}

	if discovering, err := adapter.Discovering(ctx); err != nil {
		return err
	} else if info.Adapters[0].Discovering != discovering {
		return errors.Errorf("invalid discovering value: got %v; want %v", info.Adapters[0].Discovering, discovering)
	}

	if uuids, err := adapter.UUIDs(ctx); err != nil {
		return err
	} else if len(set.DiffStringSlice(info.Adapters[0].UUIDs, uuids)) != 0 {
		return errors.Errorf("invalid uuids value: got %v; want %v", info.Adapters[0].UUIDs, uuids)
	}

	if modalias, err := adapter.Modalias(ctx); err != nil {
		return err
	} else if info.Adapters[0].Modalias != modalias {
		return errors.Errorf("invalid modalias value: got %v; want %v", info.Adapters[0].Modalias, modalias)
	}

	return nil
}

// validateAdminPolicy validate the data from AdminPolicyStatus1 interface.
func validateAdminPolicy(ctx context.Context, info *bluetoothInfo, adapter *bluez.Adapter) error {
	if serviceAllowList, err := adapter.ServiceAllowList(ctx); err != nil {
		return err
	} else if len(serviceAllowList) != len(targetAllowedServices) {
		return errors.Errorf("unexpected allowed services count: got %d; want %d", len(serviceAllowList), len(targetAllowedServices))
	} else if len(set.DiffStringSlice(info.Adapters[0].ServiceAllowList, serviceAllowList)) != 0 {
		return errors.Errorf("invalid serviceAllowList value: got %v; want %v", info.Adapters[0].ServiceAllowList, serviceAllowList)
	}

	return nil
}

// validateAdvertising validate the data from LEAdvertisingManager1 interface.
func validateAdvertising(ctx context.Context, info *bluetoothInfo, adapter *bluez.Adapter) error {
	if supportedCapabilities, err := adapter.SupportedCapabilities(ctx); err != nil {
		// Pass if neither cros_healthd nor D-Bus has supportedCapabilities.
		if info.Adapters[0].SupportedCapabilities == nil {
			return nil
		}
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
	if err := initiateBluetoothAdapterData(ctx); err != nil {
		s.Fatalf("Failed to initiate bluetooth adapter data, err [%v]", err)
	}

	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryBluetooth}
	var info bluetoothInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &info); err != nil {
		s.Fatal("Failed to get Bluetooth telemetry info: ", err)
	}

	if err := validateBluetoothAdapterData(ctx, &info); err != nil {
		s.Fatalf("Failed to validate bluetooth adapter data, err [%v]", err)
	}

	if err := resetBluetoothAdapterData(ctx); err != nil {
		s.Fatalf("Failed to reset bluetooth adapter data, err [%v]", err)
	}
}
