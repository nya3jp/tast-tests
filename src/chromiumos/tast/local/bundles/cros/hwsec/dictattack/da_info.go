// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dictattack hosts common code that is used by dictionary attack reset test for TPMv1.2 and TPMv2.0.
package dictattack

import (
	"context"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
)

// DAInfo calls GetDAInfo from both tpm_manager and cryptohome and sees if they match.
func DAInfo(ctx context.Context, cryptohomeUtil *hwsec.UtilityCryptohomeBinary, tpmManagerUtil *hwsec.UtilityTpmManagerBinary) (info *hwsec.DAInfo, returnedError error) {
	infoFromTpmManager, err := tpmManagerUtil.GetDAInfo(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get dictionary attack info from TpmManager")
	}

	infoFromCryptohome, err := cryptohomeUtil.GetDAInfo(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get dictionary attack info from cryptohome")
	}

	// Now check the values.
	if infoFromCryptohome.Counter != infoFromTpmManager.Counter {
		return nil, errors.Errorf("cryptohome (%d) and tpm_manager (%d) disagree on counter value", infoFromCryptohome.Counter, infoFromTpmManager.Counter)
	}
	if infoFromCryptohome.Threshold != infoFromTpmManager.Threshold {
		return nil, errors.Errorf("cryptohome (%d) and tpm_manager (%d) disagree on threshold value", infoFromCryptohome.Threshold, infoFromTpmManager.Threshold)
	}
	if infoFromCryptohome.InEffect != infoFromTpmManager.InEffect {
		return nil, errors.Errorf("cryptohome (%t) and tpm_manager (%t) disagree on in effect value", infoFromCryptohome.InEffect, infoFromTpmManager.InEffect)
	}
	if infoFromCryptohome.Remaining != infoFromTpmManager.Remaining {
		return nil, errors.Errorf("cryptohome (%d) and tpm_manager (%d) disagree on remaining value", infoFromCryptohome.Remaining, infoFromTpmManager.Remaining)
	}

	return infoFromCryptohome, nil
}
