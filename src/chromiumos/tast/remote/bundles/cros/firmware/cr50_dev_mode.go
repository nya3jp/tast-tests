// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Cr50DevMode,
		Desc:         "Verify cr50 can tell the state of the dev mode switch",
		Contacts:     []string{"tj@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		Fixture:      fixture.NormalMode,
		Timeout:      5 * time.Minute,
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

const (
	normalModeTPMValue string = ""
	devModeTPMValue    string = "dev_mode"
)

func Cr50DevMode(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servod")
	}
	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Creating mode switcher: ", err)
	}

	if err = checkCr50TPMInfo(ctx, h, normalModeTPMValue); err != nil {
		s.Fatal("Checking boot mode: ", err)
	}
	if err = ms.RebootToMode(ctx, fwCommon.BootModeDev); err != nil {
		s.Fatal("Failed to switch to dev mode: ", err)
	}
	if err = checkCr50TPMInfo(ctx, h, devModeTPMValue); err != nil {
		s.Fatal("Checking boot mode: ", err)
	}
	if err = ms.RebootToMode(ctx, fwCommon.BootModeNormal); err != nil {
		s.Fatal("Failed to switch to normal mode: ", err)
	}
	if err = checkCr50TPMInfo(ctx, h, normalModeTPMValue); err != nil {
		s.Fatal("Checking boot mode: ", err)
	}
}

// checkCr50TPMInfo parses the output of ccd command in cr50 console and verifies that TPM value matches the current boot mode
func checkCr50TPMInfo(ctx context.Context, h *firmware.Helper, expectedValue string) error {
	output, err := h.Servo.RunCR50CommandGetOutput(ctx, "ccd", []string{`TPM\s*:\s*(\S*)\s*\n`})
	if err != nil {
		return errors.Wrap(err, "failed to get boot mode info from cr50 CCD")
	}
	if output[0][1] != expectedValue {
		return errors.Wrapf(err, "incorrect boot mode info from cr50 CCD: got %q want %q", output[0][1], expectedValue)
	}
	testing.ContextLogf(ctx, "Boot mode info got from cr50 CCD matched successfully: %q", output[0][1])
	return nil
}
