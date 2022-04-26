// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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

func init() {
	testing.AddTest(&testing.Test{
		Func:         RollbackFirmware,
		Desc:         "Test EC Keyboard interface",
		Contacts:     []string{"tij@google.com", "cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		ServiceDeps:  []string{"tast.cros.firmware.BiosService"},
		SoftwareDeps: []string{"crossystem", "flashrom"},
		Vars:         []string{"firmware.skipFlashUSB", "firmware.noVerifyUSB"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Timeout:      20 * time.Minute,
		Params: []testing.Param{
			{
				Name:    "dev_mode",
				Fixture: fixture.DevMode,
				Val:     fixture.DevMode,
			},
			{
				Name:    "normal_mode",
				Fixture: fixture.NormalMode,
				Val:     fixture.NormalMode,
			},
		},
	})
}

func RollbackFirmware(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}

	prog := pb.Programmer_BIOSProgrammer
	vblockA := pb.ImageSection_VBLOCKAImageSection
	// vblockB := pb.ImageSection_VBLOCKBImageSection
	// fwToImg := map[string]pb.ImageSection{
	// 	"A": vblockA,
	// 	"B": vblockB,
	// }

	if noVerifyUSBStr, ok := s.Var("firmware.skipFlashUSB"); ok {
		noVerifyUSB, err := strconv.ParseBool(noVerifyUSBStr)
		if err != nil {
			s.Fatalf("Invalid value for var firmware.noVerifyUSB: got %q, want true/false", noVerifyUSBStr)
		}
		if !noVerifyUSB {
			s.Log("wth")
			skipFlashUSB := false
			if skipFlashUSBStr, ok := s.Var("firmware.skipFlashUSB"); ok {
				skip, err := strconv.ParseBool(skipFlashUSBStr)
				if err != nil {
					s.Fatalf("Invalid value for var firmware.skipFlashUSB: got %q, want true/false", skipFlashUSBStr)
				}
				skipFlashUSB = skip
			}
			cs := s.CloudStorage()
			if skipFlashUSB {
				cs = nil
			}
			if err := h.SetupUSBKey(ctx, cs); err != nil {
				s.Fatal("USBKey not working: ", err)
			}
		}
	}
	// List of fw sections to back up.
	sectionInfos := []*pb.ImageSectionInfo{
		&pb.ImageSectionInfo{Section: vblockA, Programmer: prog},
		// &pb.ImageSectionInfo{Section: vblockB, Programmer: prog},
		// &pb.ImageSectionInfo{Section: pb.ImageSection_FWMAINAImageSection, Programmer: prog},
		// &pb.ImageSectionInfo{Section: pb.ImageSection_FWMAINBImageSection, Programmer: prog},
	}
	backups, err := backupFWToServo(ctx, h, sectionInfos...)
	if err != nil {
		s.Fatal("Failed to back up fw: ", err)
	}

	cleanupContext := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Minute)
	defer cancel()
	// Restore AP.
	defer func(ctx context.Context) {
		// Require again here since reboots in test cause nil pointer errors otherwise.
		if err := h.RequireBiosServiceClient(ctx); err != nil {
			s.Fatal("Requiring BiosServiceClient: ", err)
		}

		s.Log("Show backups")
		if out, err := h.DUT.Conn().CommandContext(ctx, "ls", "-l", filepath.Dir(backups[0].Backup.Path)).Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to show backups: ", err)
		} else {
			s.Logf("Out %v", string(out))
		}

		if err := restoreFWFromServo(ctx, h, backups); err != nil {
			s.Fatal("Failed to restore backups: ", err)
		}

		s.Log("Show backups, verify removed")
		if out, err := h.DUT.Conn().CommandContext(ctx, "ls", "-l", filepath.Dir(backups[0].Backup.Path)).Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to show backups: ", err)
		} else {
			s.Logf("Out %v", string(out))
		}
	}(cleanupContext)

	// initialFw, err := h.Reporter.CrossystemParam(ctx, reporters.CrossystemParamMainfwAct)
	// if err != nil {
	// 	s.Fatal("Failed to get crossystem param mainfw_act: ", err)
	// }
	// s.Logf("The initial mainfw_act is %q", initialFw)

	// altFw := "B"
	// if initialFw == altFw {
	// 	altFw = "A"
	// }

	// s.Log("Check current mainfw act")
	// if err := checkMainFwAct(ctx, h, initialFw); err != nil {
	// 	s.Fatal("Unexpected main fw: ", err)
	// }

	// s.Log("Get current fw version")
	// ver, err := h.BiosServiceClient.GetSectionVersion(ctx, &pb.ImageSectionInfo{Section: fwToImg[initialFw], Programmer: prog})
	// if err != nil {
	// 	s.Fatal("Failed to get current version: ", err)
	// }
	// s.Logf("Current vblock %v version is %d", ver.Section, ver.Version)
	// ver.Version--
	// s.Logf("Setting vblock %v version to %d", ver.Section, ver.Version)
	// if _, err := h.BiosServiceClient.SetSectionVersion(ctx, ver); err != nil {
	// 	s.Fatal("Failed to set current version: ", err)
	// }

	// s.Log("Verifying version changed")
	// newVer, err := h.BiosServiceClient.GetSectionVersion(ctx, &pb.ImageSectionInfo{Section: fwToImg[initialFw], Programmer: prog})
	// if err != nil {
	// 	s.Fatal("Failed to get current version: ", err)
	// }
	// if newVer.Version == ver.Version {
	// 	s.Fatal("Expected version to change from %d to %d but got %d", ver.Version, ver.Version-1, newVer.Version)
	// }

	// s.Log("Reboot DUT to change mainfw_act")
	// ms, err := firmware.NewModeSwitcher(ctx, h)
	// if err != nil {
	// 	s.Fatal("Failed to create mode switcher: ", err)
	// }
	// if err := ms.ModeAwareReboot(ctx, firmware.ColdReset); err != nil {
	// 	s.Fatal("Failed to perform mode aware reboot: ", err)
	// }

	// if err := h.Servo.SetPowerState(ctx, servo.PowerStateWarmReset); err != nil {
	// 	s.Fatal("Failed to set usb mux to host: ", err)
	// }

	// s.Log("Sleeping for 5s")
	// if err := testing.Sleep(ctx, 5*time.Second); err != nil {
	// 	s.Fatal("Failed to sleep for 5s: ", err)
	// }

	// s.Logf("Verify mainfw act changed to %q", altFw)
	// if err := checkMainFwAct(ctx, h, altFw); err != nil {
	// 	s.Fatal("Unexpected main fw: ", err)
	// }

	// Need to keep re-requiring since RPC connection drops during mode aware reboot.
	// if err := h.RequireBiosServiceClient(ctx); err != nil {
	// 	s.Fatal("Requiring BiosServiceClient: ", err)
	// }
	// ver, err = h.BiosServiceClient.GetSectionVersion(ctx, &pb.ImageSectionInfo{Section: fwToImg[altFw], Programmer: prog})
	// if err != nil {
	// 	s.Fatal("Failed to get current version: ", err)
	// }
	// s.Logf("Current vblock %v version is %d", ver.Section, ver.Version)
	// ver.Version--
	// s.Logf("Setting vblock %v version to %d", ver.Section, ver.Version)
	// if _, err := h.BiosServiceClient.SetSectionVersion(ctx, ver); err != nil {
	// 	s.Fatal("Failed to set current version: ", err)
	// }

	// s.Log("Reboot DUT to change mainfw_act")
	// ms, err = firmware.NewModeSwitcher(ctx, h)
	// if err != nil {
	// 	s.Fatal("Failed to create mode switcher: ", err)
	// }
	// if err := ms.ModeAwareReboot(ctx, firmware.ColdReset); err != nil {
	// 	s.Fatal("Failed to perform mode aware reboot: ", err)
	// }

	// s.Log("USB to host")
	// if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxHost); err != nil {
	// 	s.Fatal("Failed to set usb mux to host: ", err)
	// }

	// s.Log("Go to PowerStateReset")
	// if err := h.Servo.SetPowerState(ctx, servo.PowerStateReset); err != nil {
	// 	s.Fatal("Failed reset dut: ", err)
	// }

	// s.Log("Sleeping for 5s")
	// if err := testing.Sleep(ctx, 5*time.Second); err != nil {
	// 	s.Fatal("Failed to sleep for 5s: ", err)
	// }

	// s.Log("Sleeping for 5s")
	// if err := testing.Sleep(ctx, 5*time.Second); err != nil {
	// 	s.Fatal("Failed to sleep for 5s: ", err)
	// }

	// ms, err = firmware.NewModeSwitcher(ctx, h)
	// if err != nil {
	// 	s.Fatal("Failed to create mode switcher: ", err)
	// }

	// s.Log("Move from recovery to boot mode")
	// bootMode := s.Param().(fwCommon.BootMode)
	// if bootMode == fixture.NormalMode {
	// 	if err := ms.FwScreenToNormalMode(ctx); err != nil {
	// 		s.Fatal("Failed to boot to normal mode: ", err)
	// 	}
	// } else {
	// 	if err := ms.FwScreenToDevMode(ctx); err != nil {
	// 		s.Fatal("Failed to boot to dev mode: ", err)
	// 	}
	// }

	// s.Log("Go to PowerStateRec")
	// if err := h.Servo.SetPowerState(ctx, servo.PowerStateRec); err != nil {
	// 	s.Fatal("Failed reset dut: ", err)
	// }

	// s.Log("Sleeping for 30s")
	// if err := testing.Sleep(ctx, 30*time.Second); err != nil {
	// 	s.Fatal("Failed to sleep for 5s: ", err)
	// }
	// s.Log("USB to DUT")
	// if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxDUT); err != nil {
	// 	s.Fatal("Failed to set usb mux to host: ", err)
	// }

	// if err := checkRecoveryCrosParams(ctx, h); err != nil {
	// 	s.Fatal("Failed to check recovery crossystem params: ", err)
	// }

}

