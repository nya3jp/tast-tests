// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"os"
	"strconv"
	"time"

	"chromiumos/tast/ctxutil"
	fwUtils "chromiumos/tast/remote/bundles/cros/firmware/utils"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/firmware/reporters"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CorruptBothFWBodyAB,
		Desc:         "Servo based both firmware body A and B corruption test. This test requires a USB disk with ChromeOS test image plugged-in. this test corrupts both firmware body A and B. On next reboot, the firmware verification fails and enters recovery mode. This test then checks the success of the recovery boot",
		Contacts:     []string{"pf@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_experimental", "firmware_usb"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Timeout:      50 * time.Minute,
		Vars:         []string{"firmware.skipFlashUSB"},
		SoftwareDeps: []string{"crossystem", "flashrom"},
		ServiceDeps:  []string{"tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
		Params: []testing.Param{
			{
				Name:    "normal_mode",
				Fixture: fixture.NormalMode,
				Val:     "normal",
			},
			/* Disabled due to <1% pass rate over 30 days. See b/246820226
			{
				Name:    "dev_mode",
				Fixture: fixture.DevMode,
				Val:     "developer",
			}
			*/
		},
	})
}

func CorruptBothFWBodyAB(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}

	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Creating mode switcher: ", err)
	}

	cleanupContext := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Minute)
	defer cancel()
	s.Log("Backup firmware A/B body")
	FWBodyABkp, err := h.BiosServiceClient.BackupImageSection(ctx, &pb.FWSectionInfo{Section: pb.ImageSection_FWBodyAImageSection, Programmer: pb.Programmer_BIOSProgrammer})
	if err != nil {
		s.Fatal("Failed to backup current FW Body A region: ", err)
	}
	defer func(ctx context.Context) {
		if _, err := h.DUT.Conn().CommandContext(ctx, "rm", FWBodyABkp.Path).Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to delete FW Body A backup: ", err)
		}
	}(cleanupContext)
	FWBodyBBkp, err := h.BiosServiceClient.BackupImageSection(ctx, &pb.FWSectionInfo{Section: pb.ImageSection_FWBodyBImageSection, Programmer: pb.Programmer_BIOSProgrammer})
	if err != nil {
		s.Fatal("Failed to backup current FW Body B region: ", err)
	}
	defer func(ctx context.Context) {
		if _, err := h.DUT.Conn().CommandContext(ctx, "rm", FWBodyBBkp.Path).Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to delete FW Body B backup: ", err)
		}
	}(cleanupContext)

	s.Log("Copy backup files to the Host")
	FWBodyADst, err := os.CreateTemp("", "FWBodyABackup")
	if err != nil {
		s.Fatal("Failed to create temporary file for firmware sign A backup")
	}
	defer func() {
		FWBodyADst.Close()
		os.Remove(FWBodyADst.Name())
	}()

	FWBodyBDst, err := os.CreateTemp("", "FWBodyBBackup")
	if err != nil {
		s.Fatal("Failed to create temporary file for firmware sign B backup")
	}
	defer func() {
		FWBodyBDst.Close()
		os.Remove(FWBodyBDst.Name())
	}()

	if err := linuxssh.GetFile(ctx, s.DUT().Conn(), FWBodyABkp.Path, FWBodyADst.Name(), linuxssh.PreserveSymlinks); err != nil {
		s.Fatal("Failed to copy a FW A Sign backup to the Host")
	}
	if err := linuxssh.GetFile(ctx, s.DUT().Conn(), FWBodyBBkp.Path, FWBodyBDst.Name(), linuxssh.PreserveSymlinks); err != nil {
		s.Fatal("Failed to copy a FW B Sign backup to the Host")
	}

	s.Log("Setup USB Key")
	skipFlashUSB := false
	if skipFlashUSBStr, ok := s.Var("firmware.skipFlashUSB"); ok {
		skipFlashUSB, err = strconv.ParseBool(skipFlashUSBStr)
		if err != nil {
			s.Fatalf("Invalid value for var firmware.skipFlashUSB: got %q, want true/false", skipFlashUSBStr)
		}
	}
	cs := s.CloudStorage()
	if skipFlashUSB {
		cs = nil
	}
	if err := h.SetupUSBKey(ctx, cs); err != nil {
		s.Fatal("USBKey not working: ", err)
	}

	// Restore FW Signatures
	defer func(ctx context.Context) {
		// Disable wp so backup can be restored.
		if err := fwUtils.SetFWWriteProtect(ctx, h, false); err != nil {
			s.Fatal("Failed to set FW write protect state: ", err)
		}

		if err := h.RequireServo(ctx); err != nil {
			s.Fatal("Failed to init servo: ", err)
		}

		s.Log("Syncing TAST File from HOST")
		if err := h.SyncTastFilesToDUT(ctx); err != nil {
			s.Log(err, "syncing Tast files to DUT after booting to recovery")
		}

		// Require again here since reboots in test cause nil pointer errors otherwise.
		if err := h.RequireBiosServiceClient(ctx); err != nil {
			s.Fatal("Requiring BiosServiceClient: ", err)
		}

		s.Log("Get back FW Bodys backup from host to DUT")
		if _, err := linuxssh.PutFiles(ctx, s.DUT().Conn(), map[string]string{FWBodyADst.Name(): FWBodyABkp.Path, FWBodyBDst.Name(): FWBodyBBkp.Path}, linuxssh.PreserveSymlinks); err != nil {
			s.Fatal("Failed to get backup files to DUT from Host")
		}

		s.Log("Restore firmware bodies")
		if _, err := h.BiosServiceClient.RestoreImageSection(ctx, FWBodyABkp); err != nil {
			s.Fatal("Failed to restore FW Body A: ", err)
		}
		if _, err := h.BiosServiceClient.RestoreImageSection(ctx, FWBodyBBkp); err != nil {
			s.Fatal("Failed to restore FW Body B: ", err)
		}

		bootModeName := s.Param().(string)

		if bootModeName == "developer" {
			if err := ms.ModeAwareReboot(ctx, firmware.WarmReset, firmware.AssumeRecoveryMode, firmware.ExpectDevModeAfterReboot); err != nil {
				s.Fatal("Failed to perform mode aware reboot: ", err)
			}
		} else {
			if err := ms.ModeAwareReboot(ctx, firmware.WarmReset, firmware.AssumeRecoveryMode); err != nil {
				s.Fatal("Failed to perform mode aware reboot: ", err)
			}
		}

		s.Log(ctx, "Reestablishing connection to DUT")
		connectCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
		if err := h.WaitConnect(connectCtx); err != nil {
			s.Fatal("Failed to reconnect to DUT after booting to recovery mode: ", err)
		}

		if mainFWType, err := h.Reporter.CrossystemParam(ctx, reporters.CrossystemParamMainfwType); err != nil {
			s.Fatal("Failed to get crossystem mainfw_type: ", err)
		} else if mainFWType != bootModeName {
			s.Fatalf("Failed to match mainfw_type: got %q, want %q", mainFWType, bootModeName)
		}

	}(cleanupContext)

	s.Log("Corrupt Firmware A Body")
	if _, err := h.BiosServiceClient.CorruptFWSection(ctx, &pb.FWSectionInfo{Section: pb.ImageSection_FWBodyAImageSection, Programmer: pb.Programmer_BIOSProgrammer}); err != nil {
		s.Fatal("Failed to corrupt Firmware A Body (FVMAIN) section: ", err)
	}

	s.Log("Corrupt Firmware B Body")
	if _, err := h.BiosServiceClient.CorruptFWSection(ctx, &pb.FWSectionInfo{Section: pb.ImageSection_FWBodyBImageSection, Programmer: pb.Programmer_BIOSProgrammer}); err != nil {
		s.Fatal("Failed to corrupt Firmware B Body (FVMAINB) section: ", err)
	}

	s.Log("Copy TAST Files from DUT")
	if err := h.CopyTastFilesFromDUT(ctx); err != nil {
		s.Fatal(err, "copying Tast files from DUT to test server")
	}

	if err := h.DUT.Conn().CommandContext(ctx, "sync").Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to sync DUT: ", err)
	}

	s.Log("Check recovery reason")
	if err := fwUtils.CheckRecReason(ctx, h, ms, []reporters.RecoveryReason{reporters.RecoveryReasonROInvalidRW, reporters.RecoveryReasonRWVerifyBody}); err != nil {
		s.Fatal("Failed when checking recovery reason: ", err)
	}
}
