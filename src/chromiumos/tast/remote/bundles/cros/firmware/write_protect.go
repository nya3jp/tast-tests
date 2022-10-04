// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	fwUtils "chromiumos/tast/remote/bundles/cros/firmware/utils"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// Create enum to specify which tests need to be run
type wpTestType int

const (
	rebootWithModeAwareReboot wpTestType = iota
	rebootWithShutdownCmd
	rebootWithRebootCmd
	rebootWithPowerBtn
	readWriteTest
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     WriteProtect,
		Desc:     "Verify enabling and disabling write protect works as expected",
		Contacts: []string{"tij@google.com", "cros-fw-engprod@google.com"},
		// Disabling from running pending fixes.
		Attr:         []string{},
		SoftwareDeps: []string{"crossystem", "flashrom"},
		ServiceDeps:  []string{"tast.cros.firmware.BiosService"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Timeout:      45 * time.Minute,
		Params: []testing.Param{
			{
				Name:    "dev_mode_read_write",
				Fixture: fixture.DevModeGBB,
				Val:     readWriteTest,
			},
			{
				Name:    "normal_mode_read_write",
				Fixture: fixture.NormalMode,
				Val:     readWriteTest,
			},
			{
				Name:    "dev_mode_with_mode_aware_reboot",
				Fixture: fixture.DevModeGBB,
				Val:     rebootWithModeAwareReboot,
			},
			{
				Name:    "normal_mode_with_mode_aware_reboot",
				Fixture: fixture.NormalMode,
				Val:     rebootWithModeAwareReboot,
			},
			{
				Name:    "normal_mode_with_shutdown_cmd",
				Fixture: fixture.NormalMode,
				Val:     rebootWithShutdownCmd,
			},
			{
				Name:    "normal_mode_with_reboot_cmd",
				Fixture: fixture.NormalMode,
				Val:     rebootWithRebootCmd,
			},
			{
				Name:    "normal_mode_with_power_btn",
				Fixture: fixture.NormalMode,
				Val:     rebootWithPowerBtn,
			},
		},
	})
}

type wpTarget string

const (
	targetBIOS wpTarget = "bios"
	targetEC   wpTarget = "ec"
)

var wpTargetToProg = map[wpTarget]pb.Programmer{
	targetBIOS: pb.Programmer_BIOSProgrammer,
	targetEC:   pb.Programmer_ECProgrammer,
}

var wpTargetToRegion = map[wpTarget]pb.ImageSection{
	targetBIOS: pb.ImageSection_FWSignBImageSection,
	targetEC:   pb.ImageSection_ECRWImageSection,
}

var wpTmpDirPath = filepath.Join("/", "mnt", "stateful_partition", fmt.Sprintf("flashrom_%d", time.Now().Unix()))

