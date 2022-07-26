// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/firmware/reporters"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// Create enum to specify which tests need to be run
type wpTestRebootMethod int

const (
	rebootWithModeAwareReboot wpTestRebootMethod = iota
	rebootWithShutdownCmd
	rebootWithRebootCmd
	rebootWithPowerBtn
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WriteProtect,
		Desc:         "Verify enabling and disabling write protect works as expected",
		Contacts:     []string{"tij@google.com", "cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		SoftwareDeps: []string{"crossystem", "flashrom"},
		ServiceDeps:  []string{"tast.cros.firmware.BiosService"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Timeout:      45 * time.Minute,
		Params: []testing.Param{
			{
				Name:    "dev_mode_with_mode_aware_reboot",
				Fixture: fixture.DevMode,
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

const (
	shutdownTimeout time.Duration = 2 * time.Second
	rebootTimeout   time.Duration = 5 * time.Second
)

var (
	reFlashromSuccess = regexp.MustCompile(`SUCCESS`)
	reFlashromFail    = regexp.MustCompile(`FAILED`)
	reWPROfmap        = regexp.MustCompile(`WP_RO\s+(\d+)\s+(\d+)`)
	reROFRIDfmap      = regexp.MustCompile(`RO_FRID\s+(\d+)\s+(\d+)`)
)

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
	targetBIOS: pb.ImageSection_APWPROImageSection,
	targetEC:   pb.ImageSection_ECRWImageSection,
}

var tmpDirPath = filepath.Join("/", "mnt", "stateful_partition", fmt.Sprintf("flashrom_%d", time.Now().Unix()))

func WriteProtect(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	rebootMethod := s.Param().(wpTestRebootMethod)
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	cleanupContext := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Minute)
	defer cancel()

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}

	// Back up EC_RW.
	s.Log("Back up current EC_RW region")
	ecrwPath, err := h.BiosServiceClient.BackupImageSection(ctx, &pb.FWBackUpSection{
		Section:    pb.ImageSection_ECRWImageSection,
		Programmer: pb.Programmer_ECProgrammer,
	})
	if err != nil {
		s.Fatal("Failed to backup current EC_RW region: ", err)
	}
	s.Log("EC_RW region backup is stored at: ", ecrwPath.Path)
	defer func(ctx context.Context) {
		s.Log("Delete EC backup")
		if _, err := h.DUT.Conn().CommandContext(ctx, "rm", ecrwPath.Path).Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to delete ec backup: ", err)
		}
	}(cleanupContext)

	s.Log("Back up current AP WP_RO region")
	wproPath, err := h.BiosServiceClient.BackupImageSection(ctx, &pb.FWBackUpSection{
		Section:    pb.ImageSection_APWPROImageSection,
		Programmer: pb.Programmer_BIOSProgrammer,
	})
	if err != nil {
		s.Fatal("Failed to backup current WP_RO region: ", err)
	}
	s.Log("WP_RO region backup is stored at: ", wproPath.Path)
	defer func(ctx context.Context) {
		s.Log("Delete AP backup")
		if _, err := h.DUT.Conn().CommandContext(ctx, "rm", wproPath.Path).Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to delete ap backup: ", err)
		}
	}(cleanupContext)

	s.Log("Create temp dir in DUT")
	if _, err := h.DUT.Conn().CommandContext(ctx, "mkdir", "-p", tmpDirPath).Output(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	// Clean up temp directory.
	defer func(ctx context.Context) {
		s.Log("Delete temp dir and contained files from DUT")
		if _, err := h.DUT.Conn().CommandContext(ctx, "rm", "-r", tmpDirPath).Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to delete temp dir: ", err)
		}
	}(cleanupContext)

	// Restore EC fw.
	defer func(ctx context.Context) {
		s.Log("Reset write protect to false")
		if err := setWriteProtect(ctx, h, targetEC, false); err != nil {
			s.Fatal("Failed to set ec write protect state: ", err)
		}
		if err := setWriteProtect(ctx, h, targetBIOS, false); err != nil {
			s.Fatal("Failed to set ap write protect state: ", err)
		}
		if err := h.RequireBiosServiceClient(ctx); err != nil {
			s.Fatal("Requiring BiosServiceClient: ", err)
		}
		s.Log("Restore EC_RW region with backup from: ", ecrwPath.Path)
		if _, err := h.BiosServiceClient.RestoreImageSection(ctx, ecrwPath); err != nil {
			s.Fatal("Failed to restore EC_RW image: ", err)
		}
		s.Log("Restore AP WP_RO region with backup from: ", wproPath.Path)
		if _, err := h.BiosServiceClient.RestoreImageSection(ctx, wproPath); err != nil {
			s.Fatal("Failed to restore WP_RO image: ", err)
		}
	}(cleanupContext)

	// This call takes ~= 10 mins to complete.
	s.Log("Test wp state over reboot on target EC")
	if err := testWPOverReboot(ctx, h, targetEC, rebootMethod); err != nil {
		s.Fatal("Failed to preserve wp state over reboots: ", err)
	}

	// This call takes ~= 10 mins to complete.
	s.Log("Test wp state over reboot on target BIOS")
	if err := testWPOverReboot(ctx, h, targetBIOS, rebootMethod); err != nil {
		s.Fatal("Failed to preserve wp state over reboots: ", err)
	}

	// This call takes ~= 2 mins to complete.
	s.Log("Test flashrom read/write with wp on target EC")
	if err := testReadWrite(ctx, h, targetEC); err != nil {
		s.Fatal("Read/write behaved unexpectedly: ", err)
	}

	// This call takes ~= 2 mins to complete.
	s.Log("Test flashrom read/write with wp on target BIOS")
	if err := testReadWrite(ctx, h, targetBIOS); err != nil {
		s.Fatal("Read/write behaved unexpectedly: ", err)
	}

}

