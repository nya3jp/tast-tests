// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type wirelessInteface struct {
	IEEEStandard      string `json:"ieee_standard"`
	InterfaceName     string `json:"interface_name"`
	PowerManagementOn bool   `json:"power_management_on"`
}

type networkInterface struct {
	WirelessInterfaces wirelessInteface `json:"wireless_interface"`
}

type wiFiInfo struct {
	NetworkInterfaces []networkInterface `json:"network_interfaces"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func: ProbeWifiInfo,
		Desc: "Check that we can probe cros_healthd for WiFi info",
		Contacts: []string{"pathan.jilani@intel.com",
			"intel-chrome-system-automation-team@intel.com",
			"cros-tdm-tpe-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		HardwareDeps: hwdep.D(hwdep.Model("brya")),
		Fixture:      "crosHealthdRunning",
	})
}

func validateWiFiInteface(wiFi wiFiInfo) error {
	if len(wiFi.NetworkInterfaces) < 1 {
		return errors.New("expected at least one WiFi interface")
	}

	for _, interfaces := range wiFi.NetworkInterfaces {
		if interfaces.WirelessInterfaces.IEEEStandard == "" {
			return errors.New("failed to get IEEEStandard")
		}
		if interfaces.WirelessInterfaces.InterfaceName == "" {
			return errors.New("failed to get InterfaceName")
		}
		if !interfaces.WirelessInterfaces.PowerManagementOn {
			return errors.New("failed to get PowerManagementOn")
		}
	}
	return nil
}

func ProbeWifiInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: "network_interface"}
	var wiFi wiFiInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &wiFi); err != nil {
		s.Fatal("Failed to get WiFi telemetry info: ", err)
	}
	if err := validateWiFiInteface(wiFi); err != nil {
		s.Fatal("Failed to validate WiFi data: ", err)
	}

}
