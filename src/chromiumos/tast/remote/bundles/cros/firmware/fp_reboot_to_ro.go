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
		Func: FpRebootToRO,
		Desc: "Validates that booting into RO fingerprint firmware succeeds",
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

func FpRebootToRO(ctx context.Context, s *testing.State) {
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

	s.Log("Rebooting into RO image")
	if err := fingerprint.RebootFpmcu(ctx, d, fingerprint.ImageTypeRO); err != nil {
		s.Fatal("Failed to reboot into RO image: ", err)
	}

	s.Log("Validating that we're now running the RO image")
	if err := fingerprint.CheckRunningFirmwareCopy(ctx, d, fingerprint.ImageTypeRO); err != nil {
		s.Fatal("Not running RO image: ", err)
	}

	s.Log("Validating flash protection hasn't changed")
	if err := fingerprint.CheckWriteProtectStateCorrect(ctx, d, t.FPBoard(), fingerprint.ImageTypeRO, true, true); err != nil {
		s.Fatal("Incorrect write protect state: ", err)
	}

	s.Log("Rebooting back into RW")
	if err := fingerprint.RebootFpmcu(ctx, d, fingerprint.ImageTypeRW); err != nil {
		s.Fatal("Failed to reboot into RW image: ", err)
	}

	s.Log("Validating we're now running RW version")
	if err := fingerprint.CheckRunningFirmwareCopy(ctx, d, fingerprint.ImageTypeRW); err != nil {
		s.Fatal("Not running RW image: ", err)
	}

	s.Log("Validating flash protection hasn't changed")
	if err := fingerprint.CheckWriteProtectStateCorrect(ctx, d, t.FPBoard(), fingerprint.ImageTypeRW, true, true); err != nil {
		s.Fatal("Incorrect write protect state: ", err)
	}
}