func testWPOverReboot(ctx context.Context, h *firmware.Helper, target wpTarget, testRebootMethod wpTestRebootMethod) error {
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
	testing.ContextLog(ctx, "Expect write protect state to be enabled")
	if err := checkCrossystem(ctx, h, 1); err != nil {
		return errors.Wrap(err, "failed to check crossystem")
	}
	testing.ContextLog(ctx, "Disable Write Protect")
	if err := setWriteProtect(ctx, h, targetEC, false); err != nil {
		return errors.Wrap(err, "failed to disable FW write protect state")
	}
	testing.ContextLog(ctx, "Reboot DUT using ", rebootMethod)
	if err := rebootFunc(ctx, h); err != nil {
		return errors.Wrapf(err, "failed to reboot with %q", rebootMethod)
	}
	testing.ContextLog(ctx, "Expect write protect state to be disabled")
	if err := checkCrossystem(ctx, h, 0); err != nil {
		return errors.Wrap(err, "failed to check crossystem")
	}
	return nil
}

func performRebootWithRebootCmd(ctx context.Context, h *firmware.Helper) error {
	cmd := h.DUT.Conn().CommandContext(ctx, "reboot")
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to reboot DUT")
	}

	if err := h.RequireConfig(ctx); err != nil {
		return errors.Wrap(err, "failed to require configs")
	}

	testing.ContextLog(ctx, "Sleep, wait for DUT to reboot")
	if err := testing.Sleep(ctx, 2*h.Config.Shutdown); err != nil {
		return errors.Wrapf(err, "failed to sleep for %s after initiating reboot", 2*h.Config.Shutdown)
	}

	testing.ContextLog(ctx, "Check for S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}
	return h.WaitConnect(ctx)
}

func performRebootWithShutdownCmd(ctx context.Context, h *firmware.Helper) error {
	cmd := h.DUT.Conn().CommandContext(ctx, "/sbin/shutdown", "-P", "now")
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to shut down DUT")
	}

	if err := h.RequireConfig(ctx); err != nil {
		return errors.Wrap(err, "failed to require configs")
	}

	testing.ContextLog(ctx, "Sleep, wait for DUT to shutdown")
	if err := testing.Sleep(ctx, h.Config.Shutdown); err != nil {
		return errors.Wrapf(err, "failed to sleep for %s after initiating shutdown", h.Config.Shutdown)
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
	return h.WaitConnect(ctx)
}

func performRebootWithPowerBtn(ctx context.Context, h *firmware.Helper) error {
	if err := h.RequireConfig(ctx); err != nil {
		return errors.Wrap(err, "failed to require configs")
	}

	testing.ContextLog(ctx, "Power DUT off with long press of the power button")
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOff)); err != nil {
		return errors.Wrap(err, "failed to power on DUT with short press of the power button")
	}

	testing.ContextLog(ctx, "Sleep, wait for DUT to power off")
	if err := testing.Sleep(ctx, h.Config.Shutdown); err != nil {
		return errors.Wrapf(err, "failed to sleep for %s after pressing powerbutton", rebootTimeout)
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
	return h.WaitConnect(ctx)
}

