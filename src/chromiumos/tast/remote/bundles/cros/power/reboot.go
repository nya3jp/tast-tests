// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"

	"chromiumos/tast/dut"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Reboot,
		Desc:         "Verifies that system comes back after rebooting",
		Contacts: []string{"derat@chromium.org", "tast-users@chromium.org"},
		SoftwareDeps: []string{"reboot"},
	})
}

func Reboot(ctx context.Context, s *testing.State) {
	d, ok := dut.FromContext(ctx)
	if !ok {
		s.Fatal("Failed to get DUT")
	}

	s.Log("Rebooting DUT")
	// Run the reboot command in the background to avoid the DUT potentially going down before
	// success is reported over the SSH connection. Redirect all I/O streams to ensure that the
	// SSH exec request doesn't hang (see https://en.wikipedia.org/wiki/Nohup#Overcoming_hanging).
	cmd := "nohup sh -c 'sleep 2; reboot' >/dev/null 2>&1 </dev/null &"
	if _, err := d.Run(ctx, cmd); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	s.Log("Waiting for DUT to become unreachable")
	if err := d.WaitUnreachable(ctx); err != nil {
		s.Fatal("Failed to wait for DUT to become unreachable: ", err)
	}
	s.Log("DUT became unreachable (as expected)")

	s.Log("Reconnecting to DUT")
	if err := d.WaitConnect(ctx); err != nil {
		s.Fatal("Failed to reconnect to DUT: ", err)
	}
	s.Log("Reconnected to DUT")
}
