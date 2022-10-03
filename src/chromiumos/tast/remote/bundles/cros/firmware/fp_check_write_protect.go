// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/fingerprint"
	"chromiumos/tast/remote/firmware/fingerprint/rpcdut"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FpCheckWriteProtect,
		Desc: "Validate that write protect signal is correctly reported by FPMCU",
		Contacts: []string{
			"patrykd@google.com", // Test author
			"chromeos-fingerprint@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      6 * time.Minute,
		SoftwareDeps: []string{"crossystem"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
		ServiceDeps:  []string{"tast.cros.platform.UpstartService", "tast.cros.baserpc.FileSystem"},
		Vars:         []string{"servo"},
	})
}

// FpCheckWriteProtect checks if changes in write protect signal are reflected
// in FPMCU. This is achieved by setting WP to some known state, checking if
// wpsw_cur value (from crossystem) reports WP state properly and check if
// WP state reported by FPMCU is also correct.
func FpCheckWriteProtect(ctx context.Context, s *testing.State) {
	d, err := rpcdut.NewRPCDUT(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect RPCDUT: ", err)
	}
	defer d.Close(ctx)

	servoSpec, ok := s.Var("servo")
	if !ok {
		servoSpec = ""
	}
	t, err := fingerprint.NewFirmwareTest(ctx, d, servoSpec, s.OutDir(), false, false)
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

	testing.ContextLog(ctx, "Checking if FPMCU respects disabled write protect")
	if err := testWriteProtect(ctx, t, false); err != nil {
		s.Fatal("Failed to test write protect signal with WP disabled: ", err)
	}

	testing.ContextLog(ctx, "Checking if FPMCU respects enabled write protect")
	if err := testWriteProtect(ctx, t, true); err != nil {
		s.Fatal("Failed to test write protect signal with WP enabled: ", err)
	}
}

func testWriteProtect(ctx context.Context, t *fingerprint.FirmwareTest, writeProtectEnabled bool) error {
	d := t.DUT()

	fwWpDesiredState := servo.FWWPStateOff
	if writeProtectEnabled {
		fwWpDesiredState = servo.FWWPStateOn
	}

	if err := t.Servo().Servo().SetFWWPState(ctx, fwWpDesiredState); err != nil {
		return errors.Wrapf(err, "failed to set hardware write protection to %s state", fwWpDesiredState)
	}

	r := reporters.New(d.DUT())
	csMap, err := r.Crossystem(ctx, reporters.CrossystemParamWpswCur)
	if err != nil {
		return errors.Wrapf(err, "failed to get %s value", reporters.CrossystemParamWpswCur)
	}

	wpState, err := strconv.ParseUint(csMap[reporters.CrossystemParamWpswCur], 10, 32)
	if err != nil {
		return errors.Wrapf(err, "unexpected crossystem value for %s: got %s; want uint", reporters.CrossystemParamWpswCur, csMap[reporters.CrossystemParamWpswCur])
	}

	testing.ContextLogf(ctx, "Write protect state reported by crossystem (%s) is %d", reporters.CrossystemParamWpswCur, wpState)
	if (wpState != 0) != writeProtectEnabled {
		return errors.Errorf("Write protect state reported by crossystem (%s = %d) doesn't match desired WP state", reporters.CrossystemParamWpswCur, wpState)
	}

	testing.ContextLog(ctx, "Validating that FPMCU write protect state is correct")
	fp, err := fingerprint.GetFlashProtect(ctx, d.DUT())
	if err != nil {
		return errors.Wrap(err, "failed to get FPMCU write protect state")
	}
	fpmcuWpState := fp.IsHardwareWriteProtected()

	testing.ContextLogf(ctx, "Write protect state reported by FPMCU is %t", fpmcuWpState)
	if fpmcuWpState != writeProtectEnabled {
		return errors.Errorf("Write protect state reported by FPMCU (%t) doesn't match desired WP state (%t)", fpmcuWpState, writeProtectEnabled)
	}

	return nil
}
