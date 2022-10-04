// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
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
		Desc:         "Verify corrupting RW firmware in EFS system results in switching between RW and RW_B and is corrected by AP",
		Contacts:     []string{"tij@google.com", "cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware", "firmware_ec"},
		SoftwareDeps: []string{"crossystem", "flashrom"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Platform("fizz", "kalista")),
		ServiceDeps:  []string{"tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
		Timeout:      15 * time.Minute,
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

	initActiveCopy, err := h.Servo.GetString(ctx, servo.ECActiveCopy)
	if err != nil {
		s.Fatal("Failed to get ec active copy: ", err)
	} else if initActiveCopy != "RW" {
		s.Logf("Expected initial active copy to be 'RW', got %q instead. Attempting to sysjump to RW A", initActiveCopy)
		if err := h.Servo.RunECCommand(ctx, "sysjump A"); err != nil {
			s.Log("Failed to perform sysjump: ", err)
		}
		// Expect EC to be in RW_A now.
		if initActiveCopy, err := h.Servo.GetString(ctx, servo.ECActiveCopy); err != nil {
			s.Fatal("Failed to get ec active copy: ", err)
		} else if initActiveCopy != "RW" {
			s.Fatalf("Expected initial active copy to be 'RW', got %q instead", initActiveCopy)
		}
	}

	cleanupContext := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Minute)
	defer cancel()

	s.Log("Get initial write protect state")
	initialState, err := h.Servo.GetString(ctx, servo.FWWPState)
	if err != nil {
		s.Fatal("Failed to get initial write protect state: ", err)
	}

	// Back up AP fw, EC_RW, and EC_RW_B.
	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}
	s.Log("Back up EC_RW firmware")
	ecrwPath, err := h.BiosServiceClient.BackupImageSection(ctx, &pb.FWSectionInfo{Section: pb.ImageSection_ECRWImageSection, Programmer: pb.Programmer_ECProgrammer})
	if err != nil {
		s.Fatal("Failed to backup current EC_RW region: ", err)
	}
	s.Log("EC_RW backup is stored at: ", ecrwPath.Path)
	defer func(ctx context.Context) {
		s.Log("Delete EC_RW fw backup")
		if _, err := h.DUT.Conn().CommandContext(ctx, "rm", ecrwPath.Path).Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to delete EC_RW backup: ", err)
		}
	}(cleanupContext)

	s.Log("Back up EC_RW_B firmware")
	ecrwbPath, err := h.BiosServiceClient.BackupImageSection(ctx, &pb.FWSectionInfo{Section: pb.ImageSection_ECRWBImageSection, Programmer: pb.Programmer_ECProgrammer})
	if err != nil {
		s.Fatal("Failed to backup current EC_RW_B region: ", err)
	}
	s.Log("EC_RW_B backup is stored at: ", ecrwbPath.Path)
	defer func(ctx context.Context) {
		s.Log("Delete EC_RW_B fw backup")
		if _, err := h.DUT.Conn().CommandContext(ctx, "rm", ecrwbPath.Path).Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to delete EC_RW_B backup: ", err)
		}
	}(cleanupContext)

	workPath := filepath.Join(tmpUpdateIDDir, "work")
	s.Log("Create temp dir in DUT")
	if _, err = h.DUT.Conn().CommandContext(ctx, "mkdir", "-p", workPath).Output(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to create temp dirs: ", err)
	}
	s.Log("Created temp directory at: ", tmpUpdateIDDir)
	defer func(ctx context.Context) {
		s.Log("Delete temp dir and contained files from DUT")
		if _, err := h.DUT.Conn().CommandContext(ctx, "rm", "-r", tmpUpdateIDDir).Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to delete temp dir: ", err)
		}
	}(cleanupContext)

	defer func(ctx context.Context) {
		s.Log("Reset write protect to initial state: ", initialState)
		setInitWP := false
		if servo.FWWPStateValue(initialState) == servo.FWWPStateOn {
			setInitWP = true
		}
		if err := setFWWriteProtect(ctx, h, setInitWP); err != nil {
			s.Fatal("Failed to set FW write protect state: ", err)
		}
		// Disable wp so backup can be restored.
		if err := setFWWriteProtect(ctx, h, false); err != nil {
			s.Fatal("Failed to set FW write protect state: ", err)
		}

		// Restore EC_RW and EC_RW_B.
		// Require again here since reboots in test cause nil pointer errors otherwise.
		if err := h.RequireBiosServiceClient(ctx); err != nil {
			s.Fatal("Requiring BiosServiceClient: ", err)
		}

		s.Log("Restore EC_RW firmware with backup from: ", ecrwPath.Path)
		if _, err := h.BiosServiceClient.RestoreImageSection(ctx, ecrwPath); err != nil {
			s.Fatal("Failed to restore EC_RW firmware: ", err)
		}

		s.Log("Restore EC_RW_B firmware with backup from: ", ecrwbPath.Path)
		if _, err := h.BiosServiceClient.RestoreImageSection(ctx, ecrwbPath); err != nil {
			s.Fatal("Failed to restore EC_RW_B firmware: ", err)
		}
	}(cleanupContext)

	flags := pb.GBBFlagsState{Clear: []pb.GBBFlag{pb.GBBFlag_DISABLE_EC_SOFTWARE_SYNC}, Set: []pb.GBBFlag{pb.GBBFlag_DEV_SCREEN_SHORT_DELAY}}
	if err := common.ClearAndSetGBBFlags(ctx, s.DUT(), &flags); err != nil {
		s.Fatal("Error setting gbb flags: ", err)
	}

	initialHash, err := firmware.NewECTool(s.DUT(), firmware.ECToolNameMain).Hash(ctx)
	if err != nil {
		s.Fatal("Failed to get initial ec hash: ", err)
	}

	s.Log("Verify active copy is correctly changed after corrupting")
	activeCopy, err := testCorruptActiveSectionAndReboot(ctx, h, s.DUT(), initialHash)
	if err != nil {
		s.Fatal("Failed to test changing active copy after corruption: ", err)
	}
	s.Log("Current active copy: ", string(activeCopy))

	s.Log("Verify active copy is correctly changed back after corrupting secondary copy")
	activeCopy, err = testCorruptActiveSectionAndReboot(ctx, h, s.DUT(), initialHash)
	if err != nil {
		s.Fatal("Failed to test changing active copy after corruption: ", err)
	}
	s.Log("Current active copy: ", string(activeCopy))
}

