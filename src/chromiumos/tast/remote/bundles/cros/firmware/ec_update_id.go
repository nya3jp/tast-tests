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

	common "chromiumos/tast/common/firmware"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECUpdateID,
		Desc:         "Compare ec flash size to expected ec size from a chip-to-size map",
		Contacts:     []string{"tij@google.com", "cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		SoftwareDeps: []string{"crossystem", "flashrom"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Platform("fizz", "kalista")),
		ServiceDeps:  []string{"tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
		Params: []testing.Param{
			{
				Name:    "normal_mode",
				Fixture: fixture.NormalMode,
			},
			{
				Name:    "dev_gbb",
				Fixture: fixture.DevModeGBB,
			},
		},
	})
}

var tmpUpdateIDDir = filepath.Join("/", "mnt", "stateful_partition", fmt.Sprintf("flashrom_%d", time.Now().Unix()))

func ECUpdateID(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	// Back up AP fw, EC_RW, and EC_RW_B.
	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}
	s.Log("Back up entire current AP firmware")
	apPath, err := h.BiosServiceClient.BackupImageSection(ctx, &pb.FWBackUpSection{Section: pb.ImageSections(3), Programmer: pb.Programmers(0)})
	if err != nil {
		s.Fatal("Failed to backup current AP fw: ", err)
	}
	s.Log("AP firmware backup is stored at: ", apPath.Path)

	s.Log("Back up entire EC firmware")
	ecPath, err := h.BiosServiceClient.BackupImageSection(ctx, &pb.FWBackUpSection{Section: pb.ImageSections(3), Programmer: pb.Programmers(1)})
	if err != nil {
		s.Fatal("Failed to backup current EC region: ", err)
	}
	s.Log("EC backup is stored at: ", ecPath.Path)

	// Restore EC_RW and EC_RW_B.
	defer func() {
		// Require again here since reboots in test cause nil pointer errors otherwise.
		if err := h.RequireBiosServiceClient(ctx); err != nil {
			s.Fatal("Requiring BiosServiceClient: ", err)
		}

		s.Log("Restore AP firmware with backup from: ", apPath.Path)
		if _, err := h.BiosServiceClient.RestoreImageSection(ctx, apPath); err != nil {
			s.Fatal("Failed to restore AP firmware: ", err)
		}

		s.Log("Restore EC firmware with backup from: ", ecPath.Path)
		if _, err := h.BiosServiceClient.RestoreImageSection(ctx, ecPath); err != nil {
			s.Fatal("Failed to restore EC firmware: ", err)
		}

		s.Log("Delete AP fw backup")
		if _, err := h.DUT.Conn().CommandContext(ctx, "rm", apPath.Path).Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to delete AP backup: ", err)
		}

		s.Log("Delete EC fw backup")
		if _, err := h.DUT.Conn().CommandContext(ctx, "rm", ecPath.Path).Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to delete EC backup: ", err)
		}
	}()

	s.Log("Get initial write protect state")
	initialState, err := h.Servo.GetString(ctx, servo.FWWPState)
	if err != nil {
		s.Fatal("Failed to get initial write protect state: ", err)
	}
	defer func() {
		s.Log("Reset write protect to initial state: ", initialState)
		setInitWP := false
		if servo.FWWPStateValue(initialState) == servo.FWWPStateOn {
			setInitWP = true
		}
		if err := setFWWriteProtect(ctx, h, setInitWP); err != nil {
			s.Fatal("Failed to set FW write protect state: ", err)
		}
	}()

	workPath := filepath.Join(tmpUpdateIDDir, "work")
	s.Log("Create temp dir in DUT")
	if _, err = h.DUT.Conn().CommandContext(ctx, "mkdir", "-p", workPath).Output(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to create temp dirs: ", err)
	}
	// Clean up temp directory.
	defer func() {
		s.Log("Delete temp dir and contained files from DUT")
		if _, err := h.DUT.Conn().CommandContext(ctx, "rm", "-r", tmpUpdateIDDir).Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to delete temp dir: ", err)
		}
	}()

	flags := pb.GBBFlagsState{Clear: []pb.GBBFlag{pb.GBBFlag_DISABLE_EC_SOFTWARE_SYNC}, Set: []pb.GBBFlag{pb.GBBFlag_DEV_SCREEN_SHORT_DELAY}}
	if err := common.ClearAndSetGBBFlags(ctx, s.DUT(), flags); err != nil {
		s.Fatal("Error setting gbb flags: ", err)
	}

	s.Log("Disable write protect to allow for r/w for test")
	if err := setFWWriteProtect(ctx, h, false); err != nil {
		s.Fatal("Failed to disable FW write protect state: ", err)
	}

	activeCopy, err := h.Servo.GetString(ctx, servo.ECActiveCopy)
	if err != nil {
		s.Fatal("Failed to get ec active copy: ", err)
	}
	s.Log("Current active copy: ", string(activeCopy))
	if string(activeCopy) == "RW_B" {
		// Move EC_RW_B to EC_RW.
		s.Log("Current active copy is RW_B, switch to RW")
		ecrwbPath := filepath.Join(tmpUpdateIDDir, "ecrwb.bin")
		s.Logf("Save EC_RW_B image to %q in DUT", ecrwbPath)
		if out, err := h.DUT.Conn().CommandContext(ctx, "flashrom", "-p", "ec", "-r", "-i", fmt.Sprintf("EC_RW_B:%s", ecrwbPath)).Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to run flashrom cmd: ", err)
		} else if match := regexp.MustCompile(`SUCCESS`).FindSubmatch(out); match == nil {
			s.Fatal("Flashrom did not produce sucess message: ", string(out))
		}

		s.Log("Write EC_RW_B to EC_RW")
		if out, err := h.DUT.Conn().CommandContext(ctx, "flashrom", "-p", "ec", "-w", "-i", fmt.Sprintf("EC_RW:%s", ecrwbPath)).Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to run flashrom cmd: ", err)
		} else if match := regexp.MustCompile(`SUCCESS`).FindSubmatch(out); match == nil {
			s.Fatal("Flashrom did not produce sucess message: ", string(out))
		}

		if err := performModeAwareReboot(ctx, h); err != nil {
			s.Fatal("Failed to perform mode aware reboot")
		}
	}

	s.Log("Verify active copy is correctly changed to RW")
	activeCopy, err := h.Servo.GetString(ctx, servo.ECActiveCopy)
	if err != nil {
		s.Fatal("Failed to get ec active copy: ", err)
	} else if string(activeCopy) != "RW" {
		s.Fatal("Expected EC active copy to be RW not RW_B")
	}

	initialHash, err := firmware.NewECTool(s.DUT(), firmware.ECToolNameMain).Hash(ctx)
	if err != nil {
		s.Fatal("Failed to get initial ec hash: ", err)
	}
	s.Log("Initial ec hash: ", initialHash)

	s.Log("Modify ECID")
	if err := modifyECID(ctx, h); err != nil {
		s.Fatal("Failed to modify ec id: ", err)
	}

	modifiedHash, err := firmware.NewECTool(s.DUT(), firmware.ECToolNameMain).Hash(ctx)
	if err != nil {
		s.Fatal("Failed to get current ec hash: ", err)
	}
	s.Log("Current ec hash: ", modifiedHash)

	if err := performModeAwareReboot(ctx, h); err != nil {
		s.Fatal("Failed to perform mode aware reboot")
	}

	s.Log("Verify active copy is correctly changed to RW_B")
	activeCopy, err = h.Servo.GetString(ctx, servo.ECActiveCopy)
	if err != nil {
		s.Fatal("Failed to get ec active copy: ", err)
	} else if string(activeCopy) != "RB" {
		s.Fatal("Expected EC active copy to be RW not RW_B")
	}

	s.Log("Disable write protect to allow for corrupting section")
	if err := setFWWriteProtect(ctx, h, false); err != nil {
		s.Fatal("Failed to disable FW write protect state: ", err)
	}

	s.Log("Corrupt current active section")
	if err := corruptSection(ctx, h, "EC_RW_B"); err != nil {
		s.Fatal("Failed to corrupt EC_RW_B: ", err)
	}

	s.Log("Reenable write protect")
	if err := setFWWriteProtect(ctx, h, true); err != nil {
		s.Fatal("Failed to enable FW write protect state: ", err)
	}

	s.Log("Verify active copy is correctly changed to RW_B")
	activeCopy, err = h.Servo.GetString(ctx, servo.ECActiveCopy)
	if err != nil {
		s.Fatal("Failed to get ec active copy: ", err)
	} else if string(activeCopy) != "RW_B" {
		s.Fatal("Expected EC active copy to be RW_B not RW")
	}

	s.Log("Verify current hash is modified hash")
	currHash, err := firmware.NewECTool(s.DUT(), firmware.ECToolNameMain).Hash(ctx)
	if err != nil {
		s.Fatal("Failed to get current ec hash: ", err)
	} else if currHash != modifiedHash {
		s.Fatal("Expected current hash to match modified hash")
	}

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}
	s.Log("Restore AP firmware with backup from: ", apPath.Path)
	if _, err := h.BiosServiceClient.RestoreImageSection(ctx, apPath); err != nil {
		s.Fatal("Failed to restore AP firmware: ", err)
	}

	if err := performModeAwareReboot(ctx, h); err != nil {
		s.Fatal("Failed to perform mode aware reboot")
	}

	s.Log("Verify active copy is correctly back to RW")
	activeCopy, err = h.Servo.GetString(ctx, servo.ECActiveCopy)
	if err != nil {
		s.Fatal("Failed to get ec active copy: ", err)
	} else if string(activeCopy) != "RW" {
		s.Fatal("Expected EC active copy to be RW not RW_B")
	}

	s.Log("Verify current hash is reset to initial hash")
	currHash, err = firmware.NewECTool(s.DUT(), firmware.ECToolNameMain).Hash(ctx)
	if err != nil {
		s.Fatal("Failed to get current ec hash: ", err)
	} else if currHash != initialHash {
		s.Fatal("Expected current hash to match modified hash")
	}
}

