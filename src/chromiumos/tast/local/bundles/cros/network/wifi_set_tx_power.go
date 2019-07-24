// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
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
		mode string
		args []string
	}{
		// Run tablet mode first, then switch back to non-tablet mode.
		{"tablet", []string{"--tablet"}},
		{"non-tablet", []string{}},
	} {
		if err := testexec.CommandContext(ctx, setTxPowerExe, tc.args...).Run(testexec.DumpLogOnError); err != nil {
			s.Errorf("Failed to set TX power for %s mode: %v", tc.mode, err)
		}
	}
}
