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
	RunShell(ctx context.Context, cmd string) ([]byte, error)
}

// Helper provides various helper functions that could be shared across all
// hwsec integration test regardless of run-type, i.e., remote or local.
type Helper struct {
	ti TpmInitializer
}

// NewHelper creates a new Helper, with ti responsible for TPM initialization.
func NewHelper(ti TpmInitializer) *Helper {
	return &Helper{ti}
}

// RunShell runs command in a new shell; the underlying implementation is provided by |r|.
func RunShell(ctx context.Context, r CmdRunner, cmd string) ([]byte, error) {
	return r.Run(ctx, "sh", "-c", cmd)
}

// TpmInitializer is a collection of TPM-initialiaztion-related functions.
type TpmInitializer interface {
	// IsTPMReady checks if currently TPM is owned.
	IsTPMReady(ctx context.Context) (bool, error)
	// EnsureOwnership() owns TPM when it's not owned yet.
	EnsureOwnership(ctx context.Context) (bool, error)
	// IsPreparedForEnrollment checks is currently attestation is prepared for enrollment.
	IsPreparedForEnrollment(ctx context.Context) (bool, error)
}

// EnsureTPMIsReady ensures the TPM is ready when the function returns |nil|.
// Otherwise, returns any encountered error, including timeout after
// |timeoutInMs|.
func (h *Helper) EnsureTPMIsReady(ctx context.Context, timeoutInMs int) error {
	isReady, err := h.ti.IsTPMReady(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to ensure ownership due to error in |IsTPMReady|")
	}
	if isReady == false {
		result, err := h.ti.EnsureOwnership(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to ensure ownership due to error in |TakeOwnership|")
		}
		if result == false {
			return errors.New("failed to take ownership")
		}
	}
	return testing.Poll(ctx, func(context.Context) error {
		isReady, _ := h.ti.IsTPMReady(ctx)
		if isReady {
			return nil
		}
		return errors.New("haven't confirmed to be owned")
	}, &testing.PollOptions{
		Timeout:  time.Duration(timeoutInMs) * time.Millisecond,
		Interval: time.Duration(pollingIntervalMillis) * time.Millisecond,
	})
}

// EnsureIsPreparedForEnrollment ensures the DUT is prepareed for enrollment
// when the function returns |nil|. Otherwise, returns any encountered error,
// including timeout after |timeoutInMs|.
func (h *Helper) EnsureIsPreparedForEnrollment(ctx context.Context, timeoutInMs int) error {
	return testing.Poll(ctx, func(context.Context) error {
		// intentionally ignores error; retry the operation until timeout.
		isPrepared, err := h.ti.IsPreparedForEnrollment(ctx)
		if err != nil {
			return err
		}
		if !isPrepared {
			return errors.New("not prepared yet")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  time.Duration(timeoutInMs) * time.Millisecond,
		Interval: time.Duration(pollingIntervalMillis) * time.Millisecond,
	})
}