func corruptSection(ctx context.Context, h *firmware.Helper, section string) error {
	// Temp file to hold current and later corrupted image section.
	sectionPath := filepath.Join(tmpUpdateIDDirm, fmt.Sprintf("img_%d", time.Now().Unix()))
	testing.ContextLogf(ctx, "Read %v section to file %v", section, sectionPath)
	if out, err := h.DUT.Conn().CommandContext(ctx, "flashrom", "-p", "ec", "-r", "-i", fmt.Sprintf("%s:%s", section, sectionPath)).Output(ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to run flashrom cmd")
	} else if match := regexp.MustCompile(`SUCCESS`).FindSubmatch(out); match == nil {
		return errors.Errorf("flashrom did not produce sucess message: %s", string(out))
	}

	out, err := h.DUT.Conn().CommandContext(ctx, "dump_fmap", "-p", sectionPath).Output(ssh.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to dump fmap")
	}
	testing.ContextLog(ctx, "dump_fmap output: ", string(out))

	// Format for the dumped fmap is "SectionName offset size".
	sectionMatch := regexp.MustCompile(fmt.Sprintf(`%s\s+(\d+)\s+(\d+)`, section)).FindSubmatch(out)
	if sectionMatch == nil {
		return errors.Errorf("didn't find %q section in fmap: %v", section, string(out))
	}
	sectionSize, err := strconv.Atoi(string(sectionMatch[2]))
	testing.ContextLogf(ctx, "Section %q size: %d", section, sectionSize)

	if out, err = h.DUT.Conn().CommandContext(ctx, "rm", sectionPath).Output(ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to delete temp file")
	}

	ddArgs := []string{
		"if=/dev/urandom", fmt.Sprintf("of=%s", sectionPath),
		"bs=1", fmt.Sprintf("count=%d", sectionSize),
	}
	testing.ContextLogf(ctx, "Generate random file of size: %d to path %v", sectionSize, sectionPath)
	if out, err = h.DUT.Conn().CommandContext(ctx, "dd", ddArgs...).Output(ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to create random file with dd cmd")
	}

	testing.ContextLog(ctx, "Write random file to section")
	if out, err := h.DUT.Conn().CommandContext(ctx, "flashrom", "-p", "ec", "-w", "-i", fmt.Sprintf("%s:%s", section, sectionPath)).Output(ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to run flashrom cmd")
	} else if match := regexp.MustCompile(`SUCCESS`).FindSubmatch(out); match == nil {
		return errors.Errorf("flashrom did not produce sucess message: %s", string(out))
	}

	testing.ContextLog(ctx, "Delete temp file at path ", sectionPath)
	if out, err = h.DUT.Conn().CommandContext(ctx, "rm", sectionPath).Output(ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to delete temp file")
	}
	return nil
}

