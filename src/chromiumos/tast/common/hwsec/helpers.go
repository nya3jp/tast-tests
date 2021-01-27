// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

/*
This file implements miscellaneous and unsorted helpers.
*/

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// CmdRunner declares interface that runs command on DUT.
type CmdRunner interface {
	Run(ctx context.Context, cmd string, args ...string) ([]byte, error)
}

// Helper provides various helper functions that could be shared across all
// hwsec integration test regardless of run-type, i.e., remote or local.
type Helper struct {
	cmdRunner      CmdRunner
	cryptohomeUtil *UtilityCryptohomeBinary
	tpmManagerUtil *UtilityTpmManagerBinary
}

// NewHelper creates a new Helper, with r responsible for CmdRunner.
func NewHelper(r CmdRunner) (*Helper, error) {
	cryptohomeUtil, err := NewUtilityCryptohomeBinary(r)
	if err != nil {
		return nil, err
	}
	tpmManagerUtil, err := NewUtilityTpmManagerBinary(r)
	if err != nil {
		return nil, err
	}
	return &Helper{
		cmdRunner:      r,
		cryptohomeUtil: cryptohomeUtil,
		tpmManagerUtil: tpmManagerUtil,
	}, nil
}

// CmdRunner exposes the cmdRunner of helper
func (h *Helper) CmdRunner() CmdRunner { return h.cmdRunner }

// CryptohomeUtil exposes the cryptohomeUtil of helper
func (h *Helper) CryptohomeUtil() *UtilityCryptohomeBinary { return h.cryptohomeUtil }

// TPMManagerUtil exposes the tpmManagerUtil of helper
func (h *Helper) TPMManagerUtil() *UtilityTpmManagerBinary { return h.tpmManagerUtil }

// EnsureTPMIsReady ensures the TPM is ready when the function returns |nil|.
// Otherwise, returns any encountered error.
func (h *Helper) EnsureTPMIsReady(ctx context.Context, timeout time.Duration) error {
	isReady, err := h.cryptohomeUtil.IsTPMReady(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to ensure ownership due to error in |IsTPMReady|")
	}
	if !isReady {
		result, err := h.cryptohomeUtil.EnsureOwnership(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to ensure ownership due to error in |TakeOwnership|")
		}
		if !result {
			return errors.New("failed to take ownership")
		}
	}
	return testing.Poll(ctx, func(context.Context) error {
		isReady, err := h.cryptohomeUtil.IsTPMReady(ctx)
		if err != nil {
			return errors.New("error during checking TPM readiness")
		}
		if isReady {
			return nil
		}
		return errors.New("haven't confirmed to be owned")
	}, &testing.PollOptions{
		Timeout:  timeout,
		Interval: PollingInterval,
	})
}

// EnsureIsPreparedForEnrollment ensures the DUT is prepareed for enrollment
// when the function returns |nil|. Otherwise, returns any encountered error.
func (h *Helper) EnsureIsPreparedForEnrollment(ctx context.Context, timeout time.Duration) error {
	return testing.Poll(ctx, func(context.Context) error {
		// intentionally ignores error; retry the operation until timeout.
		isPrepared, err := h.cryptohomeUtil.IsPreparedForEnrollment(ctx)
		if err != nil {
			return err
		}
		if !isPrepared {
			return errors.New("not prepared yet")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  timeout,
		Interval: PollingInterval,
	})
}
