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
		Func:         H1Console,
		Desc:         "Verifies that H1 console is working",
		Contacts:     []string{"jbettis@chromium.org", "cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware", "firmware_cr50", "firmware_bringup"},
		Vars:         []string{"servo"},
		HardwareDeps: hwdep.D(hwdep.GSCUART()),
	})
}

// H1Console opens the H1 (cr50) console and runs the version command.
func H1Console(ctx context.Context, s *testing.State) {
	servoSpec, _ := s.Var("servo")
	h := firmware.NewHelperWithoutDUT("", servoSpec, s.DUT().KeyFile(), s.DUT().KeyDir())
	defer h.Close(ctx)

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to require servo: ", err)
	}

	strings, err := h.Servo.RunCR50CommandGetOutput(ctx, "version", []string{`Chip:\s*([^\n]*)\n`})
	if err != nil {
		s.Fatal("cr50 console version command: ", err)
	}
	s.Logf("H1 Chip: %s", strings[0][1])
}
