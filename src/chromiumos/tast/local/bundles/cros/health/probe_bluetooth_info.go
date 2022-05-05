// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/local/jsontypes"
	"chromiumos/tast/testing"
)

type deviceInfo struct {
	Address    string   `json:"address"`
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Appearance uint16   `json:"appearance"`
	Modalias   string   `json:"modalias"`
	MTU        int16    `json:"mtu"`
	RSSI       uint16   `json:"rssi"`
	UUIDs      []string `json:"uuids"`
}

type adapterInfo struct {
	Address             string           `json:"address"`
	Name                string           `json:"name"`
	NumConnectedDevices jsontypes.Uint32 `json:"num_connected_devices"`
	Powered             bool             `json:"powered"`
	ConnectedDevices    []deviceInfo     `json:"connected_devices"`
	Discoverable        bool             `json:"discoverable"`
	Discovering         bool             `json:"discovering"`
	UUIDs               []string         `json:"uuids"`
	Modalias            string           `json:"modalias"`
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

func compareSlice(cands, targets []string) bool {
	if len(cands) != len(targets) {
		return false
	}
	for _, target := range targets {
		found := false
		for _, cand := range cands {
			if cand == target {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
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
	} else if !compareSlice(info.Adapters[0].UUIDs, uuids) {
		return errors.Errorf("invalid uuids value: got %v; want %v", info.Adapters[0].UUIDs, uuids)
	}

	if modalias, err := adapters[0].Modalias(ctx); err != nil {
		return err
	} else if info.Adapters[0].Modalias != modalias {
		return errors.Errorf("invalid modalias value: got %v; want %v", info.Adapters[0].Modalias, modalias)
	}

	// Get AdminPolicyStatus1 interface.
	adminPolicies, err := bluetooth.AdminPolicies(ctx)
	if err != nil {
		return err
	}
	var adminPolicy *bluetooth.AdminPolicy
	for _, cand := range adminPolicies {
		if cand.Path() == adapters[0].Path() {
			adminPolicy = cand
		}
	}
	if adminPolicy == nil {
		return errors.Errorf("could not find object path %v in AdminPolicyStatus1 interface", adapters[0].Path())
	}

	if serviceAllowList, err := adminPolicy.ServiceAllowList(ctx); err != nil {
		return err
	} else if !compareSlice(info.Adapters[0].ServiceAllowList, serviceAllowList) {
		return errors.Errorf("invalid serviceAllowList value: got %v; want %v", info.Adapters[0].ServiceAllowList, serviceAllowList)
	}

	// Get LEAdvertisingManager1 interface.
	advertisings, err := bluetooth.Advertisings(ctx)
	if err != nil {
		return err
	}
	var advertising *bluetooth.Advertising
	for _, cand := range advertisings {
		if cand.Path() == adapters[0].Path() {
			advertising = cand
		}
	}
	if advertising == nil {
		return errors.Errorf("could not find object path %v in LEAdvertisingManager1 interface", adapters[0].Path())
	}

	if supportedCapabilities, err := advertising.SupportedCapabilities(ctx); err != nil {
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
