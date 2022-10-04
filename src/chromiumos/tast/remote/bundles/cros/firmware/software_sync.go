// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"bytes"
	"context"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	common "chromiumos/tast/common/firmware"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SoftwareSync,
		Desc:         "Servo based EC software sync test",
		Contacts:     []string{"jbettis@google.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		ServiceDeps:  []string{"tast.cros.firmware.BiosService"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Params: []testing.Param{
			{Name: "normal",
				Fixture: fixture.NormalMode,
			},
			{Name: "dev",
				Fixture: fixture.DevModeGBB,
			},
		},
	})
}

const hashCommand = "ectool echash | grep hash: | sed \"s/hash:\\s\\+//\""

func SoftwareSync(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Creating mode switcher: ", err)
	}

	// TODO(b/194910957): old test disables EC WP here

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}
	bs := h.BiosServiceClient

	old, err := bs.GetGBBFlags(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("initial GetGBBFlags failed: ", err)
	}

	if common.GBBFlagsContains(old, pb.GBBFlag_DISABLE_EC_SOFTWARE_SYNC) {
		s.Log("Clearing GBB flag DISABLE_EC_SOFTWARE_SYNC")
		req := pb.GBBFlagsState{Clear: []pb.GBBFlag{pb.GBBFlag_DISABLE_EC_SOFTWARE_SYNC}}

		if _, err := bs.ClearAndSetGBBFlags(ctx, &req); err != nil {
			s.Fatal("Failed to clear gbb flag: ", err)
		}
	}

	backup, err := bs.BackupImageSection(ctx, &pb.FWSectionInfo{Section: pb.ImageSection_ECRWImageSection, Programmer: pb.Programmer_ECProgrammer})
	if err != nil {
		s.Fatal("Could not backup EC firmware: ", err)
	}
	cleanupContext := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 60*time.Second)
	defer cancel()
	defer func(ctx context.Context) {
		if err := h.EnsureDUTBooted(ctx); err != nil {
			s.Fatal("Can't delete temp file, DUT is off: ", err)
		}
		s.Log("Deleting temp file")
		if err := h.DUT.Conn().CommandContext(ctx, "rm", "-f", backup.Path).Run(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to delete firmware backup: ", err)
		}
	}(cleanupContext)

	s.Log("Checking preconditions")
	// TODO(b/194910957): Old test checks that fw-a section does not have preamble flag PREAMBLE_USE_RO_NORMAL. this really needed?
	// TODO(b/194910957): Old test unlocks CCD, is this needed?

	// Reboot just in case the firmware version we backed up isn't the same one that software sync will restore.
	if err := ms.ModeAwareReboot(ctx, firmware.WarmReset); err != nil {
		s.Fatal("Failed to reboot: ", err)
	}
	h.CloseRPCConnection(ctx)
	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}
	bs = h.BiosServiceClient

	activeCopy, err := h.Servo.GetString(ctx, "ec_active_copy")
	if err != nil {
		s.Fatal("EC active copy failed: ", err)
	}
	if !strings.HasPrefix(activeCopy, "RW") {
		s.Fatalf("EC active copy incorrect, got %q want RW", activeCopy)
	}
	ecHashBefore, err := h.DUT.Conn().CommandContext(ctx, "sh", "-c", hashCommand).
		Output()
	if err != nil {
		s.Fatal("Failed to get ec hash: ", err)
	}
	ecSection := pb.ImageSection_ECRWImageSection
	if activeCopy == "RW_B" {
		ecSection = pb.ImageSection_ECRWBImageSection
	}
	s.Log("Corrupt the EC section: ", ecSection)
	defer func(ctx context.Context) {
		if err := h.EnsureDUTBooted(ctx); err != nil {
			s.Fatal("Can't restore firmware, DUT is off: ", err)
		}
		if err := h.RequireBiosServiceClient(ctx); err != nil {
			s.Fatal("Requiring BiosServiceClient: ", err)
		}
		s.Log("Restoring EC firmware backup")
		if _, err := h.BiosServiceClient.RestoreImageSection(ctx, backup); err != nil {
			s.Fatal("Failed to restore EC firmware: ", err)
		}
		// Reboot and check active copy after restore.
		if err := ms.ModeAwareReboot(ctx, firmware.WarmReset); err != nil {
			s.Fatal("Failed to reboot: ", err)
		}
		activeCopy, err = h.Servo.GetString(ctx, "ec_active_copy")
		if err != nil {
			s.Fatal("EC active copy failed: ", err)
		}
		if !strings.HasPrefix(activeCopy, "RW") {
			s.Fatalf("EC active copy incorrect, got %q want RW", activeCopy)
		}
	}(cleanupContext)
	if _, err = bs.CorruptFWSection(ctx, &pb.FWSectionInfo{Section: ecSection, Programmer: pb.Programmer_ECProgrammer}); err != nil {
		s.Fatal("Failed to corrupt EC: ", err)
	}

	// Tell the EC to recalculate the hash
	if err := h.DUT.Conn().CommandContext(ctx, "ectool", "echash", "start", "rw").Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("EC hash start failed: ", err)
	}
	ecHashCorrupt, err := h.DUT.Conn().CommandContext(ctx, "sh", "-c", hashCommand).Output()
	if err != nil {
		s.Fatal("Failed to get ec hash: ", err)
	}
	if bytes.Equal(ecHashCorrupt, ecHashBefore) {
		s.Fatal("EC hash unchanged, corruption step failed")
	}

	s.Log("Reboot AP, check EC hash, and software sync it")
	if err := ms.ModeAwareReboot(ctx, firmware.WarmReset, firmware.WaitSoftwareSync); err != nil {
		s.Fatal("Failed to reboot: ", err)
	}
	h.CloseRPCConnection(ctx)

	s.Log("Expect EC in RW and RW is restored")
	ecHashAfter, err := h.DUT.Conn().CommandContext(ctx, "sh", "-c", hashCommand).
		Output(ssh.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get ec hash: ", err)
	}
	if !bytes.Equal(ecHashAfter, ecHashBefore) {
		s.Fatalf("EC hash wrong, got %s want %s", ecHashAfter, ecHashBefore)
	}
	activeCopy, err = h.Servo.GetString(ctx, "ec_active_copy")
	if err != nil {
		s.Fatal("EC active copy failed: ", err)
	}
	if !strings.HasPrefix(activeCopy, "RW") {
		s.Fatalf("EC active copy incorrect, got %q want RW", activeCopy)
	}

	if features, err := h.DUT.Conn().CommandContext(ctx, "ectool", "inventory").Output(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to get features: ", err)
	} else if bytes.Contains(features, []byte("\n38 ")) {
		s.Log("Checking for NORMAL boot mode")
		if err := h.Servo.CheckGSCBootMode(ctx, "NORMAL"); err != nil {
			s.Fatal("Incorrect boot mode: ", err)
		}

		s.Log("Corrupting ECRW hashcode in TPM kernel NV index")
		if err := h.Servo.RunCR50Command(ctx, "ec_comm corrupt"); err != nil {
			s.Fatal("Failed to corrupt ECRW hashcode: ", err)
		}
		s.Log("Reboot EC, verify RO, reboot AP, check hash")
		if err := ms.ModeAwareReboot(ctx, firmware.APOff, firmware.VerifyECRO, firmware.VerifyGSCNoBoot, firmware.WaitSoftwareSync); err != nil {
			s.Fatal("Failed to reboot: ", err)
		}
		h.CloseRPCConnection(ctx)

		s.Log("Checking for NORMAL boot mode")
		if err := h.Servo.CheckGSCBootMode(ctx, "NORMAL"); err != nil {
			s.Fatal("Incorrect boot mode: ", err)
		}
		s.Log("Expect EC in RW and RW is restored")
		ecHashAfter, err := h.DUT.Conn().CommandContext(ctx, "sh", "-c", hashCommand).
			Output()
		if err != nil {
			s.Fatal("Failed to get ec hash: ", err)
		}
		if !bytes.Equal(ecHashAfter, ecHashBefore) {
			s.Fatalf("EC hash wrong, got %s want %s", ecHashAfter, ecHashBefore)
		}
		activeCopy, err = h.Servo.GetString(ctx, "ec_active_copy")
		if err != nil {
			s.Fatal("EC active copy failed: ", err)
		}
		if !strings.HasPrefix(activeCopy, "RW") {
			s.Fatalf("EC active copy incorrect, got %q want RW", activeCopy)
		}
	}
}
