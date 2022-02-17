// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
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

		Fixture: "crosHealthdRunning",
	})
}

func ProbeWifiInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: "network_interface"}
	var wifi wifiInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &wifi); err != nil {
		s.Fatal("Failed to get WIFI telemetry info: ", err)
	}
	out, err := testexec.CommandContext(ctx, "iw", "dev").Output()
	if err != nil {
		s.Fatal("Failed to execute 'iw dev' command: ", err)
	}

	// Check whether 'Interface wlan0' is presented or not.
	want := "Interface wlan0"
	if got := string(out); strings.Contains(got, want) {
		for _, interfaces := range wifi.NetworkInterfaces {
			if interfaces.WirelessInterfaces.InterfaceName == "" {
				s.Fatal("Failed to get InterfaceName")
			}
			if !interfaces.WirelessInterfaces.PowerManagementOn {
				s.Fatal("Failed to get PowerManagementOn")
			}
		}
	} else {
		if len(wifi.NetworkInterfaces) > 0 {
			s.Fatal("Failed to validate empty NetworkInterfaces data")
		}

	}
}
