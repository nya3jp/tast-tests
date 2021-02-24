// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package diag is a library of functionality to utilize the native Chrome
// network diagnostic routines.
package diag

import (
	"context"
	"fmt"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

// TODO(crbug/1127165): convert this to a data file when supported by fixtures.
const netDiagJs = `
/**
 * @fileoverview A wrapper file around the network diagnostics API.
 */
function() {
  return {
    /**
     * Network Diagnostics mojo remote.
     * @private {
     *     ?chromeos.networkDiagnostics.mojom.NetworkDiagnosticsRoutinesRemote}
     */
    networkDiagnostics_: null,

    getNetworkDiagnostics() {
      if (!this.networkDiagnostics_) {
        this.networkDiagnostics_ = chromeos.networkDiagnostics.mojom
                                       .NetworkDiagnosticsRoutines.getRemote()
      }
      return this.networkDiagnostics_;
    },

    async lanConnectivity() {
      return await this.getNetworkDiagnostics().lanConnectivity();
    },
  }
}
`

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
	if err := conn.Call(ctx, &mojoRemote, netDiagJs); err != nil {
		return nil, errors.Wrap(err, "failed to set up the network diagnostics mojo API")
	}

	return &MojoAPI{conn, &mojoRemote}, nil
}

// RoutineVerdict represents the possible return values a diagnostic routine can
// have.
type RoutineVerdict int

const (
	// VerdictUnknown is an unknown verdict.
	VerdictUnknown RoutineVerdict = -1
	// VerdictNoProblem means that the routine did not detect any problems.
	VerdictNoProblem = 0
	// VerdictProblem means that the routine detected at least one problem.
	VerdictProblem = 1
	// VerdictNotRun means that the routine was not executed.
	VerdictNotRun = 2
)

// jsWrapper takes a the name of a function that has been injected as in a
// Javascript object and returns a wrapped callable function as a string.
func jsWrapper(routine string) string {
	return fmt.Sprintf("function() { return this.%v() }", routine)
}

// LanConnectivity runs the LanConnectivity network diagnostic routine. Returns
// the RoutineVerdict on success, or an error.
func (m *MojoAPI) LanConnectivity(ctx context.Context) (RoutineVerdict, error) {
	result := struct {
		Verdict int
	}{Verdict: int(VerdictUnknown)}
	if err := m.mojoRemote.Call(ctx, &result, jsWrapper("lanConnectivity")); err != nil {
		return VerdictUnknown, errors.Wrap(err, "failed to run lanConnectivity test")
	}

	if result.Verdict == int(VerdictNoProblem) {
		return VerdictNoProblem, nil
	} else if result.Verdict == int(VerdictProblem) {
		return VerdictProblem, nil
	} else if result.Verdict == int(VerdictNotRun) {
		return VerdictNotRun, nil
	}

	return VerdictUnknown, errors.Errorf("unknown routine verdict. Got: %v", result.Verdict)
}

// Release frees the resources help by the internal MojoAPI components.
func (m *MojoAPI) Release(ctx context.Context) error {
	return m.mojoRemote.Release(ctx)
}
