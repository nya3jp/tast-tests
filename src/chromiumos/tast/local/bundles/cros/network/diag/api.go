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

    async dnsResolverPresent() {
      return await this.getNetworkDiagnostics().dnsResolverPresent();
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

// RoutineResult is a data structure to hold the result of running a network
// diagnostic routine.
type RoutineResult struct {
	Verdict  RoutineVerdict
	Problems []int
}

// CheckRoutineVerdict returns nil if the routine ran without problem, or
// returns an error.
func CheckRoutineVerdict(verdict RoutineVerdict) error {
	switch verdict {
	case VerdictProblem:
		return errors.New("routine detected a problem")
	case VerdictNotRun:
		return errors.New("routine did not run")
	case VerdictUnknown:
		return errors.New("unknown routine verdict")
	case VerdictNoProblem:
		return nil
	}

	return errors.Errorf("unexpected routine verdict: %v", verdict)
}

// List of network diagnostic routines
const (
	RoutineLanConnectivity    string = "lanConnectivity"
	RoutineDNSResolverPresent        = "dnsResolverPresent"
)

// RunRoutine calls into the injected network diagnostics mojo API and returns a
// RoutineResult on success, or an error.
func (m *MojoAPI) RunRoutine(ctx context.Context, routine string) (*RoutineResult, error) {
	result := RoutineResult{Verdict: VerdictUnknown}
	jsWrap := fmt.Sprintf("function() { return this.%v() }", routine)
	if err := m.mojoRemote.Call(ctx, &result, jsWrap); err != nil {
		return nil, errors.Wrapf(err, "failed to run %v", routine)
	}

	switch result.Verdict {
	case VerdictNoProblem, VerdictProblem, VerdictNotRun:
		return &result, nil
	}
	return nil, errors.Errorf("unknown routine verdict; got: %v", result.Verdict)
}

// Release frees the resources help by the internal MojoAPI components.
func (m *MojoAPI) Release(ctx context.Context) error {
	return m.mojoRemote.Release(ctx)
}
