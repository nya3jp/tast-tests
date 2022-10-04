// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     FWCorruptRecoveryCache,
		Desc:     "Corrupt recovery cache and then check it's rebuilt",
		Contacts: []string{"js@semihalf.com", "chromeos-firmware@google.com"},
		// TODO(b/194907751): Add back to firmware_unstable once this test actually works.
		Attr:        []string{},
		Fixture:     fixture.DevModeGBB,
		Timeout:     20 * time.Minute,
		ServiceDeps: []string{"tast.cros.firmware.BiosService"},
	})
}

// FWCorruptRecoveryCache checks for RECOVERY_MRC_CACHE, voids it
// and then goes back to recovery mode to see if it's rebuilt again.
func FWCorruptRecoveryCache(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Failed to require BiosServiceClient: ", err)
	}

	s.Log("Checking for RECOVERY_MRC_CACHE region in AP firmware")

	// If flashrom can read the section, that means it exists.
	err := h.DUT.Conn().CommandContext(ctx, "flashrom", "-p", "host", "-r", "-i", "RECOVERY_MRC_CACHE:/dev/null").Run()
	if err != nil {
		s.Fatal("Cannot find RECOVERY_MRC_CACHE section in AP firmware: ", err)
	}

	s.Log("Backing up current RECOVERY_MRC_CACHE section for safety")
	rmcPath, err := h.BiosServiceClient.BackupImageSection(ctx, &pb.FWSectionInfo{
		Programmer: pb.Programmer_BIOSProgrammer,
		Section:    pb.ImageSection_RECOVERYMRCCACHEImageSection,
	})
	if err != nil {
		s.Fatal("Failed to backup current RECOVERY_MRC_CACHE region: ", err)
	}
	s.Log("RECOVERY_MRC_CACHE region backup is stored at: ", rmcPath.Path)

	defer func(ctx context.Context) {
		s.Log("Wait for DUT to reconnect")
		if err = h.DUT.WaitConnect(ctx); err != nil {
			s.Fatal("Failed to reconnect to DUT: ", err)
		}

		s.Log("Reconnecting to RPC services on DUT")
		if err := h.RequireRPCClient(ctx); err != nil {
			s.Fatal("Failed to reconnect to the RPC service on DUT: ", err)
		}

		s.Log("Reconnecting to BiosService on DUT")
		if err := h.RequireBiosServiceClient(ctx); err != nil {
			s.Fatal("Failed to reconnect to BiosServiceClient on DUT: ", err)
		}

		if err := h.EnsureDUTBooted(ctx); err != nil {
			s.Fatal("Failed to ensure the DUT is booted: ", err)
		}

		s.Log("Restoring RECOVERY_MRC_CACHE image")
		if _, err := h.BiosServiceClient.RestoreImageSection(ctx, rmcPath); err != nil {
			s.Error("Failed to restore MRC_RECOVERY_CACHE image: ", err)
		}

		s.Log("Removing RECOVERY_MRC_CACHE image backups from DUT")
		if _, err := h.DUT.Conn().CommandContext(ctx, "rm", rmcPath.Path).Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to delete RECOVERY_MRC_CACHE image from DUT: ", err)
		}
	}(ctx)

	// Shorten the deadline for everything to save some time for the restore.
	ctx, cancel := ctxutil.Shorten(ctx, 60*time.Second)
	defer cancel()

	s.Log("Corrupting RECOVERY_MRC_CACHE section")
	if _, err := h.BiosServiceClient.CorruptFWSection(ctx,
		&pb.FWSectionInfo{
			Section:    pb.ImageSection_RECOVERYMRCCACHEImageSection,
			Programmer: pb.Programmer_BIOSProgrammer,
		}); err != nil {
		s.Fatal("Failed to corrupt RECOVERY_MRC_CACHE section: ", err)
	}

	s.Log("Rebooting into recovery mode to rebuild RECOVERY_MRC_CACHE")
	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Failed to create new boot mode switcher: ", err)
	}

	if err := ms.RebootToMode(ctx, fwCommon.BootModeRecovery); err != nil {
		s.Fatal("Failed to reboot into recovery mode: ", err)
	}

	s.Log("Reconnecting to DUT")
	if err := h.WaitConnect(ctx); err != nil {
		s.Fatal("Failed to reconnect to DUT: ", err)
	}
	s.Log("Reconnected to DUT")

	s.Log("Checking if recovery MRC cache has been rebuilt")
	const cbmemCheckCommand = `cbmem -1 | grep "'RECOVERY_MRC_CACHE' needs update."`
	if err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", cbmemCheckCommand).Run(); err != nil {
		s.Fatal("Recovery MRC cache rebuilt check failed: ", err)
	}
}
