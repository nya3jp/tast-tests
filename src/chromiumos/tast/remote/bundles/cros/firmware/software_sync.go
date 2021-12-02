// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"

	common "chromiumos/tast/common/firmware"
	pb "chromiumos/tast/services/cros/firmware"

	"github.com/golang/protobuf/ptypes/empty"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SoftwareSync,
		Desc:         "",
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

func SoftwareSync(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	// ms, err := firmware.NewModeSwitcher(ctx, h)
	// if err != nil {
	// 	s.Fatal("Creating mode switcher: ", err)
	// }

	// TODO Disable EC WP

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}
	bs := h.BiosServiceClient

	old, err := bs.GetGBBFlags(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("initial GetGBBFlags failed: ", err)
	}

	if common.GBBFlagsContains(*old, pb.GBBFlag_DISABLE_EC_SOFTWARE_SYNC) {
		s.Log("Clearing GBB flag DISABLE_EC_SOFTWARE_SYNC")
		req := pb.GBBFlagsState{Clear: []pb.GBBFlag{pb.GBBFlag_DISABLE_EC_SOFTWARE_SYNC}}

		if _, err := bs.ClearAndSetGBBFlags(ctx, &req); err != nil {
			s.Fatal("Failed to clear gbb flag: ", err)
		}

		// if err := ms.ModeAwareReboot(ctx, firmware.WarmReset); err != nil {
		// 	s.Fatal("Failed to reboot: ", err)
		// }
		// h.CloseRPCConnection(ctx)
		// if err := h.RequireBiosServiceClient(ctx); err != nil {
		// 	s.Fatal("Requiring BiosServiceClient: ", err)
		// }
		// bs = h.BiosServiceClient
	}

	// TODO backup firmware
	backup, err := bs.BackupECRW(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Could not backup EC firmware: ", err)
	}
	cleanupContext := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()
	defer func(ctx context.Context) {
		if err := h.DUT.Conn().CommandContext(ctx, "rm", "-f", backup.Path).Run(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to delete firmware backup: ", err)
		}
	}(cleanupContext)
	defer func(ctx context.Context) {
		if _, err := bs.RestoreECRW(ctx, &pb.ECRWPath{Path: backup.Path}); err != nil {
			s.Fatal("Failed to restore EC firmware: ", err)
		}
	}(cleanupContext)

	// TODO Check that fw is in RW mode. Is this really needed?
	// TODO Unlock CCD

	// TODO Run gsctool -a -g and check for NORMAL
	output, err := h.Servo.RunCR50CommandGetOutput(ctx, "ec_comm", []string{`boot_mode\s*:\s*(NORMAL|NO_BOOT)`})
	if err != nil {
		s.Fatal("CR50 cmd failed: ", err)
	}
	s.Fatalf("TODO: %+t", output[0])
	// TODO run_test_corrupt_ec_rw
	// TODO run_test_corrupt_hash_in_cr50 if EC_FEATURE_EFS2
}