func checkRecoveryCrosParams(ctx context.Context, h *firmware.Helper) error {
	testing.ContextLog(ctx, "Getting crossystem")
	mainfwType, err := h.Reporter.CrossystemParam(ctx, reporters.CrossystemParamMainfwType)
	if err != nil {
		return errors.Wrap(err, "failed to get crossystem param mainfw_act")
	}

	testing.ContextLog(ctx, "mainfw_type is: %v", mainfwType)

	recoveryReason, err := h.Reporter.CrossystemParam(ctx, reporters.CrossystemParamRecoveryReason)
	if err != nil {
		return errors.Wrap(err, "failed to get crossystem param mainfw_act")
	}

	testing.ContextLog(ctx, "recovery_reason is: %v", recoveryReason)
	return nil
}

func checkMainFwAct(ctx context.Context, h *firmware.Helper, expState string) error {
	testing.ContextLog(ctx, "Getting crossystem")
	out, err := h.Reporter.CrossystemParam(ctx, reporters.CrossystemParamMainfwAct)
	if err != nil {
		return errors.Wrap(err, "failed to get crossystem param mainfw_act")
	}

	if strings.ToUpper(out) != strings.ToUpper(expState) {
		return errors.Errorf("expected mainfw_act value to be %v, but got %v", expState, out)
	}
	return nil
}