func testReadWrite(ctx context.Context, h *firmware.Helper, target wpTarget) error {

	testing.ContextLog(ctx, "Enable Write Protect")
	if err := setWriteProtect(ctx, h, target, true); err != nil {
		return errors.Wrap(err, "failed to set FW write protect state")
	}

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		return errors.Wrap(err, "failed to connect to the bios service on the DUT")
	}

	testing.ContextLog(ctx, "Read current fw image")
	roBefore, err := h.BiosServiceClient.BackupImageSection(ctx, &pb.FWBackUpSection{
		Section:    wpTargetToRegion[target],
		Programmer: wpTargetToProg[target],
	})
	if err != nil {
		return errors.Wrap(err, "failed to save current fw image")
	}

	testing.ContextLog(ctx, "Attempt to overwrite fw with write protect enabled")
	roTest, err := h.BiosServiceClient.CorruptFWSection(ctx, &pb.CorruptSection{
		Section:    wpTargetToRegion[target],
		Programmer: wpTargetToProg[target],
	})
	if err == nil {
		errors.Wrap(err, "expected flashrom write to fail since wp is enabled")
	}

	testing.ContextLog(ctx, "Read fw, make sure write didn't succeed with wp enabled")
	roAfter, err := h.BiosServiceClient.BackupImageSection(ctx, &pb.FWBackUpSection{
		Section:    wpTargetToRegion[target],
		Programmer: wpTargetToProg[target],
	})
	if err != nil {
		return errors.Wrap(err, "failed to save current fw image")
	}

	if out, err := h.DUT.Conn().CommandContext(ctx, "cmp", roBefore.Path, roAfter.Path).CombinedOutput(ssh.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to compare %q and %q using 'cmp'", roBefore.Path, roAfter.Path)
	} else if string(out) != "" {
		return errors.Wrapf(err, "files %q and %q were not identical, so either write protect or read failed", roBefore.Path, roAfter.Path)
	}

	testing.ContextLog(ctx, "Disable Write Protect")
	if err := setWriteProtect(ctx, h, target, false); err != nil {
		return errors.Wrap(err, "failed to set FW write protect state")
	}

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		return errors.Wrap(err, "failed to connect to the bios service on the DUT")
	}

	testing.ContextLog(ctx, "Attempt to overwrite fw with write protect disabled")
	roTest, err = h.BiosServiceClient.CorruptFWSection(ctx, &pb.CorruptSection{
		Section:    wpTargetToRegion[target],
		Programmer: wpTargetToProg[target],
	})
	if err != nil {
		errors.Wrap(err, "failed to corrupt current fw image with write protect disabled")
	}

	testing.ContextLog(ctx, "Read fw, make sure write succeeded with wp disabled")
	roAfter, err = h.BiosServiceClient.BackupImageSection(ctx, &pb.FWBackUpSection{
		Section:    wpTargetToRegion[target],
		Programmer: wpTargetToProg[target],
	})
	if err != nil {
		return errors.Wrap(err, "failed to save current fw image")
	}

	if out, err := h.DUT.Conn().CommandContext(ctx, "cmp", roTest.Path, roAfter.Path).CombinedOutput(ssh.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to compare %q and %q using 'cmp'", roTest.Path, roAfter.Path)
	} else if string(out) != "" {
		return errors.Errorf("Files %q and %q were not identical", roTest.Path, roAfter.Path)
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
		if err := h.Servo.RunECCommand(ctx, "flashwp disable"); err != nil {
			return errors.Wrap(err, "failed to disable flashwp")
		}
		if err := h.Servo.SetFWWPState(ctx, servo.FWWPStateOff); err != nil {
			return errors.Wrap(err, "failed to disable firmware write protect")
		}

		if err := h.RequireBiosServiceClient(ctx); err != nil {
			return errors.Wrap(err, "failed to connect to the bios service on the DUT")
		}
		if _, err := h.BiosServiceClient.SetAPSoftwareWriteProtect(ctx, &pb.WPRequest{
			Enable: enable,
		}); err != nil {
			return errors.Wrapf(err, "failed to %s AP write protection", enableStr)
		}

		if err := h.Servo.SetFWWPState(ctx, fwwpState); err != nil {
			return errors.Wrapf(err, "failed to %s firmware write protect", enableStr)
		}
		if err := h.Servo.RunECCommand(ctx, fmt.Sprintf("flashwp %s", enableStr)); err != nil {
			return errors.Wrap(err, "failed to disable flashwp")
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

	return performModeAwareReboot(ctx, h)
}

func performModeAwareReboot(ctx context.Context, h *firmware.Helper) error {
	// Create new mode switcher every time to prevent nil pointer errors.
	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		return errors.Wrap(err, "failed to create mode switcher")
	}
	testing.ContextLog(ctx, "Performing mode aware reboot")
	return ms.ModeAwareReboot(ctx, firmware.WarmReset)
}

func checkCrossystem(ctx context.Context, h *firmware.Helper, expectedWpsw int) error {
	testing.ContextLog(ctx, "Check crossystem for write protect state param")
	r := reporters.New(h.DUT)
	currWpswStr, err := r.CrossystemParam(ctx, reporters.CrossystemParamWpswCur)
	if err != nil {
		return errors.Wrap(err, "failed to get crossystem wpsw value value")
	}
	currWpsw, err := strconv.Atoi(currWpswStr)
	if err != nil {
		return errors.Wrap(err, "failed to convert crossystem wpsw value to integer value")
	}
	testing.ContextLogf(ctx, "Current write protect state: %v, Expected state: %v", currWpsw, expectedWpsw)
	if currWpsw != expectedWpsw {
		return errors.Errorf("expected WP state to %v, is actually %v", expectedWpsw, currWpsw)
	}
	return nil
}
