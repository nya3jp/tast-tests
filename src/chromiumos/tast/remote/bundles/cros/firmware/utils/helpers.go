// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package utils contains functionality shared by tests that
// exercise firmware.
package utils

import (
	"context"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/testing"
)

// ChangeFWVariant checks if current FW variant (A/B) is equal to the fwVar, if not it switches to the fwVar
func ChangeFWVariant(ctx context.Context, h *firmware.Helper, ms *firmware.ModeSwitcher, fwVar fwCommon.RWSection) error {
	testing.ContextLogf(ctx, "Check the firmware version, looking for %q", fwVar)
	if isFWVerCorrect, err := h.Reporter.CheckFWVersion(ctx, string(fwVar)); err != nil {
		return errors.Wrap(err, "failed to check a firmware version")
	} else if !isFWVerCorrect {
		testing.ContextLogf(ctx, "Set FW tries to %q", fwVar)
		if err := firmware.SetFWTries(ctx, h.DUT, fwVar, 0); err != nil {
			return errors.Wrapf(err, "failed to set FW tries to %q", fwVar)
		}

		if err := ms.ModeAwareReboot(ctx, firmware.WarmReset); err != nil {
			return errors.Wrap(err, "failed to perform mode aware reboot")
		}

		testing.ContextLog(ctx, "Check the firmware version after reboot")
		if isFWVerCorrect, err := h.Reporter.CheckFWVersion(ctx, string(fwVar)); err != nil {
			return errors.Wrap(err, "failed to check a firmware version")
		} else if !isFWVerCorrect {
			return errors.New("failed to boot into the expected firmware version")
		}

		// Require again here since reboots in test cause nil pointer errors otherwise.
		if err := h.RequireBiosServiceClient(ctx); err != nil {
			return errors.Wrap(err, "requiring BiosServiceClient")
		}
	}
	return nil
}