func modifyECID(ctx context.Context, h *firmware.Helper) error {
	// Temp file to hold current and later corrupted image section.
	sectionPath := filepath.Join(tmpUpdateIDDirm, fmt.Sprintf("RW_FWID_%d", time.Now().Unix()))
	testing.ContextLog(ctx, "Dump EC to file ", sectionPath)
	if out, err := h.DUT.Conn().CommandContext(ctx, "flashrom", "-p", "ec", "-r", sectionPath).Output(ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to run flashrom cmd")
	} else if match := regexp.MustCompile(`SUCCESS`).FindSubmatch(out); match == nil {
		return errors.Errorf("flashrom did not produce sucess message: %s", string(out))
	}

	out, err := h.DUT.Conn().CommandContext(ctx, "dump_fmap", "-p", sectionPath).Output(ssh.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to dump fmap")
	}
	testing.ContextLog(ctx, "dump_fmap output: ", string(out))

	// Format for the dumped fmap is "SectionName offset size".
	sectionMatch := regexp.MustCompile(`RW_FWID\s+(\d+)\s+(\d+)`).FindSubmatch(out)
	if sectionMatch == nil {
		return errors.Errorf("didn't find RW_FWID section in fmap: %v", string(out))
	}

	sectionOffset, err := strconv.Atoi(string(sectionMatch[1]))
	if err != nil {
		return errors.Wrap(err, "failed to parse offset as int")
	}
	sectionSize, err := strconv.Atoi(string(sectionMatch[2]))
	if err != nil {
		return errors.Wrap(err, "failed to parse size as int")
	}
	testing.ContextLogf(ctx, "Section RW_FWID offset: %d size: %d", sectionOffset, sectionSize)

	// dd of=sectionPath bs=1 seek=secOffset count=secSize conv=notrunc
	ddArgs := []string{
		"if=/dev/urandom", fmt.Sprintf("of=%s", sectionPath),
		fmt.Sprintf("seek=%d", sectionOffset), "bs=1",
		fmt.Sprintf("count=%d", sectionSize), "conv=notrunc",
	}
	testing.ContextLog(ctx, "Replace RW_FWID with random string")
	if out, err = h.DUT.Conn().CommandContext(ctx, "dd", ddArgs...).Output(ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to update file with dd cmd")
	}

	// Dev key found at /usr/share/vboot/devkeys/key_ec_efs.vbprik2.
	keyPath := filepath.Join("/", "usr", "share", "vboot", "devkeys", "key_ec_efs.vbprik2")
	// Sign new image.
	if out, err = h.DUT.Conn().CommandContext(ctx, "futility", "sign", "--type", "rwsig", "--prikey", keyPath, sectionPath).Output(ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to run futility cmd")
	}

	testing.ContextLog(ctx, "Overwrite new image from ", sectionPath)
	if out, err = h.DUT.Conn().CommandContext(ctx, "flashrom", "-p", "ec", "-w", sectionPath).Output(ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to run flashrom cmd")
	} else if match := regexp.MustCompile(`SUCCESS`).FindSubmatch(out); match == nil {
		return errors.Errorf("flashrom did not produce sucess message: %s", string(out))
	}

	testing.ContextLog(ctx, "Delete temp file at path ", sectionPath)
	if out, err = h.DUT.Conn().CommandContext(ctx, "rm", sectionPath).Output(ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to delete temp file")
	}

	return performModeAwareReboot(ctx, h)
}

func setFWWriteProtect(ctx context.Context, h *firmware.Helper, enable bool) error {
	enableStr := "enable"
	fwwpState := servo.FWWPStateOn
	if !enable {
		enableStr = "disable"
		fwwpState = servo.FWWPStateOff
	}

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

	return performModeAwareReboot(ctx, h)
}

func performModeAwareReboot(ctx context.Context, h *firmware.Helper) error {
	// Create new mode switcher every time to prevent nil pointer errors.
	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		return errors.Wrap(err, "failed to create mode switcher")
	}
	return ms.ModeAwareReboot(ctx, firmware.ColdReset)
}
