// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/local/bundles/cros/network/iw"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	iface = "wlan0"
	phy   = "phy0"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     IWGetSet,
		Desc:     "Verifies `iw` Timed Scan executes and is parsed properly",
		Contacts: []string{"billyzhao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:     []string{"informational"},
	})
}

func IWGetSet(ctx context.Context, s *testing.State) {
	if err := iw.SetTxPower(ctx, iface, "fixed", 1800); err != nil {
		s.Error("SetTxPower failed: ", err)
	}
	defer func() {
		s.Log("cleaning up")
		if err := testexec.CommandContext(ctx, "modprobe", "-r", "iwlmvm",
			"iwlwifi").Run(testexec.DumpLogOnError); err != nil {
			s.Error("Modprobe down failed: ", err)
		}

		if err := testexec.CommandContext(ctx, "modprobe", "iwlwifi").Run(testexec.DumpLogOnError); err != nil {
			s.Error("Modprobe up failed: ", err)
		}
	}()

	// TODO: merge in interface configuration code before I can test set freq.
	// if err := iw.SetFreq(ctx, iface, 2412); err != nil {
	// 	s.Fatal("SetFreq failed: ", err)
	// }

	lv, err := iw.GetLinkValue(ctx, iface, "freq")
	if err != nil {
		s.Error("GetLinkValue failed: ", err)
	}
	s.Log(lv)
	cfg, err := iw.GetRadioConfig(ctx, iface)
	if err != nil {
		s.Error("GetRadioConfig failed: ", err)
	}
	s.Log(cfg)

}