func WriteProtect(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	testType := s.Param().(wpTestType)

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	cleanupContext := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Minute)
	defer cancel()
	s.Log("Create temp dir in DUT")
	if _, err := h.DUT.Conn().CommandContext(ctx, "mkdir", "-p", wpTmpDirPath).Output(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	// Clean up temp directory which contains all the image back ups.
	defer func(ctx context.Context) {
		s.Log("Delete temp dir and contained files from DUT")
		if _, err := h.DUT.Conn().CommandContext(ctx, "rm", "-r", wpTmpDirPath).Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to delete temp dir: ", err)
		}
	}(cleanupContext)

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}

	// Back up EC_RW.
	s.Logf("Back up current EC %v region", wpTargetToRegion[targetEC])
	ecPath, err := h.BiosServiceClient.BackupImageSection(ctx, &pb.FWSectionInfo{
		Section:    wpTargetToRegion[targetEC],
		Programmer: wpTargetToProg[targetEC],
		Path:       wpTmpDirPath,
	})
	if err != nil {
		s.Fatalf("Failed to backup current EC %v region: %v", wpTargetToRegion[targetEC], err)
	}
	s.Logf("%v region backup is stored at: %v", wpTargetToRegion[targetEC], ecPath.Path)

	s.Logf("Back up current AP %v region", wpTargetToRegion[targetBIOS])
	apPath, err := h.BiosServiceClient.BackupImageSection(ctx, &pb.FWSectionInfo{
		Section:    wpTargetToRegion[targetBIOS],
		Programmer: wpTargetToProg[targetBIOS],
		Path:       wpTmpDirPath,
	})
	if err != nil {
		s.Fatalf("Failed to backup current %v region: %v", wpTargetToRegion[targetBIOS], err)
	}
	s.Logf("%v region backup is stored at: %s", wpTargetToRegion[targetBIOS], apPath.Path)
	// Restore fw.
	defer func(ctx context.Context) {
		if err := h.RequireBiosServiceClient(ctx); err != nil {
			s.Fatal("Requiring BiosServiceClient: ", err)
		}
		s.Log("Reset write protect to false")
		if err := setWriteProtect(ctx, h, targetEC, false); err != nil {
			s.Fatal("Failed to set ec write protect state: ", err)
		}
		if err := setWriteProtect(ctx, h, targetBIOS, false); err != nil {
			s.Fatal("Failed to set ap write protect state: ", err)
		}
		s.Logf("Restore EC %v region with backup from: %v", wpTargetToRegion[targetEC], ecPath.Path)
		if _, err := h.BiosServiceClient.RestoreImageSection(ctx, ecPath); err != nil {
			s.Fatal("Failed to restore EC_RW image: ", err)
		}
		s.Logf("Restore AP %v region with backup from: %v", wpTargetToRegion[targetBIOS], apPath.Path)
		if _, err := h.BiosServiceClient.RestoreImageSection(ctx, apPath); err != nil {
			s.Fatalf("Failed to restore %v image: %v", wpTargetToRegion[targetBIOS], err)
		}
	}(cleanupContext)

	switch testType {
	case readWriteTest: // Test flashrom read/write with/without WP.
		s.Log("Test flashrom read/write with wp on target EC")
		if err := testReadWrite(ctx, h, targetEC); err != nil {
			s.Fatal("Read/write behaved unexpectedly: ", err)
		}

		s.Log("Test flashrom read/write with wp on target BIOS")
		if err := testReadWrite(ctx, h, targetBIOS); err != nil {
			s.Fatal("Read/write behaved unexpectedly: ", err)
		}
	default: // Preserve WP status over reboot test.
		s.Log("Test wp state over reboot on target EC")
		if err := testWPOverReboot(ctx, h, targetEC, testType); err != nil {
			s.Fatal("Failed to preserve wp state over reboots: ", err)
		}

		s.Log("Test wp state over reboot on target BIOS")
		if err := testWPOverReboot(ctx, h, targetBIOS, testType); err != nil {
			s.Fatal("Failed to preserve wp state over reboots: ", err)
		}
	}

}

func testReadWrite(ctx context.Context, h *firmware.Helper, target wpTarget) error {

	testing.ContextLog(ctx, "Read current fw image")
	roBefore, err := h.BiosServiceClient.BackupImageSection(ctx, &pb.FWSectionInfo{
		Section:    wpTargetToRegion[target],
		Programmer: wpTargetToProg[target],
		Path:       wpTmpDirPath,
	})
	if err != nil {
		return errors.Wrap(err, "failed to save current fw image")
	}

	testing.ContextLog(ctx, "Enable Write Protect")
	if err := setWriteProtect(ctx, h, target, true); err != nil {
		return errors.Wrap(err, "failed to set FW write protect state")
	}

	testing.ContextLog(ctx, "Attempt to overwrite fw with write protect enabled")
	if _, err := h.BiosServiceClient.CorruptFWSection(ctx, &pb.FWSectionInfo{
		Section:    wpTargetToRegion[target],
		Programmer: wpTargetToProg[target],
	}); err == nil {
		errors.Wrap(err, "expected flashrom write to fail since wp is enabled")
	} else {
		testing.ContextLog(ctx, "Flashrom output: ", err)
	}

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		return errors.Wrap(err, "failed to connect to the bios service on the DUT")
	}

	testing.ContextLog(ctx, "Read fw, make sure write didn't succeed with wp enabled")
	roAfter, err := h.BiosServiceClient.BackupImageSection(ctx, &pb.FWSectionInfo{
		Section:    wpTargetToRegion[target],
		Programmer: wpTargetToProg[target],
		Path:       wpTmpDirPath,
	})
	if err != nil {
		return errors.Wrap(err, "failed to save current fw image")
	}

	// Fw attempted to be overwritten with roTest with wp enabled, verify it's still original roBefore fw.
	if out, err := h.DUT.Conn().CommandContext(ctx, "cmp", roBefore.Path, roAfter.Path).CombinedOutput(ssh.DumpLogOnError); err != nil {
		// Cmp error code 0 == files match, 1 == files differ, 2 == error in running cmp.
		if errCode, ok := testexec.ExitCode(err); !ok || errCode == 2 {
			return errors.Wrapf(err, "failed to compare %q and %q using 'cmp'", roBefore.Path, roAfter.Path)
		} else if errCode == 1 {
			return errors.Wrapf(err, "files %q and %q were not identical, so either write protect or read failed: %s", roBefore.Path, roAfter.Path, string(out))
		}
	}

	testing.ContextLog(ctx, "Disable Write Protect")
	if err := setWriteProtect(ctx, h, target, false); err != nil {
		return errors.Wrap(err, "failed to set FW write protect state")
	}

	testing.ContextLog(ctx, "Restore original fw backup from: ", roBefore.Path)
	if _, err := h.BiosServiceClient.RestoreImageSection(ctx, roBefore); err != nil {
		return errors.Wrap(err, "failed to restore fw image")
	}

	return nil
}

