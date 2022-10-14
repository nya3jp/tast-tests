// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
	"chromiumos/tast/testing/wlan"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SetTXPower,
		Desc: "Tests WiFi TX power helper's basic operation",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr: []string{"group:mainline", "group:wificell", "wificell_func", "wificell_dut_validation", "group:firmware", "firmware_ec", "group:labqual"},
		Params: []testing.Param{
			{
				// This test only runs on devices which do not use VPD SAR tables.
				ExtraHardwareDeps: hwdep.D(hwdep.WifiNoVpdSar()),
			},
			{
				// This test only runs on devices which use VPD SAR tables.
				Name:              "vpd",
				ExtraHardwareDeps: hwdep.D(hwdep.WifiVpdSar()),
			},
		},
	})
}

func SetTXPower(ctx context.Context, s *testing.State) {
	const setTxPowerExe = "set_wifi_transmit_power"

	cmd := testexec.CommandContext(ctx, "check_powerd_config", "--set_wifi_transmit_power")
	if err := cmd.Run(); err != nil {
		if ws, ok := testexec.GetWaitStatus(err); ok && ws.ExitStatus() == 1 {
			s.Log("DUT does not support WiFi power table switching")
			return
		}
		defer cmd.DumpLog(ctx)
		s.Fatal("Failed to run check_powerd_config: ", err)
	}

	// Check to see if this is a static device based on the configuration.
	// If this is a static device, verify only the supported mode succeeds.
	staticMode, err := crosconfig.Get(ctx, "/power", "wifi-transmit-power-mode-for-static-device")
	if crosconfig.IsNotFound(err) {
		s.Log("Testing dynamic mode")
	} else if err != nil {
		s.Fatalf("Failed to execute cros_config: %s", err)
	} else {
		if staticMode != "tablet" && staticMode != "non-tablet" {
			s.Fatalf("Invalid static mode: %s", staticMode)
		}
		s.Logf("Testing static mode: %s", staticMode)
	}

	// Get the information of the WLAN device.
	devInfo, err := wlan.DeviceInfo()
	if err != nil {
		s.Fatal("Failed reading the WLAN device information: ", err)
	}

	marvell88w8897SDIO := wlan.DeviceNames[wlan.Marvell88w8897SDIO]
	marvell88w8997PCIE := wlan.DeviceNames[wlan.Marvell88w8997PCIE]

	modes := []string{"tablet", "notablet"}
	domains := []string{"fcc", "eu", "rest-of-world", "none"}
	sources := []string{"tablet_mode", "reg_domain", "proximity", "udev_event", "unknown"}
	for _, mode := range modes {
		for _, domain := range domains {
			for _, source := range sources {
				supported := true
				// Marvel devices does not support changing the tx power based on reg_domain.
				if source == "reg_domain" && (devInfo.Name == marvell88w8897SDIO || devInfo.Name == marvell88w8997PCIE) {
					supported = false
				} else {
					// Dynamic devices support all modes, whereas static devices
					// only support the specified mode.
					var currentMode string
					// powerd config uses the name "non-tablet" which represents the same power mode as the "notablet"
					// command argument.
					if mode == "notablet" {
						currentMode = "non-tablet"
					} else {
						currentMode = mode
					}
					supported = len(staticMode) == 0 || (len(staticMode) != 0 && currentMode == staticMode)
				}

				// Supported modes must not fail, and unsupported modes must not succeed.
				var args []string
				args = append(args, "--"+mode, "--domain="+domain, "--source="+source)
				var err error
				if supported {
					err = testexec.CommandContext(ctx, setTxPowerExe, args...).Run(testexec.DumpLogOnError)
				} else {
					err = testexec.CommandContext(ctx, setTxPowerExe, args...).Run()
				}
				if supported && err != nil {
					s.Errorf("Failed to set TX power for %s mode with reg domain %s and trigger source %s: %v", mode, domain, source, err)
				} else if !supported && err == nil {
					s.Errorf("Succeeded setting unsupported TX power for %s mode with reg domain %s and trigger source %s: %v", mode, domain, source, err)
				}
			}
		}
	}
}
