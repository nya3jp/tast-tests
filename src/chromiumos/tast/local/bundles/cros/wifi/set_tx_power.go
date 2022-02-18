// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

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

	// For both static and dynamic devices, attempt to set transmit power.
	for _, tc := range []struct {
		mode   string
		domain string
		args   []string
	}{
		// Iterate through each combination of regdomain and tablet mode.
		{"tablet", "fcc", []string{"--tablet", "--domain=fcc"}},
		{"tablet", "eu", []string{"--tablet", "--domain=eu"}},
		{"tablet", "rest-of-world", []string{"--tablet", "--domain=rest-of-world"}},
		{"tablet", "none", []string{"--tablet", "--domain=none"}},
		{"non-tablet", "fcc", []string{"--notablet", "--domain=fcc"}},
		{"non-tablet", "eu", []string{"--notablet", "--domain=eu"}},
		{"non-tablet", "rest-of-world", []string{"--notablet", "--domain=rest-of-world"}},
		{"non-tablet", "none", []string{"--notablet", "--domain=none"}},
	} {
		// Dynamic devices support all modes, whereas static devices
		// only support the specified mode.
		supported := len(staticMode) == 0 || (len(staticMode) != 0 && tc.mode == staticMode)

		// Supported modes must not fail, and unsupported modes must not succeed.
		s.Logf("Testing mode: %s, domain: %s, staticMode: %s, supported: %t", tc.mode, tc.domain, staticMode, supported)
		err := testexec.CommandContext(ctx, setTxPowerExe, tc.args...).Run(testexec.DumpLogOnError)
		if supported && err != nil {
			s.Errorf("Failed to set TX power for %s mode with reg domain %s: %v", tc.mode, tc.domain, err)
		} else if !supported && err == nil {
			s.Errorf("Succeeded setting unsupported TX power for %s mode with reg domain %s: %v", tc.mode, tc.domain, err)
		}
	}
}
