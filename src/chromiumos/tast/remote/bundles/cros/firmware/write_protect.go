// Copyright 2021 The ChromiumOS Authors
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
	fwUtils "chromiumos/tast/remote/bundles/cros/firmware/utils"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
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
		Func:     WriteProtect,
		Desc:     "Verify enabling and disabling write protect works as expected",
		Contacts: []string{"tij@google.com", "cros-fw-engprod@google.com"},
		// Disabling from running pending fixes.
		Attr:         []string{},
		SoftwareDeps: []string{"crossystem", "flashrom"},
		ServiceDeps:  []string{"tast.cros.firmware.BiosService"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Timeout:      20 * time.Minute,
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

var flashromTargets = map[wpTarget]string{
	targetBIOS: "host",
	targetEC:   "ec",
}

var tmpDirPath = filepath.Join("/", "mnt", "stateful_partition", fmt.Sprintf("flashrom_%d", time.Now().Unix()))

func WriteProtect(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	rebootMethod := s.Param().(wpTestRebootMethod)
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}

	// Back up EC_RW.
	s.Log("Back up current EC_RW region")
	ecrwPath, err := h.BiosServiceClient.BackupImageSection(ctx, &pb.FWSectionInfo{Section: pb.ImageSection_ECRWImageSection, Programmer: pb.Programmer_ECProgrammer})
	if err != nil {
		s.Fatal("Failed to backup current EC_RW region: ", err)
	}
	s.Log("EC_RW region backup is stored at: ", ecrwPath.Path)

	s.Log("Create temp dir in DUT")
	if _, err = h.DUT.Conn().CommandContext(ctx, "mkdir", "-p", tmpDirPath).Output(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}

	cleanupContext := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Minute)
	defer cancel()
	defer func(ctx context.Context) {
		s.Log("Reset write protect to false")
		if err := setWriteProtect(ctx, h, targetEC, false); err != nil {
			s.Fatal("Failed to set FW write protect state: ", err)
		}

		// Require again here since reboots in test cause nil pointer errors otherwise.
		if err := h.RequireBiosServiceClient(ctx); err != nil {
			s.Fatal("Requiring BiosServiceClient: ", err)
		}

		// Restore EC_RW.
		s.Log("Restore EC_RW region with backup from: ", ecrwPath.Path)
		if _, err := h.BiosServiceClient.RestoreImageSection(ctx, ecrwPath); err != nil {
			s.Fatal("Failed to restore EC_RW image: ", err)
		}

		s.Log("Delete EC backup")
		if _, err := h.DUT.Conn().CommandContext(ctx, "rm", ecrwPath.Path).Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to delete ec backup: ", err)
		}
		// Clean up temp directory.
		s.Log("Delete temp dir and contained files from DUT")
		if _, err := h.DUT.Conn().CommandContext(ctx, "rm", "-r", tmpDirPath).Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to delete temp dir: ", err)
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
		rebootMethod = "mode aware reboot"
		rebootFunc = performRebootWithShutdownCmd
	case rebootWithRebootCmd:
		rebootMethod = "mode aware reboot"
		rebootFunc = performRebootWithRebootCmd
	case rebootWithPowerBtn:
		rebootMethod = "mode aware reboot"
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
	if err := fwUtils.CheckCrossystemWPSW(ctx, h, 1); err != nil {
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

	testing.ContextLog(ctx, "Sleep, wait for DUT to reboot")
	if err := testing.Sleep(ctx, rebootTimeout); err != nil {
		return errors.Wrapf(err, "failed to sleep for %s after initiating reboot", rebootTimeout)
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

	testing.ContextLog(ctx, "Sleep, wait for DUT to shutdown")
	if err := testing.Sleep(ctx, shutdownTimeout); err != nil {
		return errors.Wrapf(err, "failed to sleep for %s after initiating shutdown", shutdownTimeout)
	}

	testing.ContextLog(ctx, "Check for G3 or S5 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3", "S5"); err != nil {
		return errors.Wrap(err, "failed to get G3 or S5 powerstate")
	}

	testing.ContextLog(ctx, "Power DUT back on with short press of the power button")
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
		return errors.Wrap(err, "failed to power on DUT with short press of the power button")
	}

	testing.ContextLog(ctx, "Check for S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}
	return h.WaitConnect(ctx)
}

func performRebootWithPowerBtn(ctx context.Context, h *firmware.Helper) error {
	testing.ContextLog(ctx, "Power DUT off with long press of the power button")
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurLongPress); err != nil {
		return errors.Wrap(err, "failed to power on DUT with short press of the power button")
	}

	testing.ContextLog(ctx, "Sleep, wait for DUT to power off")
	if err := testing.Sleep(ctx, rebootTimeout); err != nil {
		return errors.Wrapf(err, "failed to sleep for %s after pressing powerbutton", rebootTimeout)
	}

	testing.ContextLog(ctx, "Check for G3 or S5 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3", "S5"); err != nil {
		return errors.Wrap(err, "failed to get G3 or S5 powerstate")
	}

	testing.ContextLog(ctx, "Power DUT back on with short press of the power button")
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
		return errors.Wrap(err, "failed to power on DUT with short press of the power button")
	}

	testing.ContextLog(ctx, "Check for S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}
	return h.WaitConnect(ctx)
}

