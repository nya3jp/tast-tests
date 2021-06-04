// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/remote/firmware/fingerprint"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FpReadFlash,
		Desc: "Verify that fingerprint flash cannot be read",
		Contacts: []string{
			"tomhughes@chromium.org", // Test author
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

func FpReadFlash(ctx context.Context, s *testing.State) {
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

	if err := fingerprint.CheckRollbackSetToInitialValue(ctx, d); err != nil {
		s.Fatal("Failed to validate rollback state: ", err)
	}

	testing.ContextLog(ctx, "Reading from flash while running RW firmware should fail")
	if err := fingerprint.ReadFromRollbackFlash(ctx, d, t.FPBoard(), filepath.Join(t.DUTTempDir(), "test1.bin")); err == nil {
		s.Fatal("Should not be able to read from flash")
	}

	testing.ContextLog(ctx, "Reboot to RO")
	if err := fingerprint.RebootFpmcu(ctx, d, fingerprint.ImageTypeRO); err != nil {
		s.Fatal("Failed to reboot to RO: ", err)
	}

	testing.ContextLog(ctx, "Reading from flash while running RO firmware should fail")
	if err := fingerprint.ReadFromRollbackFlash(ctx, d, t.FPBoard(), filepath.Join(t.DUTTempDir(), "test2.bin")); err == nil {
		s.Fatal("Should not be able to read from flash")
	}

	testing.ContextLog(ctx, "Reboot to RW")
	if err := fingerprint.RebootFpmcu(ctx, d, fingerprint.ImageTypeRW); err != nil {
		s.Fatal("Failed to reboot to RW: ", err)
	}
}
