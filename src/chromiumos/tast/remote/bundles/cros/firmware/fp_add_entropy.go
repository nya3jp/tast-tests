// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/remote/firmware/fingerprint"
	"chromiumos/tast/remote/firmware/fingerprint/rpcdut"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FpAddEntropy,
		Desc: "Validate adding entropy only succeeds when running RO",
		Contacts: []string{
			"tomhughes@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      7 * time.Minute,
		SoftwareDeps: []string{"biometrics_daemon"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
		ServiceDeps:  []string{"tast.cros.platform.UpstartService", dutfs.ServiceName},
		Vars:         []string{"servo"},
	})
}

func FpAddEntropy(ctx context.Context, s *testing.State) {
	d, err := rpcdut.NewRPCDUT(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect RPCDUT: ", err)
	}
	defer d.Close(ctx)

	servoSpec, ok := s.Var("servo")
	if !ok {
		servoSpec = ""
	}
	t, err := fingerprint.NewFirmwareTest(ctx, d, servoSpec, s.OutDir(), true, true)
	if err != nil {
		s.Fatal("Failed to create new firmware test: ", err)
	}
	ctxForCleanup := ctx
	defer func() {
		if err := t.Close(ctxForCleanup); err != nil {
			s.Fatal("Failed to clean up: ", err)
		}
	}()
	ctx, cancel := ctxutil.Shorten(ctx, t.CleanupTime())
	defer cancel()

	firmwareCopy, err := fingerprint.RunningFirmwareCopy(ctx, d.DUT())
	if err != nil {
		s.Fatal("Failed to query running firmware copy: ", err)
	}
	if firmwareCopy != fingerprint.ImageTypeRW {
		s.Fatal("Not running RW firmware")
	}

	testing.ContextLog(ctx, "Validating initial rollback info")
	rollbackPrev, err := fingerprint.RollbackInfo(ctx, d.DUT())
	if err != nil {
		s.Fatal("Failed to get rollbackinfo: ", err)
	}
	if rollbackPrev.IsAntiRollbackSet() {
		s.Fatalf("Anti-rollback is set: %+v", rollbackPrev)
	}
	if !rollbackPrev.IsEntropySet() {
		s.Fatalf("Entropy is unset: %+v", rollbackPrev)
	}

	testing.ContextLog(ctx, "Adding entropy should fail when running RW")
	if err := fingerprint.AddEntropy(ctx, d.DUT(), false); err == nil {
		s.Fatal("Adding entropy succeeded when running RW")
	}

	testing.ContextLog(ctx, "Validating rollback didn't change")
	rollbackCur, err := fingerprint.RollbackInfo(ctx, d.DUT())
	if err != nil {
		s.Fatal("Failed to get rollbackinfo: ", err)
	}
	if rollbackPrev != rollbackCur {
		s.Fatalf("Rollback changed when adding entropy from RW: got %+v; want %+v",
			rollbackCur, rollbackPrev)
	}

	testing.ContextLog(ctx, "Adding entropy from RO should succeed")
	if err := fingerprint.RebootFpmcu(ctx, d.DUT(), fingerprint.ImageTypeRO); err != nil {
		s.Fatal("Failed to reboot to RO: ", err)
	}
	if err := fingerprint.AddEntropy(ctx, d.DUT(), false); err != nil {
		s.Fatal("Failed to add entropy: ", err)
	}
	testing.ContextLog(ctx, "Validating Block ID increases by 1, but nothing else")
	rollbackPrev = rollbackCur
	rollbackCur, err = fingerprint.RollbackInfo(ctx, d.DUT())
	if err != nil {
		s.Fatal("Failed to get rollbackinfo: ", err)
	}
	if rollbackCur.IsAntiRollbackSet() {
		s.Fatalf("Anti-rollback is set: %+v", rollbackPrev)
	}
	if expectedBlockID := rollbackPrev.BlockID + 1; expectedBlockID != rollbackCur.BlockID {
		s.Fatalf("Unexpected Rollback Block ID: got %d; want %d",
			rollbackCur.BlockID, expectedBlockID)
	}

	testing.ContextLog(ctx, "Adding entropy with reset (double write) from RO should succeed")
	if err := fingerprint.RebootFpmcu(ctx, d.DUT(), fingerprint.ImageTypeRO); err != nil {
		s.Fatal("Failed to reboot to RO: ", err)
	}
	if err := fingerprint.AddEntropy(ctx, d.DUT(), true); err != nil {
		s.Fatal("Failed to add entropy: ", err)
	}
	testing.ContextLog(ctx, "Validating Block ID increases by 2, but nothing else")
	rollbackPrev = rollbackCur
	rollbackCur, err = fingerprint.RollbackInfo(ctx, d.DUT())
	if err != nil {
		s.Fatal("Failed to get rollbackinfo: ", err)
	}
	if rollbackCur.IsAntiRollbackSet() {
		s.Fatalf("Anti-rollback is set: %+v", rollbackPrev)
	}
	if expectedBlockID := rollbackPrev.BlockID + 2; expectedBlockID != rollbackCur.BlockID {
		s.Fatalf("Unexpected Rollback Block ID: got %d; want %d",
			rollbackCur.BlockID, expectedBlockID)
	}

	testing.ContextLog(ctx, "Switching back to RW")
	if err := fingerprint.RebootFpmcu(ctx, d.DUT(), fingerprint.ImageTypeRW); err != nil {
		s.Fatal("Failed to reboot to RW: ", err)
	}
	testing.ContextLog(ctx, "Validating nothing changed")
	rollbackPrev = rollbackCur
	rollbackCur, err = fingerprint.RollbackInfo(ctx, d.DUT())
	if err != nil {
		s.Fatal("Failed to get rollbackinfo: ", err)
	}
	if rollbackPrev != rollbackCur {
		s.Fatalf("Rollback changed when rebooting to RW: got %+v; want %+v",
			rollbackCur, rollbackPrev)
	}

}
