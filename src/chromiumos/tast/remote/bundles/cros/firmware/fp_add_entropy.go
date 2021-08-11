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
	servoSpec, _ := s.Var("servo")
	t, err := fingerprint.NewFirmwareTest(ctx, s.DUT(), servoSpec, s.RPCHint(), s.OutDir(), true, true)
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

	d := t.DUT()

	firmwareCopy, err := fingerprint.RunningFirmwareCopy(ctx, d)
	if err != nil {
		s.Fatal("Failed to query running firmware copy: ", err)
	}
	if firmwareCopy != fingerprint.ImageTypeRW {
		s.Fatal("Not running RW firmware")
	}

	testing.ContextLog(ctx, "Validating initial rollback info")
	rollbackPrev, err := fingerprint.RollbackInfo(ctx, d)
	if err != nil {
		s.Fatal("Failed to get rollbackinfo")
	}

	if !rollbackPrev.IsEntropySet() || rollbackPrev.IsAntiRollbackSet() {
		s.Fatal("Failed to validate rollback state: ", err)
	}

	testing.ContextLog(ctx, "Adding entropy should fail when running RW")
	if err := fingerprint.AddEntropy(ctx, d, false); err == nil {
		s.Fatal("Adding entropy succeeded when running RW")
	}

	testing.ContextLog(ctx, "Validating rollback didn't change")
	rollbackCur, err := fingerprint.RollbackInfo(ctx, d)
	if err != nil {
		s.Fatal("Failed to get rollbackinfo")
	}
	if rollbackPrev != rollbackCur {
		s.Fatal("Rollback changed when adding entropy from RW")
	}

	testing.ContextLog(ctx, "Adding entropy from RO should succeed")
	rollbackPrev = rollbackCur
	if err := fingerprint.RebootFpmcu(ctx, d, fingerprint.ImageTypeRO); err != nil {
		s.Fatal("Failed to reboot to RO: ", err)
	}
	_ = fingerprint.AddEntropy(ctx, d, false)
	testing.ContextLog(ctx, "Validating Block ID changes, but nothing else")
	rollbackCur, err = fingerprint.RollbackInfo(ctx, d)
	if err != nil {
		s.Fatal("Failed to get rollbackinfo")
	}
	if rollbackPrev.BlockID+1 != rollbackCur.BlockID ||
		rollbackPrev.MinVersion != rollbackCur.MinVersion ||
		rollbackPrev.RWVersion != rollbackCur.RWVersion {
		s.Fatal("Rollback block did not have the correct value")
	}

	testing.ContextLog(ctx, "Adding entropy with reset (double write) from RO should succeed")
	rollbackPrev = rollbackCur
	if err := fingerprint.RebootFpmcu(ctx, d, fingerprint.ImageTypeRO); err != nil {
		s.Fatal("Failed to reboot to RO: ", err)
	}
	_ = fingerprint.AddEntropy(ctx, d, true)
	testing.ContextLog(ctx, "Validating Block ID increases by 2, but nothing else")
	rollbackCur, err = fingerprint.RollbackInfo(ctx, d)
	if err != nil {
		s.Fatal("Failed to get rollbackinfo")
	}
	if rollbackPrev.BlockID+2 != rollbackCur.BlockID ||
		rollbackPrev.MinVersion != rollbackCur.MinVersion ||
		rollbackPrev.RWVersion != rollbackCur.RWVersion {
		s.Fatal("Rollback block did not have the correct value")
	}

	testing.ContextLog(ctx, "Switching back to RW")
	rollbackPrev = rollbackCur
	if err := fingerprint.RebootFpmcu(ctx, d, fingerprint.ImageTypeRW); err != nil {
		s.Fatal("Failed to reboot to RW: ", err)
	}
	testing.ContextLog(ctx, "Validating nothing changed")
	rollbackCur, err = fingerprint.RollbackInfo(ctx, d)
	if err != nil {
		s.Fatal("Failed to get rollbackinfo")
	}
	if rollbackPrev != rollbackCur {
		s.Fatal("Rollback changed when adding entropy from RW")
	}

}
