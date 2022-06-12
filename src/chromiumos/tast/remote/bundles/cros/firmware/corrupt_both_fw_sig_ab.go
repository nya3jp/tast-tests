// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"io/ioutil"
	"os"
	"strconv"
	"time"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
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
		Func:         CorruptBothFWSigAB,
		Desc:         "Servo based both firmware signature A and B corruption test. This test requires a USB disk with ChromeOS test image plugged-in. this test corrupts both firmware signature A and B. On next reboot, the firmware verification fails and enters recovery mode. This test then checks the success of the recovery boot",
		Contacts:     []string{"pf@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_experimental", "firmware_usb"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Timeout:      20 * time.Minute,
		Vars:         []string{"firmware.skipFlashUSB"},
		SoftwareDeps: []string{"crossystem", "flashrom"},
		ServiceDeps:  []string{"tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
		Params: []testing.Param{
			{
				Name:    "normal_mode",
				Fixture: fixture.NormalMode,
				Val:     "normal",
			},
			{
				Name:    "dev_mode",
				Fixture: fixture.DevMode,
				Val:     "developer",
			},
		},
	})
}

func CorruptBothFWSigAB(ctx context.Context, s *testing.State) {
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

	s.Log("Backup firmware A/B signatures")
	FWSignABkp, err := h.BiosServiceClient.BackupImageSection(ctx, &pb.FWBackUpSection{Section: pb.ImageSection_FWSignAImageSection, Programmer: pb.Programmer_BIOSProgrammer})
	if err != nil {
		s.Fatal("Failed to backup current FW Sign A region: ", err)
	}
	defer func() {
		if _, err := h.DUT.Conn().CommandContext(ctx, "rm", FWSignABkp.Path).Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to delete FW Sign A backup: ", err)
		}
	}()
	FWSignBBkp, err := h.BiosServiceClient.BackupImageSection(ctx, &pb.FWBackUpSection{Section: pb.ImageSection_FWSignBImageSection, Programmer: pb.Programmer_BIOSProgrammer})
	if err != nil {
		s.Fatal("Failed to backup current FW Sign B region: ", err)
	}
	defer func() {
		if _, err := h.DUT.Conn().CommandContext(ctx, "rm", FWSignBBkp.Path).Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to delete FW Sign B backup: ", err)
		}
	}()

	s.Log("Copy backup files to the Host")
	FWSignADst, err := ioutil.TempFile("", "FWSignABackup")
	if err != nil {
		s.Fatal("Failed to create temporary file for firmware sign A backup")
	}
	defer os.Remove(FWSignADst.Name())
	defer FWSignADst.Close()

	FWSignBDst, err := ioutil.TempFile("", "FWSignBBackup")
	if err != nil {
		s.Fatal("Failed to create temporary file for firmware sign B backup")
	}
	defer os.Remove(FWSignBDst.Name())
	defer FWSignBDst.Close()

	if err := linuxssh.GetFile(ctx, s.DUT().Conn(), FWSignABkp.Path, FWSignADst.Name(), linuxssh.PreserveSymlinks); err != nil {
		s.Fatal("Failed to copy a FW A Sign backup to the Host")
	}
	if err := linuxssh.GetFile(ctx, s.DUT().Conn(), FWSignBBkp.Path, FWSignBDst.Name(), linuxssh.PreserveSymlinks); err != nil {
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

	s.Log("Corrupt Firmware A Sign")
	if _, err := h.BiosServiceClient.CorruptFWSection(ctx, &pb.CorruptSection{Section: pb.ImageSection_FWSignAImageSection, Programmer: pb.Programmer_BIOSProgrammer}); err != nil {
		s.Fatal("Failed to corrupt Firmware A Sign (VBOOTA) section: ", err)
	}

	s.Log("Corrupt Firmware B Sign")
	if _, err := h.BiosServiceClient.CorruptFWSection(ctx, &pb.CorruptSection{Section: pb.ImageSection_FWSignBImageSection, Programmer: pb.Programmer_BIOSProgrammer}); err != nil {
		s.Fatal("Failed to corrupt Firmware B Sign (VBOOTB) section: ", err)
	}

	// Restore FW Signatures
	defer func() {
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

		s.Log("Get back FW Signs backup from host to DUT")
		if _, err := linuxssh.PutFiles(ctx, s.DUT().Conn(), map[string]string{FWSignADst.Name(): FWSignABkp.Path, FWSignBDst.Name(): FWSignBBkp.Path}, linuxssh.PreserveSymlinks); err != nil {
			s.Fatal("Failed to get backup files to DUT from Host")
		}

		s.Log("Restore firmware signs")
		if _, err := h.BiosServiceClient.RestoreImageSection(ctx, FWSignABkp); err != nil {
			s.Fatal("Failed to restore FW Sign A: ", err)
		}
		if _, err := h.BiosServiceClient.RestoreImageSection(ctx, FWSignBBkp); err != nil {
			s.Fatal("Failed to restore FW Sign B: ", err)
		}

		if err := ms.ModeAwareReboot(ctx, firmware.WarmReset, firmware.AssumeRecoveryMode); err != nil {
			s.Fatal("Failed to perform mode aware reboot: ", err)
		}

		s.Log(ctx, "Reestablishing connection to DUT")
		connectCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
		if err := h.WaitConnect(connectCtx); err != nil {
			s.Fatal("Failed to reconnect to DUT after booting to recovery mode: ", err)
		}

		bootModeName := s.Param().(string)

		if mainFWType, err := h.Reporter.CrossystemParam(ctx, reporters.CrossystemParamMainfwType); err != nil {
			s.Fatal("Failed to get crossystem mainfw_type: ", err)
		} else if mainFWType != bootModeName {
			s.Fatalf("Failed to match mainfw_type: got %q, want %q", mainFWType, bootModeName)
		}

	}()

	s.Log("Copy TAST Files from DUT")
	if err := h.CopyTastFilesFromDUT(ctx); err != nil {
		s.Fatal(err, "copying Tast files from DUT to test server")
	}

	if err := h.DUT.Conn().CommandContext(ctx, "sync").Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to sync DUT: ", err)
	}

	if err := checkRecReason(ctx, h, ms); err != nil {
		s.Fatal("Failed when checking recovery reason: ", err)
	}

	s.Log("Set FW tries to B")
	if err := firmware.SetFWTries(ctx, h.DUT, fwCommon.RWSectionB, 0); err != nil {
		s.Fatal("Failed to set FW tries to B")
	}

	if err := checkRecReason(ctx, h, ms); err != nil {
		s.Fatal("Failed when checking recovery reason: ", err)
	}
}

