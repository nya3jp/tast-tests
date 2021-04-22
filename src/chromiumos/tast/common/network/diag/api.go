// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package diag is a library of common functionality to utilize the native
// Chrome network diagnostic routines.
package diag

import (
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"chromiumos/tast/errors"
)

// NetDiagJs is a stringified JS file that exposes the network diagnostics mojo
// API.
// TODO(crbug/1127165): convert this to a data file when supported by fixtures.
const NetDiagJs = `
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

    async hasSecureWiFiConnection() {
      return await this.getNetworkDiagnostics().hasSecureWiFiConnection();
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

// RoutineVerdict represents the possible return values a diagnostic routine can
// have.
type RoutineVerdict int32

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
	Problems []uint32
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
	RoutineLanConnectivity      = "lanConnectivity"
	RoutineDNSResolverPresent   = "dnsResolverPresent"
	RoutineDNSResolution        = "dnsResolution"
	RoutineDNSLatency           = "dnsLatency"
	RoutineHTTPFirewall         = "httpFirewall"
	RoutineHTTPSFirewall        = "httpsFirewall"
	RoutineHTTPSLatency         = "httpsLatency"
	RoutineSignalStrength       = "signalStrength"
	RoutineSecureWiFiConnection = "hasSecureWiFiConnection"
	RoutineGatewayCanBePing     = "gatewayCanBePinged"
	RoutineCaptivePortal        = "captivePortal"
	RoutineVideoConferencing    = "videoConferencing"
)
