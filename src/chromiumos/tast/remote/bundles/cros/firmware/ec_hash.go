// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"

	"chromiumos/tast/errors"
	"chromiumos/tast/common/firmware/bios"
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
		Pre:          pre.DevModeGBB(),
		Data:         pre.Data,
		ServiceDeps:  pre.ServiceDeps,
		SoftwareDeps: pre.SoftwareDeps,
		Vars:         pre.Vars,
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

func backupEC(ctx context.Context) (*bios.Image, error) {
	image, err := bios.NewImage(ctx, bios.ECRWImageSection, bios.ECProgrammer)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to backup EC firmware")
	}
	return image, nil
}

func restoreEC(ctx context.Context, image *bios.Image) error {
	err := image.WriteFlashrom(ctx, bios.ECRWImageSection, bios.ECProgrammer)
	if err != nil {
		return errors.Wrapf(err, "failed to restore EC firmware")
	}
	return nil
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

	ecImage, err := backupEC(ctx)
	if err != nil {
		s.Fatal("Failed to backup EC firmware: ", err)
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
		s.Log("Restoring EC image")
		if ecerr := restoreEC(ctx, ecImage); ecerr != nil {
			s.Fatal("Failed to restore EC image: ", ecerr )
		}
		s.Fatal("Failed to invalidate current EC hash: ", err)
	}
	invalidatedECHashMatch := ecHashRegexp.FindStringSubmatch(string(invalidatedECHashOutput))
	if len(invalidatedECHashMatch) < 2 || invalidatedECHashMatch[1] == "" {
		s.Log("Restoring EC image")
		if ecerr := restoreEC(ctx, ecImage); ecerr != nil {
			s.Fatal("Failed to restore EC image: ", ecerr )
		}
		s.Fatal("Failed to parse invalidated EC hash from ectool output")
	}
	invalidatedECHash := invalidatedECHashMatch[1]
	s.Log("Invalidated EC hash is: ", invalidatedECHash)

	if invalidatedECHash == initialECHash {
		s.Log("Restoring EC image")
		if ecerr := restoreEC(ctx, ecImage); ecerr != nil {
			s.Fatal("Failed to restore EC image: ", ecerr )
		}
		s.Fatal("Invalidated EC hash is equal to initial EC hash!")
	}

	s.Log("Warm rebooting DUT to recalculate EC hash with AP...")
	if err := h.DUT.Reboot(ctx); err != nil {
		s.Log("Restoring EC image")
		if ecerr := restoreEC(ctx, ecImage); ecerr != nil {
			s.Fatal("Failed to restore EC image: ", ecerr )
		}
		s.Fatal("Failed rebooting DUT: ", err)
	}

	s.Log("Reading EC hash after reboot")
	newECHashOutput, err := h.DUT.Conn().CommandContext(ctx, "ectool", "echash").Output()
	if err != nil {
		s.Log("Restoring EC image")
		if ecerr := restoreEC(ctx, ecImage); ecerr != nil {
			s.Fatal("Failed to restore EC image: ", ecerr )
		}
		s.Fatal("Failed to retrieve current EC hash: ", err)
	}
	newECHashMatch := ecHashRegexp.FindStringSubmatch(string(newECHashOutput))
	if len(newECHashMatch) < 2 || newECHashMatch[1] == "" {
		s.Log("Restoring EC image")
		if ecerr := restoreEC(ctx, ecImage); ecerr != nil {
			s.Fatal("Failed to restore EC image: ", ecerr )
		}
		s.Fatal("Failed to parse current EC hash from ectool output")
	}
	newECHash := newECHashMatch[1]
	s.Log("After a reboot, current EC hash is: ", newECHash)

	if invalidatedECHash == newECHash {
		s.Log("Restoring EC image")
		if ecerr := restoreEC(ctx, ecImage); ecerr != nil {
			s.Fatal("Failed to restore EC image: ", ecerr )
		}
		s.Fatal("New EC hash is equal to invalidated EC hash!")
	}

	if initialECHash != newECHash {
		s.Log("Restoring EC image")
		if ecerr := restoreEC(ctx, ecImage); ecerr != nil {
			s.Fatal("Failed to restore EC image: ", ecerr )
		}
		s.Fatal("New EC hash does not match initial EC hash!")
	}

	if initialECHash == newECHash {
		s.Log("Restoring EC image")
		if ecerr := restoreEC(ctx, ecImage); ecerr != nil {
			s.Fatal("Failed to restore EC image: ", ecerr )
		}
		s.Log("Current EC hash matches initial EC hash")
	}

	s.Log("Restoring EC image")
	if ecerr := restoreEC(ctx, ecImage); ecerr != nil {
		s.Fatal("Failed to restore EC image: ", ecerr )
	}
}