func testCorruptActiveSectionAndReboot(ctx context.Context, h *firmware.Helper, d *dut.DUT, initHash string) (string, error) {
	var activeCopyToRegionMap = map[string]string{
		"RW":   "EC_RW",
		"RW_B": "EC_RW_B",
	}

	initActiveCopy, err := h.Servo.GetString(ctx, servo.ECActiveCopy)
	if err != nil {
		return "", errors.Wrap(err, "failed to get ec active copy")
	}
	testing.ContextLog(ctx, "Initial active copy: ", initActiveCopy)

	testing.ContextLog(ctx, "Disable write protect to allow for r/w for test")
	if err := setFWWriteProtect(ctx, h, false); err != nil {
		return "", errors.Wrap(err, "failed to disable FW write protect state")
	}

	testing.ContextLog(ctx, "Corrupt current active section: ", activeCopyToRegionMap[initActiveCopy])
	if err := corruptSection(ctx, h, activeCopyToRegionMap[initActiveCopy]); err != nil {
		return "", errors.Wrapf(err, "failed to corrupt current active copy: %s", initActiveCopy)
	}

	testing.ContextLog(ctx, "Reenable write protect")
	if err := setFWWriteProtect(ctx, h, true); err != nil {
		return "", errors.Wrap(err, "failed to enable FW write protect state")
	}

	testing.ContextLog(ctx, "perform dut reboot")
	if err := coldModeAwareReboot(ctx, h); err != nil {
		return "", errors.Wrap(err, "failed to perform mode aware reboot")
	}

	testing.ContextLog(ctx, "Verify echash has not changed")
	currHash, err := firmware.NewECTool(d, firmware.ECToolNameMain).Hash(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get current ec hash")
	} else if currHash != initHash {
		return "", errors.Errorf("expected hash to remain %q but is now %q", initHash, currHash)
	}

	testing.ContextLog(ctx, "Verify active copy is correctly changed")
	activeCopy, err := h.Servo.GetString(ctx, servo.ECActiveCopy)
	if err != nil {
		return "", errors.Wrap(err, "failed to get ec active copy")
	} else if activeCopy == initActiveCopy {
		return "", errors.Wrapf(err, "Corrupting should result in different active copy, got %s", initActiveCopy)
	}

	return activeCopy, nil
}

func corruptSection(ctx context.Context, h *firmware.Helper, section string) error {
	// Temp file to hold current and later corrupted image section.
	sectionPath := filepath.Join(tmpUpdateIDDir, fmt.Sprintf("img_%d", time.Now().Unix()))
	testing.ContextLog(ctx, "Read WP_RO section to file ", sectionPath)
	if out, err := h.DUT.Conn().CommandContext(ctx, "flashrom", "-p", "ec", "-r", "-i", fmt.Sprintf("WP_RO:%s", sectionPath)).Output(ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to run flashrom cmd")
	} else if match := regexp.MustCompile(`SUCCESS`).FindSubmatch(out); match == nil {
		return errors.Errorf("flashrom did not produce sucess message: %s", string(out))
	}

	testing.ContextLog(ctx, "Checking fmap")
	out, err := h.DUT.Conn().CommandContext(ctx, "dump_fmap", "-p", sectionPath).Output(ssh.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to dump fmap")
	}

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

	return nil
}

func coldModeAwareReboot(ctx context.Context, h *firmware.Helper) error {
	// Create new mode switcher every time to prevent nil pointer errors.
	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		return errors.Wrap(err, "failed to create mode switcher")
	}
	return ms.ModeAwareReboot(ctx, firmware.ColdReset)
}
