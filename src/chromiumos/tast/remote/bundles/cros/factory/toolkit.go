// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package factory

import (
	"context"
	"time"

	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

var (
	unstableToolkitPlatforms = []string{"dedede", "drallion360", "kasumi", "kled", "fennel", "kakadu", "garg360", "dumo", "volteer", "zork"}
	unstableToolkitModels    = []string{"hana64", "nami", "nami-kernelnext", "octopus", "homestar", "ultima"}
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Toolkit,
		Desc:     "Test if toolkit is running",
		Contacts: []string{"lschyi@google.com", "chromeos-factory-eng@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Timeout:  time.Minute,
		Fixture:  "ensureToolkit",
		// Skip "nyan_kitty" due to slow reboot speed.
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("kitty")),
		Params: []testing.Param{{
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(unstableToolkitPlatforms...), hwdep.SkipOnModel(unstableToolkitModels...)),
		}, {
			Name:              "informational",
			ExtraAttr:         []string{"informational"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(unstableToolkitPlatforms...), hwdep.Model(unstableToolkitModels...)),
		}},
	})
}

func Toolkit(ctx context.Context, s *testing.State) {
	conn := s.DUT().Conn()
	probeTestListCmd := conn.CommandContext(ctx, "factory", "test-list")
	if err := probeTestListCmd.Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to run toolkit: ", err)
	}
}
