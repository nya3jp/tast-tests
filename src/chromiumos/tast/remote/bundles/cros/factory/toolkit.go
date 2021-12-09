// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package factory

import (
	"context"
	"time"

	"chromiumos/tast/remote/bundles/cros/factory/fixture"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Toolkit,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Test if toolkit is running",
		Contacts:     []string{"lschyi@google.com", "chromeos-factory-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      time.Minute,
		Fixture:      fixture.EnsureToolkit,
		// Skip "nyan_kitty" due to slow reboot speed.
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("kitty")),
		SoftwareDeps: append([]string{"factory_flow"}, fixture.EnsureToolkitSoftwareDeps...),
	})
}

func Toolkit(ctx context.Context, s *testing.State) {
	conn := s.DUT().Conn()
	probeTestListCmd := conn.CommandContext(ctx, "factory", "test-list")
	if err := probeTestListCmd.Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to run toolkit: ", err)
	}
}
