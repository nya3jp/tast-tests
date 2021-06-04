// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/remote/servo"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FpFlashFpMcuHello,
		Desc: "Validate that flash_fp_mcu can communicate with the FPMCU's bootloader",
		Contacts: []string{
			"hesling@chromium.org", // Test author
			"chromeos-fingerprint@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
		// On hatch+bloonchipper flash_fp_mcu --hello takes about 4 sec.
		Timeout:      1 * time.Minute,
		SoftwareDeps: []string{"biometrics_daemon"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
		Vars:         []string{"servo"},
	})
}

// FpFlashFpMcuHello checks if the flash_fp_mcu script can properly communicate
// with the FPMCU's ROM/vendor bootloader (not EC RO).
// This is achieved by invoking flash_fp_mcu with the "--hello" option.
//
// This does not have any preconditions, other than being able to disable the
// hardware write protect. This is because the *communication* with the
// bootloader should always be possible, regardless of the firmware/software-wp
// status of the FPMCU. However, flash_fp_mcu can fail if the FPMCU's FW
// doesn't respond after being returned to the normal operating mode.
// See https://source.chromium.org/search?q=file:flash_fp_mcu for behavior.
func FpFlashFpMcuHello(ctx context.Context, s *testing.State) {
	servoSpec, _ := s.Var("servo")
	servop, err := servo.NewProxy(ctx, servoSpec, s.DUT().KeyFile(), s.DUT().KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer servop.Close(ctx)

	if err := servop.Servo().SetFWWPState(ctx, servo.FWWPStateOff); err != nil {
		s.Fatal("Failed to disable hardware write protection: ", err)
	}

	testing.ContextLog(ctx, "Running flash_fp_mcu --hello")
	cmd := s.DUT().Conn().Command("flash_fp_mcu", "--hello")
	if err := cmd.Run(ctx, ssh.DumpLogOnError); err != nil {
		s.Fatal("Error encountered when running flash_fp_mcu: ", err)
	}
}
