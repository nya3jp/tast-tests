// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECConsole,
		Desc:         "Verifies that EC console is working",
		Contacts:     []string{"jbettis@chromium.org", "cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware", "firmware_ec", "firmware_bringup"},
		Vars:         []string{"servo"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

// ECConsole opens the EC console and runs the version command.
func ECConsole(ctx context.Context, s *testing.State) {
	servoSpec, _ := s.Var("servo")
	h := firmware.NewHelperWithoutDUT("", servoSpec, s.DUT().KeyFile(), s.DUT().KeyDir())
	defer h.Close(ctx)

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to require servo: ", err)
	}

	strings, err := h.Servo.RunECCommandGetOutput(ctx, "version", []string{`Chip:\s*([^\n]*)\n`})
	if err != nil {
		s.Fatal("ec console version command: ", err)
	}
	s.Logf("EC Chip: %s", strings[0][1])
}
