// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package diag is a library of functionality to utilize the native Chrome
// network diagnostic routines.
package diag

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/network/diag"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// MojoAPI is a struct that encapsulates a Network Diagnostics mojo remote.
// Functions are exposed to call the underlying diagnostics routines.
type MojoAPI struct {
	conn       *chrome.Conn
	mojoRemote *chrome.JSObject
}

// NewMojoAPI returns a MojoAPI object that is connected to a network
// diagnostics mojo remote instance on success, or an error.
func NewMojoAPI(ctx context.Context, conn *chrome.Conn) (*MojoAPI, error) {
	var mojoRemote chrome.JSObject
	if err := conn.Call(ctx, &mojoRemote, diag.NetDiagJs); err != nil {
		return nil, errors.Wrap(err, "failed to set up the network diagnostics mojo API")
	}

	return &MojoAPI{conn, &mojoRemote}, nil
}

// RunRoutine calls into the injected network diagnostics mojo API and returns a
// diag.RoutineResult on success, or an error.
func (m *MojoAPI) RunRoutine(ctx context.Context, routine string) (*diag.RoutineResult, error) {
	result := diag.RoutineResult{Verdict: diag.VerdictUnknown}
	jsWrap := fmt.Sprintf("function() { return this.%v() }", routine)
	if err := m.mojoRemote.Call(ctx, &result, jsWrap); err != nil {
		return nil, errors.Wrapf(err, "failed to run %v", routine)
	}

	switch result.Verdict {
	case diag.VerdictNoProblem, diag.VerdictProblem, diag.VerdictNotRun:
		return &result, nil
	}
	return nil, errors.Errorf("unknown routine verdict; got: %v", result.Verdict)
}

// PollRoutine will continuously run the specified routine until the provided
// diag.RoutineResult is matched.
func (m *MojoAPI) PollRoutine(ctx context.Context, routine string, expectedResult *diag.RoutineResult) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		result, err := m.RunRoutine(ctx, routine)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to run routine"))
		}

		if err := diag.CheckRoutineResult(result, expectedResult); err != nil {
			return err
		}

		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "timout waiting for routine to have expected results")
	}

	return nil
}

// Release frees the resources help by the internal MojoAPI components.
func (m *MojoAPI) Release(ctx context.Context) error {
	return m.mojoRemote.Release(ctx)
}
