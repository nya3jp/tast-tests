// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WifiSetTXPower,
		Desc: "Tests WiFi TX power helper's basic operation",
		Contacts: []string{
			"briannorris@chromium.org",        // Author
			"chromeos-kernel-wifi@google.com", // WiFi team
		},
		SoftwareDeps: []string{"tablet_mode"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{
			{
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(
					"maglia", // TODO(b/177656181): Broken, causing CQ issues.
					"dalboz", // TODO(b/162258095): Dalboz lab DUTs have Qualcomm chip
				)),
			},
			{
				Name:      "informational",
				ExtraAttr: []string{"informational"},
				ExtraHardwareDeps: hwdep.D(hwdep.Model(
					"maglia", // TODO(b/177656181): Broken, causing CQ issues.
					"dalboz", // TODO(b/162258095): Dalboz lab DUTs have Qualcomm chip
				)),
			},
		},
	})
}

func WifiSetTXPower(ctx context.Context, s *testing.State) {
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

	for _, tc := range []struct {
		mode   string
		domain string
		args   []string
	}{
		// Iterate through each combination of regdomain and tablet mode.
		{"tablet", "fcc", []string{"--tablet", "--domain=fcc"}},
		{"tablet", "eu", []string{"--tablet", "--domain=eu"}},
		{"tablet", "rest-of-world", []string{"--tablet", "--domain=rest-of-world"}},
		{"tablet", "none", []string{"--notablet", "--domain=none"}},
		{"non-tablet", "fcc", []string{"--notablet", "--domain=fcc"}},
		{"non-tablet", "eu", []string{"--notablet", "--domain=eu"}},
		{"non-tablet", "rest-of-world", []string{"--notablet", "--domain=rest-of-world"}},
		{"non-tablet", "none", []string{"--notablet", "--domain=none"}},
	} {
		if err := testexec.CommandContext(ctx, setTxPowerExe, tc.args...).Run(testexec.DumpLogOnError); err != nil {
			s.Errorf("Failed to set TX power for %s mode with reg domain %s: %v", tc.mode, tc.domain, err)
		}
	}
}
