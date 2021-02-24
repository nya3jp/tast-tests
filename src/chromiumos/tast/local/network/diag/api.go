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
	"chromiumos/tast/local/chrome/ui/conndiag"
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
func NewMojoAPI(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) (*MojoAPI, error) {
	app, err := conndiag.Launch(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch connectivity diagnostics app")
	}

	conn, err := app.ChromeConn(ctx, cr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get network diagnostics mojo")
	}

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
	VerdictUnknown RoutineVerdict = iota - 1
	// VerdictNoProblem means that the routine did not detect any problems.
	VerdictNoProblem
	// VerdictProblem means that the routine detected at least one problem.
	VerdictProblem
	// VerdictNotRun means that the routine was not executed.
	VerdictNotRun
)

// LanConnectivity runs the LanConnectivity network diagnostic routine. Returns
// the RoutineVerdict on success, or an error.
func (m *MojoAPI) LanConnectivity(ctx context.Context) (RoutineVerdict, error) {
	result := struct {
		Verdict int
	}{}
	// if err := m.mojoRemote.Call(ctx, &result, `function() { return this.lanConnectivity() }`); err != nil {
	if err := m.mojoRemote.Call(ctx, &result, `function() { return this.lanConnectivity() }`); err != nil {
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
func (m *MojoAPI) Release(ctx context.Context) []error {
	return []error{
		m.mojoRemote.Release(ctx),
		m.conn.Close(),
	}
}