func setWriteProtect(ctx context.Context, h *firmware.Helper, target wpTarget, enable bool) error {
	enableStr := "enable"
	fwwpState := servo.FWWPStateOn
	if !enable {
		enableStr = "disable"
		fwwpState = servo.FWWPStateOff
	}

	if target == targetBIOS {
		// Disable hardware wp for now so flashrom cmd can run.
		if err := h.Servo.SetFWWPState(ctx, servo.FWWPStateOff); err != nil {
			return errors.Wrap(err, "failed to disable firmware write protect")
		}

		if _, err := h.BiosServiceClient.SetAPSoftwareWriteProtect(ctx, &pb.WPRequest{Enable: enable}); err != nil {
			return errors.Wrapf(err, "failed to %s AP write protection", enableStr)
		}

		if err := h.Servo.SetFWWPState(ctx, fwwpState); err != nil {
			return errors.Wrapf(err, "failed to %s firmware write protect", enableStr)
		}

	} else {
		// Enable software wp before hardware wp if enabling.
		if enable {
			if err := h.Servo.RunECCommand(ctx, "flashwp enable"); err != nil {
				return errors.Wrap(err, "failed to enable flashwp")
			}
		}

		if err := h.Servo.SetFWWPState(ctx, fwwpState); err != nil {
			return errors.Wrapf(err, "failed to %s firmware write protect", enableStr)
		}

		// Disable software wp after hardware wp so its allowed.
		if !enable {
			if err := h.Servo.RunECCommand(ctx, "flashwp disable"); err != nil {
				return errors.Wrap(err, "failed to disable flashwp")
			}
		}
	}

	if err := performModeAwareReboot(ctx, h); err != nil {
		return errors.Wrap(err, "failed to perform mode aware reboot")
	}
	if err := h.RequireBiosServiceClient(ctx); err != nil {
		return errors.Wrap(err, "failed to connect to the bios service on the DUT")
	}
	return nil
}

