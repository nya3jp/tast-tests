// Copyright 2022 The Chromium OS Authors. All rights reserved.
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

		// TODO(b/194910957): old test does warm reset, seems unneeded
	}

	backup, err := bs.BackupImageSection(ctx, &pb.FWBackUpSection{Section: pb.ImageSection_ECRWImageSection, Programmer: pb.Programmer_ECProgrammer})
	if err != nil {
		s.Fatal("Could not backup EC firmware: ", err)
	}
	cleanupContext := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()
	defer func(ctx context.Context) {
		s.Log("Deleting temp file")
		if err := h.DUT.Conn().CommandContext(ctx, "rm", "-f", backup.Path).Run(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to delete firmware backup: ", err)
		}
	}(cleanupContext)

	// TODO(b/194910957): Old test checks that fw-a section does not have preamble flag PREAMBLE_USE_RO_NORMAL. this really needed?
	// TODO(b/194910957): Old test unlocks CCD, is this needed?

	// Check CR50 boot mode is NORMAL
	output, err := h.Servo.RunCR50CommandGetOutput(ctx, "ec_comm", []string{`boot_mode\s*:\s*(\S+)\b`})
	if err != nil {
		s.Fatal("CR50 cmd failed: ", err)
	}
	if output[0][1] != "NORMAL" {
		s.Fatalf("CR50 boot mode incorrect, got %q want %q", output[0][1], "NORMAL")
	}

	s.Log("Corrupt EC firmware RW body")
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
		s.Log("Restoring EC firmware backup")
		if _, err := bs.RestoreImageSection(ctx, backup); err != nil {
			s.Fatal("Failed to restore EC firmware: ", err)
		}
	}(cleanupContext)
	if _, err = bs.CorruptFWSection(ctx, &pb.CorruptSection{Section: ecSection, Programmer: pb.Programmer_ECProgrammer}); err != nil {
		s.Fatal("Failed to corrupt EC: ", err)
	}

	s.Log("Reboot AP, check EC hash, and software sync it")
	if err := ms.ModeAwareReboot(ctx, firmware.WarmReset); err != nil {
		s.Fatal("Failed to reboot: ", err)
	}
	h.CloseRPCConnection(ctx)
	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}
	bs = h.BiosServiceClient

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

	// TODO(b/194910957): run_test_corrupt_hash_in_cr50 if EC_FEATURE_EFS2
}
