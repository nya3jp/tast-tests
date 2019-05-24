// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"os"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WifiSetTxPower,
		Desc: "Tests WiFi TX power helper's basic operation",
		Contacts: []string{
			"briannorris@chromium.org",        // Author
			"chromeos-kernel-wifi@google.com", // WiFi team
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"tablet_mode"},
	})
}

func WifiSetTxPower(ctx context.Context, s *testing.State) {
	const (
		// TODO: add unibuild support.
		tabletModePref = "/usr/share/power_manager/board_specific/set_wifi_transmit_power_for_tablet_mode"
		setTxPowerExe  = "set_wifi_transmit_power"
	)

	if _, err := os.Stat(tabletModePref); err != nil && os.IsNotExist(err) {
		s.Log("DUT does not support WiFi power switching")
		return
	}

	for _, args := range [][]string{
		// Run tablet mode first, then switch back to laptop mode.
		{"--tablet"},
		{},
	} {
		cmd := testexec.CommandContext(ctx, setTxPowerExe, args...)
		err := cmd.Run()
		if err != nil {
			cmd.DumpLog(ctx)
			s.Error("Failed to set TX power: ", err)
		}
	}
}
