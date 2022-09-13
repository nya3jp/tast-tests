// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

type servoV4Flash struct {
	firmwareType string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         FlashUsingServoV4,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "System should support flashing ec/coreboot using Servo v4",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"servo"},
		ServiceDeps:  []string{"tast.cros.firmware.BiosService", "tast.cros.security.BootLockboxService"},
		Fixture:      fixture.NormalMode,
		Vars:         []string{"platform.firmwarePath"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{{
			Name: "ec",
			Val:  servoV4Flash{firmwareType: "ec"},
		}, {
			Name: "coreboot",
			Val:  servoV4Flash{firmwareType: "coreboot"},
		}},
	})
}

func FlashUsingServoV4(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	firmwareType := s.Param().(servoV4Flash).firmwareType

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}

	if isV4, err := h.Servo.IsServoV4(ctx); err != nil {
		s.Fatal("Failed to determine whether servo is v4: ", err)
	} else if !isV4 {
		s.Fatal("Servo must be v4")
	}

	pathtoFirmware := s.RequiredVar("platform.firmwarePath")

	versionBefore, err := firmwareVersion(ctx, h, firmwareType)
	if err != nil {
		s.Fatalf("Failed to determine %s version: %v", firmwareType, err)
	}

	board, err := h.Reporter.Board(ctx)
	if err != nil {
		s.Fatal("Failed to get board name: ", err)
	}

	var cmdFlash *testexec.Cmd
	if firmwareType == "ec" {
		cmdFlash = testexec.CommandContext(ctx, "flash_ec", "--board="+board, fmt.Sprintf("--image=%s", pathtoFirmware))
	} else {
		cmdFlash = testexec.CommandContext(ctx, "sudo", "flashrom", "-p", "raiden_debug_spi:target=AP", "-w", pathtoFirmware)
	}

	// Flashing the chip may fail due to hardware reasons. Allow |maxFlashAttempts|.
	attempt := 0
	maxFlashAttempts := 2
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		attempt++
		if attempt > maxFlashAttempts {
			return testing.PollBreak(errors.Errorf("failed to flash binary after %d attempts", maxFlashAttempts))
		}
		testing.ContextLogf(ctx, "Running command: %q", shutil.EscapeSlice(cmdFlash.Args))
		if err := cmdFlash.Run(testexec.DumpLogOnError); err != nil {
			testing.ContextLogf(ctx, "Flashing failed on attempt %d", attempt)
			return errors.Wrap(err, "failed to flash firmware")
		}
		return nil
	}, &testing.PollOptions{Interval: 5 * time.Second, Timeout: 4 * time.Minute}); err != nil {
		s.Fatalf("Failed to flash %s: %v", firmwareType, err)
	}

	s.Log("Waiting for DUT to reboot after flashing")
	if err := h.WaitConnect(ctx); err != nil {
		s.Fatal("Failed to wait for device to connect: ", err)
	}

	// Perform a Chrome login.
	s.Log("Login to Chrome")
	if err := powercontrol.ChromeOSLogin(ctx, h.DUT, s.RPCHint()); err != nil {
		s.Fatal("Failed to login to chrome: ", err)
	}

	versionAfter, err := firmwareVersion(ctx, h, firmwareType)
	if err != nil {
		s.Fatalf("Failed to determine %s version: %v", firmwareType, err)
	}

	if versionBefore == versionAfter {
		s.Fatalf("Failed: %s version is not updated, want %s, got %s ", firmwareType, versionAfter, versionBefore)
	}

}

// firmwareVersion returns the version of either EC or Coreboot.
func firmwareVersion(ctx context.Context, h *firmware.Helper, firmwareType string) (string, error) {
	switch firmwareType {
	case "ec":
		ec := firmware.NewECTool(h.DUT, firmware.ECToolNameMain)
		version, err := ec.Version(ctx)
		if err != nil {
			return "", errors.Wrap(err, "failed to determine EC version")
		}
		return version, nil
	case "coreboot":
		fwidOut, err := h.DUT.Conn().CommandContext(ctx, "crossystem", "fwid").Output()
		if err != nil {
			return "", errors.Wrap(err, "failed to check firmware version")
		}
		return strings.TrimSpace(string(fwidOut)), nil
	}
	return "", errors.New("failed to find firmware version")
}