type servoBackUpInfo struct {
	Backup    *pb.FWBackUpInfo
	ServoPath string
}

func restoreFWFromServo(ctx context.Context, h *firmware.Helper, backups []*servoBackUpInfo) error {
	if err := h.RequireBiosServiceClient(ctx); err != nil {
		return errors.Wrap(err, "requiring BiosServiceClient")
	}
	if err := h.RequireServo(ctx); err != nil {
		return errors.Wrap(err, "requiring servo")
	}
	// return nil

	servoPath := backups[0].ServoPath
	if err := h.CopyFwBackupToDUT(ctx, servoPath); err != nil {
		err = errors.Wrapf(err, "failed to restore region backup from path on servo")
		return err
	}

	if _, err := h.DUT.Conn().CommandContext(ctx, "cp", "-r", "/usr/local/share/tast/fw_backup", "/var/tmp/").Output(ssh.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to delete region backup from path")
	}

	collectedErr := make([]error, 0)
	for _, info := range backups {
		backup := info.Backup
		servoPath := info.ServoPath
		// testing.ContextLogf(ctx, "Touch %v region backup from path %v", backup.Section, backup.Path)
		// if _, err := h.DUT.Conn().CommandContext(ctx, "touch", backup.Path).Output(ssh.DumpLogOnError); err != nil {
		// 	err = errors.Wrapf(err, "failed to delete %v region backup from path %v", backup.Section, backup.Path)
		// 	return err
		// 	collectedErr = append(collectedErr, err)
		// }
		testing.ContextLogf(ctx, "Restore %v region backup from servo path %v to local path %v", backup.Section, servoPath, backup.Path)
		// if err := remotebios.RestoreFWFileFromRemote(ctx, h.ServoProxy, backup, servoPath); err != nil {
		// 	err = errors.Wrapf(err, "failed to restore %v region backup from path %v on servo", backup.Section, servoPath)
		// 	return err
		// 	collectedErr = append(collectedErr, err)
		// }

		// testing.ContextLogf(ctx, "Restoring %v region with programmer %v from path %v", backup.Section, backup.Programmer, backup.Path)
		// if _, err := h.BiosServiceClient.RestoreImageSection(ctx, backup); err != nil {
		// 	err = errors.Wrapf(err, "failed to restore %v region with programmer %v from path %v", backup.Section, backup.Programmer, backup.Path)
		// 	return err
		// 	collectedErr = append(collectedErr, err)
		// }
		// testing.ContextLogf(ctx, "Deleting %v region backup from path %v", backup.Section, backup.Path)
		// if _, err := h.DUT.Conn().CommandContext(ctx, "rm", backup.Path).Output(ssh.DumpLogOnError); err != nil {
		// 	collectedErr = append(collectedErr, errors.Wrapf(err, "failed to delete %v region backup from path %v", backup.Section, backup.Path))
		// }
	}
	if len(collectedErr) != 0 {
		joinedErr := collectedErr[0].Error()
		for _, err := range collectedErr[1:] {
			joinedErr = joinedErr + fmt.Sprintf("\n%s", err.Error())
		}
		return errors.New(joinedErr)
	}
	return nil

}

