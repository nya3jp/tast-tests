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

    // Simplifies the response from the network diagnostic routines to be
    // easily converted to go structs.
    parseResult_(response) {
      const result = response.result;
      let ret = {Verdict: result.verdict};
      ret.Problems = Object.values(result.problems)[0];
      return ret;
    },

    async lanConnectivity() {
      return this.parseResult_(await this.getNetworkDiagnostics().runLanConnectivity());
    },

    async dnsResolverPresent() {
      return this.parseResult_(await this.getNetworkDiagnostics().runDnsResolverPresent());
    },

    async dnsResolution() {
      return this.parseResult_(await this.getNetworkDiagnostics().runDnsResolution());
    },

    async dnsLatency() {
      return this.parseResult_(await this.getNetworkDiagnostics().runDnsLatency());
    },

    async httpFirewall() {
      return this.parseResult_(await this.getNetworkDiagnostics().runHttpFirewall());
    },

    async httpsFirewall() {
      return this.parseResult_(await this.getNetworkDiagnostics().runHttpsFirewall());
    },

    async httpsLatency() {
      return this.parseResult_(await this.getNetworkDiagnostics().runHttpsLatency());
    },

    async hasSecureWiFiConnection() {
      return this.parseResult_(await this.getNetworkDiagnostics().runHasSecureWiFiConnection());
    },

    async signalStrength() {
      return this.parseResult_(await this.getNetworkDiagnostics().runSignalStrength());
    },

    async gatewayCanBePinged() {
      return this.parseResult_(await this.getNetworkDiagnostics().runGatewayCanBePinged());
    },

    async captivePortal() {
      return this.parseResult_(await this.getNetworkDiagnostics().runCaptivePortal());
    },

    async videoConferencing() {
      return this.parseResult_(await this.getNetworkDiagnostics().runVideoConferencing());
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

// RoutineResult is a simplified data structure to hold the result of running a
// network diagnostic routine. It collapses the mojo problems union to a single
// Problems field for simplicity.
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
	RoutineSecureWiFiConnection = "hasSecureWiFiConnection"
	RoutineSignalStrength       = "signalStrength"
	RoutineGatewayCanBePing     = "gatewayCanBePinged"
	RoutineCaptivePortal        = "captivePortal"
	RoutineVideoConferencing    = "videoConferencing"
)
