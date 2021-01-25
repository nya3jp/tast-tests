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

// DAInfo returns DAInfo only if the results from tpm_manager and cryptohome match.
func DAInfo(ctx context.Context, cryptohome *hwsec.UtilityCryptohomeBinary, tpmManager *hwsec.UtilityTPMManagerBinary) (*hwsec.DAInfo, error) {
	infoT, err := tpmManager.GetDAInfo(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get dictionary attack info from TPMManager")
	}

	infoC, err := cryptohome.GetDAInfo(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get dictionary attack info from cryptohome")
	}

	// Now check the values.
	if infoC.Counter != infoT.Counter {
		return nil, errors.Errorf("cryptohome (%d) and tpm_manager (%d) disagree on counter value", infoC.Counter, infoT.Counter)
	}
	if infoC.Threshold != infoT.Threshold {
		return nil, errors.Errorf("cryptohome (%d) and tpm_manager (%d) disagree on threshold value", infoC.Threshold, infoT.Threshold)
	}
	if infoC.InEffect != infoT.InEffect {
		return nil, errors.Errorf("cryptohome (%t) and tpm_manager (%t) disagree on in effect value", infoC.InEffect, infoT.InEffect)
	}
	if infoC.Remaining != infoT.Remaining {
		return nil, errors.Errorf("cryptohome (%d) and tpm_manager (%d) disagree on remaining value", infoC.Remaining, infoT.Remaining)
	}

	return infoC, nil
}
