// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/servo"
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
		// On hatch+bloonchipper(Dratini) flash_fp_mcu --hello takes about
		// 4 seconds and the full test with reboot takes about 30 seconds.
		// Given flash_fp_mcu can run into scenarios were it needs to retry a
		// few times, we delegate a minute for flash_fp_mcu and a minute for
		// the reboot cleanup.
		Timeout:      2 * time.Minute,
		SoftwareDeps: []string{"biometrics_daemon"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
		VarDeps:      []string{"servo"},
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
	defer func(ctx context.Context) {
		// The flash_fp_mcu script will reboot the FPMCU. This causes the FPMCU
		// to lose the at-boot TPM seed (provided by bio_crypto_init). The only
		// way to restore the correct TPM seed is to reboot the DUT.
		//
		// This is also needed for Zork, where the cros-ec driver cannot be
		// rebound without a DUT reboot.
		testing.ContextLog(ctx, "Rebooting DUT to restore TPM seed")
		if err := s.DUT().Reboot(ctx); err != nil {
			s.Fatal("Failed to reboot device on test exit: ", err)
		}
	}(ctx)

	// Reduce overall ctx time for servo and flash_fp_mcu by a minute to
	// reserve the remaining minute for the above cleanup.
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

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
	out, err := cmd.CombinedOutput(ctx)
	s.Log("flash_fp_mcu output:", "\n", string(out))
	if err != nil {
		s.Fatal("Error encountered when running flash_fp_mcu: ", err)
	}
}
