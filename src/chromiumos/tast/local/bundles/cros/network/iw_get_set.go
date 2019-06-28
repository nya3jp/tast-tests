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
		Desc:     "Verifies `iw` Timed Scan executes and is parsed properly",
		Contacts: []string{"billyzhao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:     []string{"informational"},
	})
}

// resetState will reset the intel wifi driver.
// This function serves as our setup and cleanup routine.
func resetState(ctx context.Context) []error {
	var e []error
	e = append(e, testexec.CommandContext(ctx, "modprobe", "-r", "iwlmvm",
		"iwlwifi").Run(testexec.DumpLogOnError))
	e = append(e, testexec.CommandContext(ctx, "modprobe", "iwlwifi").Run(testexec.DumpLogOnError))
	return e
}

func IWGetSet(ctx context.Context, s *testing.State) {
	const iface = "wlan0"
	e := resetState(ctx)
	for _, err := range e {
		if err != nil {
			s.Error("Failed in resetState: ", err)
		}
	}
	// Wait for device to fully be brought up.
	s.Log("Waiting for device to be brought up")
	t := func(ctx context.Context) error {
		return testexec.CommandContext(ctx, "ip", "add", "sh", "dev", iface).Run()
	}
	opts := testing.PollOptions{Timeout: 5 * time.Second, Interval: 100 * time.Millisecond}
	testing.Poll(ctx, t, &opts)
	s.Log("Device brought up")

	// Run tests
	if err := iw.SetTxPower(ctx, iface, "fixed", 1800); err != nil {
		s.Fatal("SetTxPower failed: ", err)
	}

	defer func() {
		e := resetState(ctx)
		for _, err := range e {
			if err != nil {
				s.Error("Failed in resetState: ", err)
			}
		}

	}()

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