func testReadWrite(ctx context.Context, h *firmware.Helper, target wpTarget) error {
	// Current firmware image as read from flash.
	roBefore := filepath.Join(tmpDirPath, fmt.Sprintf("%s_ro_before.bin", target))
	// Current firmware image with modification to test writing.
	roTest := filepath.Join(tmpDirPath, fmt.Sprintf("%s_ro_test.bin", target))
	// Firmware as read after writing flash.
	roAfter := filepath.Join(tmpDirPath, fmt.Sprintf("%s_ro_after.bin", target))

	testing.ContextLog(ctx, "Enable Write Protect")
	if err := setWriteProtect(ctx, h, target, true); err != nil {
		return errors.Wrap(err, "failed to set FW write protect state")
	}

	testing.ContextLogf(ctx, "Save current fw image to %q in DUT", roBefore)
	out, err := h.DUT.Conn().CommandContext(ctx, "flashrom", "-p", flashromTargets[target], "-r", "-i", fmt.Sprintf("WP_RO:%s", roBefore)).CombinedOutput(ssh.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to run command flashrom")
	} else if match := reFlashromSuccess.FindSubmatch(out); match == nil {
		return errors.Errorf("flashrom did not produce sucess message: %s", string(out))
	}

	testing.ContextLog(ctx, "Checking fmap")
	out, err = h.DUT.Conn().CommandContext(ctx, "dump_fmap", "-p", roBefore).CombinedOutput(ssh.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to dump fmap")
	}

	// Format for the dumped fmap is "Name offset size".
	wproMatch := reWPROfmap.FindSubmatch(out)
	if wproMatch == nil {
		return errors.New("didn't find WP_RO in fmap")
	}
	wproOffset, err := strconv.Atoi(string(wproMatch[1]))
	if err != nil {
		return errors.Wrap(err, "failed to get WP_RO offset as integer")
	}

	if rofridMatch := reROFRIDfmap.FindSubmatch(out); rofridMatch != nil {
		rofridOffset, err := strconv.Atoi(string(rofridMatch[1]))
		if err != nil {
			return errors.Wrap(err, "failed to get offset int for RO_FRID")
		}

		rofridSize, err := strconv.Atoi(string(rofridMatch[2]))
		if err != nil {
			return errors.Wrap(err, "failed to get size int for RO_FRID")
		}

		if _, err := h.DUT.Conn().CommandContext(ctx, "cp", roBefore, roTest).CombinedOutput(ssh.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to copy %q to %q", roBefore, roTest)
		}

		dd1Args := []string{
			fmt.Sprintf("if=%s", roTest), "bs=1",
			fmt.Sprintf("count=%d", rofridSize),
			fmt.Sprintf("skip=%d", rofridOffset-wproOffset),
		}
		trArgs := []string{`"[a-zA-Z]"`, `"[A-Za-z]"`}
		dd2Args := []string{
			fmt.Sprintf("of=%s", roTest), "bs=1",
			fmt.Sprintf("count=%d", rofridSize),
			fmt.Sprintf("seek=%d", rofridOffset-wproOffset), "conv=notrunc",
		}
		dd1Cmd := h.DUT.Conn().CommandContext(ctx, "dd", dd1Args...)
		trCmd := h.DUT.Conn().CommandContext(ctx, "tr", trArgs...)
		dd2Cmd := h.DUT.Conn().CommandContext(ctx, "dd", dd2Args...)
		trCmd.Stdin, _ = dd1Cmd.StdoutPipe()
		dd2Cmd.Stdin, _ = trCmd.StdoutPipe()
		if err := dd2Cmd.Start(); err != nil {
			return errors.Wrap(err, "failed to start second dd cmd")
		}
		if err := trCmd.Start(); err != nil {
			return errors.Wrap(err, "failed to start tr cmd")
		}
		if err := dd1Cmd.Run(); err != nil {
			return errors.Wrap(err, "failed to run first dd cmd")
		}
		if err := trCmd.Wait(); err != nil {
			return errors.Wrap(err, "failed to wait for tr cmd to complete")
		}
		if err := dd2Cmd.Wait(); err != nil {
			return errors.Wrap(err, "failed to wait for second dd cmd to complete")
		}
	} else {
		return errors.New("could not find RO_FRID in fmap")
	}

	testing.ContextLog(ctx, "Attempt to write fw with wp enabled")
	out, err = h.DUT.Conn().CommandContext(ctx, "flashrom", "-p", flashromTargets[target], "-w", "-i", fmt.Sprintf("WP_RO:%s", roTest)).CombinedOutput(ssh.DumpLogOnError)
	// We expect an error, so err shouldn't be nil, but neither should out. If out is nil, then the error is from an unrelated source.
	if out == nil {
		return errors.New("flashrom command did not produce any output")
	} else if match := reFlashromFail.FindSubmatch(out); match == nil {
		return errors.Errorf("flashrom did not produce failure message when trying to write: %s", string(out))
	}

	testing.ContextLog(ctx, "Read fw, make sure write didn't succeed with wp enabled")
	out, err = h.DUT.Conn().CommandContext(ctx, "flashrom", "-p", flashromTargets[target], "-r", "-i", fmt.Sprintf("WP_RO:%s", roAfter)).CombinedOutput(ssh.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to run command flashrom")
	} else if match := reFlashromSuccess.FindSubmatch(out); match == nil {
		return errors.Errorf("flashrom did not produce success message when trying to read: %s", string(out))
	}

	if err := performModeAwareReboot(ctx, h); err != nil {
		return errors.Wrap(err, "failed to do a mode aware reboot")
	}

	out, err = h.DUT.Conn().CommandContext(ctx, "cmp", roBefore, roAfter).CombinedOutput(ssh.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "failed to compare %q and %q using 'cmp'", roBefore, roAfter)
	} else if string(out) != "" {
		return errors.Wrapf(err, "files %q and %q were not identical, so either write protect or read failed", roBefore, roAfter)
	}

	testing.ContextLog(ctx, "Disable Write Protect")
	if err := setWriteProtect(ctx, h, target, false); err != nil {
		return errors.Wrap(err, "failed to set FW write protect state")
	}

	testing.ContextLog(ctx, "Attempt to write fw with write protect disabled")
	out, err = h.DUT.Conn().CommandContext(ctx, "flashrom", "-p", flashromTargets[target], "-w", "-i", fmt.Sprintf("WP_RO:%s", roTest)).CombinedOutput(ssh.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to run command flashrom")
	} else if match := reFlashromSuccess.FindSubmatch(out); match == nil {
		return errors.Errorf("flashrom did not produce success message when trying to write: %s", string(out))
	}

	testing.ContextLog(ctx, "Read fw, make sure write succeeded with wp disabled")
	out, err = h.DUT.Conn().CommandContext(ctx, "flashrom", "-p", flashromTargets[target], "-r", "-i", fmt.Sprintf("WP_RO:%s", roAfter)).CombinedOutput(ssh.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to run command flashrom")
	} else if match := reFlashromSuccess.FindSubmatch(out); match == nil {
		return errors.Errorf("flashrom did not produce success message when trying to read: %s", string(out))
	}

	out, err = h.DUT.Conn().CommandContext(ctx, "cmp", roTest, roAfter).CombinedOutput(ssh.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "failed to compare %q and %q using 'cmp'", roTest, roAfter)
	} else if string(out) != "" {
		return errors.Errorf("Files %q and %q were not identical", roTest, roAfter)
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

		// This file will get removed when tmpDirPath is removed.
		tmpImagePath := filepath.Join(tmpDirPath, "tmp_file.bin")
		flashromArgs := []string{
			"-p", flashromTargets[target],
			"-i", fmt.Sprintf("WP_RO:%s", tmpImagePath),
			"--wp-region", "WP_RO",
			fmt.Sprintf("--wp-%s", enableStr),
		}
		out, err := h.DUT.Conn().CommandContext(ctx, "flashrom", flashromArgs...).Output(ssh.DumpLogOnError)
		if err != nil {
			return errors.Wrapf(err, "failed run flashrom cmd: %s", string(out))
		} else if match := reFlashromSuccess.FindSubmatch(out); match == nil {
			return errors.Errorf("flashrom did not produce success message when trying to write: %s", string(out))
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

	return performModeAwareReboot(ctx, h)
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
