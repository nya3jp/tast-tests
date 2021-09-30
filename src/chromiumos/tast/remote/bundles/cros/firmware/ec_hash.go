// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	common "chromiumos/tast/common/firmware"
	pb "chromiumos/tast/services/cros/firmware"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/firmware/pre"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECHash,
		Desc:         "Basic check for EC hash validation",
		Contacts:     []string{"js@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		Pre:          pre.NormalMode(),
		Data:         pre.Data,
		ServiceDeps:  pre.ServiceDeps,
		SoftwareDeps: pre.SoftwareDeps,
		Vars:         pre.Vars,
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

// ECHash tries to invalidate the hash of EC firmware and then check
// if its get recomputed back again on warm reboot to ensure that AP
// correctly measures EC validity
func ECHash(ctx context.Context, s *testing.State) {
	ecHashRegexp := regexp.MustCompile(`([0-9a-f]{64})`)

	h := s.PreValue().(*pre.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}

	bs := h.BiosServiceClient

	s.Log("Backing up current EC_RW region for safety")
	ecPath, err := bs.BackupECRW(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to backup current EC_RW region: ", err)
	}
	s.Log("EC_RW region backup is stored at: ", ecPath.Path)

	defer func(ctx context.Context) {

		s.Log("Reconnecting to RPC services on DUT")
		// Reconnect to the RPC service on the DUT.
		if err := h.RequireRPCClient(ctx); err != nil {
			s.Fatal("Failed to reconnect to the RPC service on DUT: ", err)
		}

		s.Log("Reconnecting to BiosService on DUT")
		if err := h.RequireBiosServiceClient(ctx); err != nil {
			s.Fatal("Failed to reconnect to BiosServiceClient on DUT: ", err)
		}

		s.Log("Restoring EC image")
		if err := h.EnsureDUTBooted(ctx); err != nil {
			s.Fatal("Failed to ensure the DUT is booted!")
		}
		if _, err := bs.RestoreECRW(ctx, ecPath); err != nil {
			s.Fatal("Failed to restore EC image: ", err)
		}
		s.Log("Removing EC image backup from DUT")
		if err := h.DUT.Conn().CommandContext(ctx, "rm", ecPath.Path); err != nil {
			s.Fatal("Failed to delete EC image from DUT: ", err)
		}

	}(ctx)

	// Shorten the deadline for everything o save some time for the restore
	ctx, cancel := ctxutil.Shorten(ctx, 60*time.Second)
	defer cancel()

	// Set GBBFlag_DISABLE_EC_SOFTWARE_SYNC
	flg, err := bs.GetGBBFlags(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed getting current GBB flags")
	}

	common.GBBClearFlag(flg, pb.GBBFlag_DISABLE_EC_SOFTWARE_SYNC)
	if _, err := bs.ClearAndSetGBBFlags(ctx, flg); err != nil {
		s.Fatal("Failed setting DISABLE_EC_SOFTWARE_SYNC GBB flag")
	}

	s.Log("Reading current EC hash")
	initialECHashOutput, err := h.DUT.Conn().CommandContext(ctx, "ectool", "echash").Output()
	if err != nil {
		s.Fatal("Failed to retrieve current EC hash: ", err)
	}
	initialECHashMatch := ecHashRegexp.FindStringSubmatch(string(initialECHashOutput))
	if len(initialECHashMatch) < 2 || initialECHashMatch[1] == "" {
		s.Fatal("Failed to parse current EC hash from ectool output")
	}
	initialECHash := initialECHashMatch[1]
	s.Log("Current EC hash is: ", initialECHash)

	// Below this line, the EC_RW contents should be considered as
	// modified and every failure should lead to immediate restore
	// of EC firmware!
	s.Log("Invalidating current EC hash")
	invalidatedECHashOutput, err := h.DUT.Conn().CommandContext(ctx, "ectool", "echash", "recalc", "0", "4").Output()
	if err != nil {
		s.Fatal("Failed to invalidate current EC hash: ", err)
	}
	invalidatedECHashMatch := ecHashRegexp.FindStringSubmatch(string(invalidatedECHashOutput))
	if len(invalidatedECHashMatch) < 2 || invalidatedECHashMatch[1] == "" {
		s.Fatal("Failed to parse invalidated EC hash from ectool output")
	}
	invalidatedECHash := invalidatedECHashMatch[1]
	s.Log("Invalidated EC hash is: ", invalidatedECHash)

	if invalidatedECHash == initialECHash {
		s.Fatal("Invalidated EC hash is equal to initial EC hash!")
	}

	s.Log("Warm rebooting DUT to recalculate EC hash with AP...")
	if err := h.DUT.Reboot(ctx); err != nil {
		s.Fatal("Failed rebooting DUT: ", err)
	}

	s.Log("Reading EC hash after reboot")
	newECHashOutput, err := h.DUT.Conn().CommandContext(ctx, "ectool", "echash").Output()
	if err != nil {
		s.Fatal("Failed to retrieve current EC hash: ", err)
	}
	newECHashMatch := ecHashRegexp.FindStringSubmatch(string(newECHashOutput))
	if len(newECHashMatch) < 2 || newECHashMatch[1] == "" {
		s.Fatal("Failed to parse current EC hash from ectool output")
	}
	newECHash := newECHashMatch[1]
	s.Log("After a reboot, current EC hash is: ", newECHash)

	if invalidatedECHash == newECHash {
		s.Fatal("New EC hash is equal to invalidated EC hash!")
	}

	if initialECHash != newECHash {
		s.Fatal("New EC hash does not match initial EC hash!")
	}

	if initialECHash == newECHash {
		s.Log("Current EC hash matches initial EC hash")
	}

}
