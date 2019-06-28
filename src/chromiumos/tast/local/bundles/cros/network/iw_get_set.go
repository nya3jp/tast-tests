// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
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
func resetState(ctx context.Context, s *testing.State) {
	s.Log("Resetting WiFi driver")
	if err := testexec.CommandContext(ctx, "modprobe", "-r", "iwlmvm",
		"iwlwifi").Run(testexec.DumpLogOnError); err != nil {
		s.Error("Modprobe down failed: ", err)
	}

	if err := testexec.CommandContext(ctx, "modprobe", "iwlwifi").Run(testexec.DumpLogOnError); err != nil {
		s.Error("Modprobe up failed: ", err)
	}
}

func IWGetSet(ctx context.Context, s *testing.State) {
	const iface = "wlan0"
	resetState(ctx, s)

	// Wait for device to fully be brought up.
	for flag := false; flag == false; {
		o, err := testexec.CommandContext(ctx, "ip", "link", "show").Output()
		if err != nil {
			s.Fatal(errors.Wrap(err, "ip link failed").Error())
		}
		if strings.Contains(string(o), iface) {
			flag = true
		}
		testing.Sleep(ctx, 5000*time.Millisecond)
	}

	if err := iw.SetTxPower(ctx, iface, "fixed", 1800); err != nil {
		s.Fatal("SetTxPower failed: ", err)
	}
	defer resetState(ctx, s)

	// TODO: merge in interface configuration code before I can test set freq.
	// if err := iw.SetFreq(ctx, iface, 2412); err != nil {
	// 	s.Fatal("SetFreq failed: ", err)
	// }

	if lv, err := iw.GetLinkValue(ctx, iface, "freq"); err != nil {
		s.Error("GetLinkValue failed: ", err)
	} else {
		s.Log("Frequency: ", lv)
	}
	if cfg, err := iw.GetRadioConfig(ctx, iface); err != nil {
		s.Error("GetRadioConfig failed: ", err)
	} else {
		s.Log("Config: ", cfg)
	}
}
