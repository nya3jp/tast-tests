// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"strconv"

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

func IWGetSet(ctx context.Context, s *testing.State) {
	iface := "wlan0"
	phy := "phy0"
	defer func() {
		s.Log("cleaning up.")
		testexec.CommandContext(ctx, "modprobe", "-r", "iwlmvm", "iwlwifi")
		testexec.CommandContext(ctx, "modprobe", "iwlwifi")
	}()
	err := iw.SetTxPower(ctx, iface, 1)
	if err != nil {
		s.Fatal("SetTxPower failed: ", err)
	}
	err := iw.SetFreq(ctx, iface, 2412)
	if err != nil {
		s.Fatal("SetFreq failed: ", err)
	}
	err := iw.SetRegulatoryDomain(ctx, "LV")
	if err != nil {
		s.Fatal("SetTxPower failed: ", err)
	}
	fragThresh, err := iw.GetFragmentationThreshold(ctx, phy)
	if err != nil {
		s.Fatal("GetFragmentationThreshold failed: ", err)
	}
	s.Log(strconv.Itoa(fragThresh))
	lv, err := iw.GetLinkValue(ctx, iface, "freq")
	if err != nil {
		s.Fatal("GetLinkValue failed: ", err)
	}
	s.Log(lv)
	cfg, err := iw.GetRadioConfig(ctx, iface)
	if err != nil {
		s.Fatal("GetRadioConfig failed: ", err)
	}
	s.Log(cfg)

}
