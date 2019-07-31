// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/network/iw"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     IWGetSet,
		Desc:     "Test IW getter and setter functions",
		Contacts: []string{"billyzhao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:     []string{"informational"},
	})
}

// resetState will reset the intel wifi driver.
// This function serves as our setup and cleanup routine.
func resetState(ctx context.Context, s *testing.State) {
	if err := testexec.CommandContext(ctx, "modprobe", "-r", "iwlmvm", "iwlwifi").Run(testexec.DumpLogOnError); err != nil {
		s.Error("Remove wireless driver failed: ", err)
	}
	if err := testexec.CommandContext(ctx, "modprobe", "iwlwifi").Run(testexec.DumpLogOnError); err != nil {
		s.Error("Start wireless driver failed: ", err)
	}
}

func IWGetSet(ctx context.Context, s *testing.State) {
	const iface = "wlan0"
	resetState(ctx, s)
	// Wait for device to fully be brought up.
	s.Log("Waiting for device to be brought up")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return testexec.CommandContext(ctx, "ip", "add", "sh", "dev", iface).Run()
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Failed to wait for device: ", err)
	}

	s.Log("Device brought up")

	// Run tests.
	if err := iw.SetTxPower(ctx, iface, "fixed", 1800); err != nil {
		s.Fatal("SetTxPower failed: ", err)
	}

	defer resetState(ctx, s)

	res, err := iw.GetRegulatoryDomain(ctx)
	if err != nil {
		s.Fatal("GetRegulatoryDomain failed: ", err)
	}
	s.Log("Regulatory Domain: ", res)
	// TODO: merge in interface configuration code before I can test set freq.
	// if err := iw.SetFreq(ctx, iface, 2412); err != nil {
	// 	s.Fatal("SetFreq failed: ", err)
	// }
}