func testWPOverReboot(ctx context.Context, h *firmware.Helper, target wpTarget, testRebootMethod wpTestType) error {
	var rebootMethod string
	var rebootFunc func(context.Context, *firmware.Helper) error
	switch testRebootMethod {
	case rebootWithModeAwareReboot:
		rebootMethod = "mode aware reboot"
		rebootFunc = performModeAwareReboot
	case rebootWithShutdownCmd:
		rebootMethod = "shutdown cmd"
		rebootFunc = performRebootWithShutdownCmd
	case rebootWithRebootCmd:
		rebootMethod = "reboot cmd"
		rebootFunc = performRebootWithRebootCmd
	case rebootWithPowerBtn:
		rebootMethod = "power button press"
		rebootFunc = performRebootWithPowerBtn
	}

	testing.ContextLog(ctx, "Enable Write Protect")
	if err := setWriteProtect(ctx, h, target, true); err != nil {
		return errors.Wrap(err, "failed to enable FW write protect state")
	}
	testing.ContextLog(ctx, "Reboot DUT using ", rebootMethod)
	if err := rebootFunc(ctx, h); err != nil {
		return errors.Wrapf(err, "failed to reboot with %q", rebootMethod)
	}

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		return errors.Wrap(err, "failed to connect to the bios service on the DUT")
	}

	testing.ContextLog(ctx, "Expect write protect state to be enabled")
	if err := fwUtils.CheckCrossystemWPSW(ctx, h, 1); err != nil {
		return errors.Wrap(err, "failed to check crossystem")
	}

	if target == targetEC {
		// This covers the firmware_ECSystemLocked test.
		testing.ContextLog(ctx, "Expect sysinfo to show ec is locked")
		if out, err := h.Servo.RunECCommandGetOutput(ctx, "sysinfo", []string{`Flags:\s*(\S+)\s`}); err != nil {
			return errors.Wrap(err, "failed to get sysinfo")
		} else if out == nil || len(out[0]) < 2 {
			return errors.Errorf("failed to parse sysinfo correctly, got: %v", out)
		} else if out[0][1] != "locked" {
			return errors.Wrapf(err, "expected flags to show locked, got %q instead:", out[0][1])
		}
	}

	testing.ContextLog(ctx, "Disable Write Protect")
	if err := setWriteProtect(ctx, h, targetEC, false); err != nil {
		return errors.Wrap(err, "failed to disable FW write protect state")
	}

	testing.ContextLog(ctx, "Reboot DUT using ", rebootMethod)
	if err := rebootFunc(ctx, h); err != nil {
		return errors.Wrapf(err, "failed to reboot with %q", rebootMethod)
	}

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		return errors.Wrap(err, "failed to connect to the bios service on the DUT")
	}

	testing.ContextLog(ctx, "Expect write protect state to be disabled")
	if err := fwUtils.CheckCrossystemWPSW(ctx, h, 0); err != nil {
		return errors.Wrap(err, "failed to check crossystem")
	}
	return nil
}

func performRebootWithRebootCmd(ctx context.Context, h *firmware.Helper) error {
	cmd := h.DUT.Conn().CommandContext(ctx, "reboot")
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to reboot DUT")
	}

	// Expect shutdown and then subsequent boot.
	testing.ContextLog(ctx, "Check for G3 or S5 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3", "S5"); err != nil {
		return errors.Wrap(err, "failed to get G3 or S5 powerstate")
	}

	testing.ContextLog(ctx, "Check for S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	return nil
}

func performRebootWithShutdownCmd(ctx context.Context, h *firmware.Helper) error {
	cmd := h.DUT.Conn().CommandContext(ctx, "/sbin/shutdown", "-P", "now")
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to shut down DUT")
	}

	if err := h.RequireConfig(ctx); err != nil {
		return errors.Wrap(err, "failed to require configs")
	}

	testing.ContextLog(ctx, "Check for G3 or S5 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3", "S5"); err != nil {
		return errors.Wrap(err, "failed to get G3 or S5 powerstate")
	}

	testing.ContextLog(ctx, "Power DUT back on with short press of the power button")
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOn)); err != nil {
		return errors.Wrap(err, "failed to power on DUT with short press of the power button")
	}

	testing.ContextLog(ctx, "Check for S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	return nil
}

func performRebootWithPowerBtn(ctx context.Context, h *firmware.Helper) error {
	if err := h.RequireConfig(ctx); err != nil {
		return errors.Wrap(err, "failed to require configs")
	}

	testing.ContextLog(ctx, "Power DUT off with long press of the power button")
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOff)); err != nil {
		return errors.Wrap(err, "failed to power on DUT with short press of the power button")
	}

	testing.ContextLog(ctx, "Check for G3 or S5 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3", "S5"); err != nil {
		return errors.Wrap(err, "failed to get G3 or S5 powerstate")
	}

	testing.ContextLog(ctx, "Power DUT back on with short press of the power button")
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOn)); err != nil {
		return errors.Wrap(err, "failed to power on DUT with short press of the power button")
	}

	testing.ContextLog(ctx, "Check for S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}
	return nil
}

func performModeAwareReboot(ctx context.Context, h *firmware.Helper) error {
	// Create new mode switcher every time to prevent nil pointer errors.
	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		return errors.Wrap(err, "failed to create mode switcher")
	}
	testing.ContextLog(ctx, "Performing mode aware reboot")

	return ms.ModeAwareReboot(ctx, firmware.ColdReset)
}