func checkRecReason(ctx context.Context, h *firmware.Helper, ms *firmware.ModeSwitcher) error {
	testing.ContextLog(ctx, "Set the USB Mux direction to Host")
	if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxHost); err != nil {
		errors.Wrap(err, "failed to set the USB Mux direction to the Host")
	}

	// Test element required if rebooting from recovery to anything
	if err := h.Servo.WatchdogRemove(ctx, servo.WatchdogCCD); err != nil {
		errors.Wrap(err, "failed to remove watchdog for ccd")
	}

	testing.ContextLog(ctx, "Warm reboot with skiping waiting and sync")
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateWarmReset); err != nil {
		errors.Wrap(err, "failed to warm reset DUT")
	}

	testing.ContextLog(ctx, "Require a servo")
	if err := h.RequireServo(ctx); err != nil {
		errors.Wrap(err, "failed to init servo")
	}

	testing.ContextLog(ctx, "Close RPC Connection")
	if err := h.CloseRPCConnection(ctx); err != nil {
		errors.Wrap(err, "failed to close RPC connection")
	}

	// Recovery mode requires the DUT to boot the image on the USB.
	// Thus, the servo must show the USB to the DUT.
	testing.ContextLog(ctx, "Enable Recovery mode")
	if err := ms.EnableRecMode(ctx, servo.USBMuxDUT); err != nil {
		errors.Wrap(err, "failed to enable recovery mode")
	}

	testing.ContextLog(ctx, "Reestablishing connection to DUT")
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if err := h.WaitConnect(connectCtx); err != nil {
		errors.Wrap(err, "failed to reconnect to DUT after booting to recovery mode")
	}

	testing.ContextLog(ctx, "Expect recovery boot")
	if isRecovery, err := h.Reporter.CheckBootMode(ctx, fwCommon.BootModeRecovery); err != nil {
		errors.Wrap(err, "failed to check a boot mode")
	} else if !isRecovery {
		errors.New("failed to boot into the recovery mode")
	}

	testing.ContextLog(ctx, "Check recovery reason")
	if containsRecReason, err := h.Reporter.ContainsRecoveryReason(ctx, []reporters.RecoveryReason{reporters.RecoveryReasonROInvalidRW, reporters.RecoveryReasonRWVerifyKeyblock}); err != nil || !containsRecReason {
		errors.Wrap(err, "failed to get expected recovery reason")
	}

	return nil
}
