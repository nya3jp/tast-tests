// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/network/iw"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     IWScan,
		Desc:     "Verifies `iw` Timed Scan executes and is parsed properly",
		Contacts: []string{"billyzhao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:     []string{"informational"},
	})
}

func IWScan(ctx context.Context, s *testing.State) {
	// We lose connectivity briefly. Tell recover_duts not to worry.
	unlock, err := network.LockCheckNetworkHook(ctx)
	if err != nil {
		s.Fatal("Failed to lock the check network hook: ", err)
	}
	defer unlock()

	// Stop shill temporarily and remove the default profile.
	if err := shill.SafeStop(ctx); err != nil {
		s.Fatal("Failed stopping shill: ", err)
	}
	defer func() {
		if err := shill.SafeStart(ctx); err != nil {
			s.Fatal("Failed starting shill: ", err)
		}
	}()
	// Bring up wireless device after it's released from shill.
	s.Log("Enable wifi manually")
	if err := testexec.CommandContext(ctx, "ifconfig", "wlan0", "up").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Could not bring up wlan0 after disable")
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := iw.TimedScan(ctx, "wlan0", nil, nil)
		if err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second}); err != nil {
		s.Fatal("TimedScan failed: ", err)
	}

}
