// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/firmware/fingerprint"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FpSystemIsLocked,
		Desc: "Verify that system_is_locked() is true in the firmware (i.e., CONFIG_CMD_FPSENSOR_DEBUG) is disabled",
		Contacts: []string{
			"tomhughes@chromium.org", // Test author
			"chromeos-fingerprint@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      7 * time.Minute,
		SoftwareDeps: []string{"biometrics_daemon"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
		ServiceDeps:  fingerprint.FirmwareTestServiceDeps,
		Vars:         []string{"servo"},
	})
}

func FpSystemIsLocked(ctx context.Context, s *testing.State) {
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

	testing.ContextLog(ctx, "Checking that firmware is functional")
	if _, err := fingerprint.CheckFirmwareIsFunctional(ctx, d); err != nil {
		s.Fatal("Firmware is not functional: ", err)
	}

	testing.ContextLog(ctx, "Checking that system is locked")
	if err := fingerprint.CheckSystemIsLocked(ctx, d); err != nil {
		s.Fatal("System is not locked: ", err)
	}

	testing.ContextLog(ctx, "Checking that we cannot access raw frame")
	if err := fingerprint.CheckRawFPFrameFails(ctx, d); err != nil {
		s.Fatal("Reading raw frame should fail: ", err)
	}
}
