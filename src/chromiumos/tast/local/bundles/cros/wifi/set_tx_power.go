// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SetTXPower,
		Desc: "Tests WiFi TX power helper's basic operation",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr: []string{"group:mainline", "group:wificell", "wificell_func", "wificell_dut_validation", "group:firmware", "firmware_ec", "group:labqual"},
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

	modes := []string{"--tablet", "--notablet"}
	domains := []string{"--domain=fcc", "--domain=eu", "--domain=rest-of-world", "--domain=none"}
	sources := []string{"--source=tablet_mode", "--source=reg_domain", "--source=proximity", "--source=udev_event", "--source=unknown"}
	for _, mode := range modes {
		for _, domain := range domains {
			for _, source := range sources {
				// Dynamic devices support all modes, whereas static devices
				// only support the specified mode.
				supported := len(staticMode) == 0 || (len(staticMode) != 0 && mode == staticMode)

				// Supported modes must not fail, and unsupported modes must not succeed.
				s.Logf("Testing mode=%s, %s, %s, staticMode=%s, supported=%t", strings.TrimLeft(mode, "--"), strings.TrimLeft(domain, "--"), strings.TrimLeft(source, "--"), staticMode, supported)
				var args []string
				args = append(args, mode, domain, source)
				err := testexec.CommandContext(ctx, setTxPowerExe, args...).Run(testexec.DumpLogOnError)
				if supported && err != nil {
					s.Errorf("Failed to set TX power for %s mode with reg domain %s and trigger source %s: %v", mode, domain, source, err)
				} else if !supported && err == nil {
					s.Errorf("Succeeded setting unsupported TX power for %s mode with reg domain %s and trigger source %s: %v", mode, domain, source, err)
				}
			}
		}
	}
}
