// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SetTXPower,
		Desc: "Tests WiFi TX power helper's basic operation",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		SoftwareDeps: []string{"tablet_mode"},
		Attr:         []string{"group:mainline", "group:wificell", "wificell_func"},
		Params: []testing.Param{
			{
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(badTXPowerModels...)),
			},
			{
				Name:              "informational",
				ExtraAttr:         []string{"informational", "wificell_unstable"},
				ExtraHardwareDeps: hwdep.D(hwdep.Model(badTXPowerModels...)),
			},
		},
	})
}

// These models are known to fail this test, and so we cannot run them as 'critical'. We run them as
// 'informational', while tracking followup bugs to fix them.
var badTXPowerModels = []string{}

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
