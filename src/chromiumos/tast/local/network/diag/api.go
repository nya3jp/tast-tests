// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package diag is a library of functionality to utilize the native Chrome
// network diagnostic routines.
package diag

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

// MojoAPI is a struct that encapsulates a Network Diagnostics mojo remote.
// Functions are exposed to call the underlying diagnostics routines.
type MojoAPI struct {
	conn       *chrome.Conn
	mojoRemote *chrome.JSObject
}

// RoutineVerdict represents the possible return values a diagnostic routine can
// have.
type RoutineVerdict int

const (
	// VerdictUnknown is an unknown verdict.
	VerdictUnknown RoutineVerdict = -1
	// VerdictNoProblem means that the routine did not detect any problems.
	VerdictNoProblem RoutineVerdict = 0
	// VerdictProblem means that the routine detected at least one problem.
	VerdictProblem RoutineVerdict = 1
	// VerdictNotRun means that the routine was not executed.
	VerdictNotRun RoutineVerdict = 2
)

// LanConnectivity runs the LanConnectivity network diagnostic routine. Returns
// the RoutineVerdict on success, or an error.
func (m *MojoAPI) LanConnectivity(ctx context.Context) (RoutineVerdict, error) {
	result := struct {
		verdict int
	}{}
	if err := m.mojoRemote.Call(ctx, &result, `function() { return this.lanConnectivity() }`); err != nil {
		return VerdictUnknown, errors.Wrap(err, "failed to run lanConnectivity test")
	}

	if result.verdict == int(VerdictNoProblem) {
		return VerdictNoProblem, nil
	} else if result.verdict == int(VerdictProblem) {
		return VerdictProblem, nil
	} else if result.verdict == int(VerdictNotRun) {
		return VerdictNotRun, nil
	} else {
		return VerdictUnknown, errors.Errorf("unknown routine verdict. Got: %v", result.verdict)
	}
}

// Release frees the resources help by the internal MojoAPI components.
func (m *MojoAPI) Release(ctx context.Context) []error {
	return []error{
		m.mojoRemote.Release(ctx),
		m.conn.Close(),
	}
}