func backupFWToServo(ctx context.Context, h *firmware.Helper, sectionInfos ...*pb.ImageSectionInfo) ([]*servoBackUpInfo, error) {
	backups := make([]*servoBackUpInfo, len(sectionInfos))

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		return nil, errors.Wrap(err, "requiring BiosServiceClient")
	}
	if err := h.RequireServo(ctx); err != nil {
		return nil, errors.Wrap(err, "requiring servo")
	}

	if _, err := h.DUT.Conn().CommandContext(ctx, "mkdir", "/usr/local/share/tast/fw_backup").Output(ssh.DumpLogOnError); err != nil {
		return nil, errors.Wrapf(err, "failed to delete region backup from path")
	}

	for idx, info := range sectionInfos {
		testing.ContextLogf(ctx, "Backing up current %v region with programmer %v", info.Section, info.Programmer)
		backup, err := h.BiosServiceClient.BackupImageSection(ctx, info)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to backup current %v region with programmer %v", info.Section, info.Programmer)
		}
		if _, err := h.DUT.Conn().CommandContext(ctx, "cp", backup.Path, "/usr/local/share/tast/fw_backup").Output(ssh.DumpLogOnError); err != nil {
			return nil, errors.Wrapf(err, "failed to delete %v region backup from path %v", backup.Section, backup.Path)
		}

		// testing.ContextLog(ctx, "Copying file to servo")
		// servoPath, err := remotebios.CopyFWFileToRemote(ctx, h, backup)
		// if err != nil {
		// 	return nil, errors.Wrapf(err, "failed to copy %v region backup to servo", backup.Section)
		// }

		// testing.ContextLogf(ctx, "Deleting %v region backup from path %v", backup.Section, backup.Path)
		// if _, err := h.DUT.Conn().CommandContext(ctx, "rm", backup.Path).Output(ssh.DumpLogOnError); err != nil {
		// 	return nil, errors.Wrapf(err, "failed to delete %v region backup from path %v", backup.Section, backup.Path)
		// }
		backups[idx] = &servoBackUpInfo{ServoPath: "", Backup: backup}
		testing.ContextLogf(ctx, "backup info: %v, servoPath: %v", backup, "")
	}

	testing.ContextLog(ctx, "Copying file to servo")
	servoPath, err := h.CopyFwBackupFromDUT(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to copy region backup to servo")
	}
	backups[0].ServoPath = servoPath

	return backups, nil
}
