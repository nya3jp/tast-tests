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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
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

    async dnsResolution() {
      return await this.getNetworkDiagnostics().dnsResolution();
    },

    async dnsLatency() {
      return await this.getNetworkDiagnostics().dnsLatency();
    },

    async httpFirewall() {
      return await this.getNetworkDiagnostics().httpFirewall();
    },

    async httpsFirewall() {
      return await this.getNetworkDiagnostics().httpsFirewall();
    },

    async httpsLatency() {
      return await this.getNetworkDiagnostics().httpsLatency();
    },

    async signalStrength() {
      return await this.getNetworkDiagnostics().signalStrength();
    },

    async gatewayCanBePinged() {
      return await this.getNetworkDiagnostics().gatewayCanBePinged();
    },

    async captivePortal() {
      return await this.getNetworkDiagnostics().captivePortal();
    },

    async videoConferencing() {
      return await this.getNetworkDiagnostics().videoConferencing();
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

// CheckRoutineResult compares the routine result to the expected result. If
// they are not the same an error is returned.
func CheckRoutineResult(result, expectedResult *RoutineResult) error {
	if result.Verdict != expectedResult.Verdict {
		return errors.Errorf("expected routine verdict; got: %v, want: %v", result.Verdict, expectedResult.Verdict)
	}

	sortInts := cmpopts.SortSlices(func(a, b int) bool { return a < b })
	if !cmp.Equal(result.Problems, expectedResult.Problems, sortInts, cmpopts.EquateEmpty()) {
		return errors.Errorf("unexpected routine problems: got %v, want %v", result.Problems, expectedResult.Problems)
	}

	return nil
}

// List of network diagnostic routines
const (
	RoutineLanConnectivity    = "lanConnectivity"
	RoutineDNSResolverPresent = "dnsResolverPresent"
	RoutineDNSResolution      = "dnsResolution"
	RoutineDNSLatency         = "dnsLatency"
	RoutineHTTPFirewall       = "httpFirewall"
	RoutineHTTPSFirewall      = "httpsFirewall"
	RoutineHTTPSLatency       = "httpsLatency"
	RoutineSignalStrength     = "signalStrength"
	RoutineGatewayCanBePing   = "gatewayCanBePinged"
	RoutineCaptivePortal      = "captivePortal"
	RoutineVideoConferencing  = "videoConferencing"
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

// PollRoutine will continuously run the specified routine until the provided
// RoutineResult is matched.
func (m *MojoAPI) PollRoutine(ctx context.Context, routine string, expectedResult *RoutineResult) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		result, err := m.RunRoutine(ctx, routine)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to run routine"))
		}

		if err := CheckRoutineResult(result, expectedResult); err != nil {
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
