// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"

	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type wirelessInteface struct {
	InterfaceName     string `json:"interface_name"`
	PowerManagementOn bool   `json:"power_management_on"`
}

type networkInterface struct {
	WirelessInterfaces wirelessInteface `json:"wireless_interface"`
}

type wifiInfo struct {
	NetworkInterfaces []networkInterface `json:"network_interfaces"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ProbeWifiInfo,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Check that we can probe cros_healthd for WIFI info",
		Contacts: []string{
			"cros-tdm-tpe-eng@google.com",
			"pathan.jilani@intel.com",
			"intel-chrome-system-automation-team@intel.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		// TODO(b/207569436): Define hardware dependency and get rid of hard-coding the models.
		HardwareDeps: hwdep.D(hwdep.Model("brya", "redrix", "kano", "anahera", "primus", "crota")),
		Fixture:      "crosHealthdRunning",
	})
}

func ProbeWifiInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: "network_interface"}
	var wifi wifiInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &wifi); err != nil {
		s.Fatal("Failed to get WIFI telemetry info: ", err)
	}
	for _, interfaces := range wifi.NetworkInterfaces {
		if interfaces.WirelessInterfaces.InterfaceName == "" {
			s.Fatal("Failed to get InterfaceName")
		}
		if !interfaces.WirelessInterfaces.PowerManagementOn {
			s.Fatal("Failed to get PowerManagementOn")
		}
	}
}
