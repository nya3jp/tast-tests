// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"os"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

type wirelessLinkInfo struct {
	AccessPointAddress string `json:"access_point_address_str"`
	EncyptionOn        bool   `json:"encyption_on"`
	LinkQuality        string `json:"link_quality"`
	RxBitRateMbps      string `json:"rx_bit_rate_mbps"`
	SignalLevelDBm     int    `json:"signal_level_dBm"`
	TxBitRateMbps      string `json:"tx_bit_rate_mbps"`
	TxPowerDBm         int    `json:"tx_power_dBm"`
}

type wirelessInteface struct {
	InterfaceName     string            `json:"interface_name"`
	LinkInfo          *wirelessLinkInfo `json:"link_info"`
	PowerManagementOn bool              `json:"power_management_on"`
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
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check that we can probe cros_healthd for WIFI info",
		Contacts: []string{
			"cros-tdm-tpe-eng@google.com",
			"pathan.jilani@intel.com",
			"intel-chrome-system-automation-team@intel.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
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
		for _, ifc := range wifi.NetworkInterfaces {
			if ifc.WirelessInterfaces.InterfaceName != "wlan0" {
				s.Fatal("Failed to get InterfaceName")
			}
			if _, err := os.Stat("/sys/module/iwlmvm/parameters/power_scheme"); !os.IsNotExist(err) {
				if !ifc.WirelessInterfaces.PowerManagementOn {
					s.Fatal("Failed to get PowerManagementOn value as true when power_scheme present")
				}
			} else {
				if ifc.WirelessInterfaces.PowerManagementOn {
					s.Fatal("Failed to get PowerManagementOn value as false when power_scheme not present")
				}
			}
			// Check whether router connected or not.
			iwconfigOut, err := testexec.CommandContext(ctx, "iwconfig").Output()
			if err != nil {
				s.Fatal("Failed to execute iwconfig command: ", err)
			}
			// Check whether 'Access Point: Not-Associated' is presented or not.
			want = "Access Point: Not-Associated"
			isRouterConnected := !strings.Contains(string(iwconfigOut), want)
			if !isRouterConnected {
				if ifc.WirelessInterfaces.LinkInfo != nil {
					s.Fatal("Failed to validate empty LinkInfo data")
				}
			} else {
				if ifc.WirelessInterfaces.LinkInfo.AccessPointAddress == "" {
					s.Fatal("Failed to get AccessPointAddress")
				}
				if !ifc.WirelessInterfaces.LinkInfo.EncyptionOn {
					s.Fatal("Failed to get EncyptionOn")
				}
				if ifc.WirelessInterfaces.LinkInfo.LinkQuality == "" {
					s.Fatal("Failed to get LinkQuality")
				}
				if ifc.WirelessInterfaces.LinkInfo.RxBitRateMbps == "" {
					s.Fatal("Failed to get RxBitRateMbps")
				}
				if ifc.WirelessInterfaces.LinkInfo.SignalLevelDBm > 0 {
					s.Fatal("Failed to get SignalLevelDBm")
				}
				if ifc.WirelessInterfaces.LinkInfo.TxBitRateMbps == "" {
					s.Fatal("Failed to get TxBitRateMbps")
				}
				if ifc.WirelessInterfaces.LinkInfo.TxPowerDBm < 0 {
					s.Fatal("Failed to get TxPowerDBm")
				}
			}
		}
	} else {
		if len(wifi.NetworkInterfaces) > 0 {
			s.Fatal("Failed to validate empty NetworkInterfaces data")
		}
	}
}
