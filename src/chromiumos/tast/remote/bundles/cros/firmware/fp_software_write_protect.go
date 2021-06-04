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
		Func: FpSoftwareWriteProtect,
		Desc: "Verify that software write protect cannot be disabled when hardware write protect is enabled",
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

func FpSoftwareWriteProtect(ctx context.Context, s *testing.State) {
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

	testing.ContextLog(ctx, "Rebooting into RO image")
	if err := fingerprint.RebootFpmcu(ctx, d, fingerprint.ImageTypeRO); err != nil {
		s.Fatal("Failed to reboot into RO image: ", err)
	}

	testing.ContextLog(ctx, "Validating that we're now running the RO image")
	if err := fingerprint.CheckRunningFirmwareCopy(ctx, d, fingerprint.ImageTypeRO); err != nil {
		s.Fatal("Not running RO image: ", err)
	}

	testing.ContextLog(ctx, "Validating flash protection hasn't changed")
	if err := fingerprint.CheckWriteProtectStateCorrect(ctx, d, t.FPBoard(), fingerprint.ImageTypeRO, true, true); err != nil {
		s.Fatal("Incorrect write protect state: ", err)
	}

	testing.ContextLog(ctx, "Disabling software write protect when hardware write protect is enabled when running RO")
	if err := fingerprint.EctoolCommand(ctx, d, "flashprotect", "disable").Run(ctx); err == nil {
		s.Fatal("Disabling software write protect should fail")
	}

	testing.ContextLog(ctx, "Validating flash protection hasn't changed")
	if err := fingerprint.CheckWriteProtectStateCorrect(ctx, d, t.FPBoard(), fingerprint.ImageTypeRO, true, true); err != nil {
		s.Fatal("Incorrect write protect state: ", err)
	}

	testing.ContextLog(ctx, "Rebooting into RW image")
	if err := fingerprint.RebootFpmcu(ctx, d, fingerprint.ImageTypeRW); err != nil {
		s.Fatal("Failed to reboot into RW image: ", err)
	}

	testing.ContextLog(ctx, "Validating that we're now running the RW image")
	if err := fingerprint.CheckRunningFirmwareCopy(ctx, d, fingerprint.ImageTypeRW); err != nil {
		s.Fatal("Not running RW image: ", err)
	}

	testing.ContextLog(ctx, "Validating flash protection hasn't changed")
	if err := fingerprint.CheckWriteProtectStateCorrect(ctx, d, t.FPBoard(), fingerprint.ImageTypeRW, true, true); err != nil {
		s.Fatal("Incorrect write protect state: ", err)
	}

	testing.ContextLog(ctx, "Disabling software write protect when hardware write protect is enabled when running RW")
	if err := fingerprint.EctoolCommand(ctx, d, "flashprotect", "disable").Run(ctx); err == nil {
		s.Fatal("Disabling software write protect should fail")
	}

	testing.ContextLog(ctx, "Validating flash protection hasn't changed")
	if err := fingerprint.CheckWriteProtectStateCorrect(ctx, d, t.FPBoard(), fingerprint.ImageTypeRW, true, true); err != nil {
		s.Fatal("Incorrect write protect state: ", err)
	}
}
