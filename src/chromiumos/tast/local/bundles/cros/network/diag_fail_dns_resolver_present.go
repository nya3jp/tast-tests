// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	diagcommon "chromiumos/tast/common/network/diag"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/local/bundles/cros/network/diag"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

type dnsResolverPresentProblem uint32

const (
	// problemNoNameServersFound - IP config has no name servers available
	problemNoNameServersFound dnsResolverPresentProblem = 0
	// problemMalformedNameServers - IP config has at least one malformed name server
	problemMalformedNameServers = 1
	// problemEmptyNameServers - IP config has an empty list of name servers
	problemEmptyNameServers = 2
)

type dnsResolverPresentParams struct {
	NameServers     []string
	ExpectedProblem dnsResolverPresentProblem
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DiagFailDNSResolverPresent,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Tests that the DNS resolver present network diagnostic test fails as expected with malformed DNS names",
		Contacts: []string{
			"khegde@chromium.org",            // test maintainer
			"khegde@chromium.org",            // network diagnostics author
			"stevenjb@chromium.org",          // network-health tech lead
			"cros-network-health@google.com", // network-health team
		},
		SoftwareDeps: []string{"chrome", "no_qemu"},
		Attr:         []string{"group:mainline"},
		Fixture:      "networkDiagnosticsShillReset",
		Params: []testing.Param{{
			Name: "no_name_servers",
			Val: &dnsResolverPresentParams{
				NameServers:     []string{},
				ExpectedProblem: problemNoNameServersFound,
			},
			ExtraAttr: []string{"informational"},
		}, {
			Name: "malformed_name_servers",
			Val: &dnsResolverPresentParams{
				NameServers:     []string{"bad.ip.address"},
				ExpectedProblem: problemMalformedNameServers,
			},
			ExtraAttr: []string{"informational"},
		}, {
			Name: "empty_name_servers",
			Val: &dnsResolverPresentParams{
				NameServers:     []string{""},
				ExpectedProblem: problemNoNameServersFound,
			},
			ExtraAttr: []string{"informational"},
		}, {

			Name: "default_name_servers",
			Val: &dnsResolverPresentParams{
				NameServers:     []string{"0.0.0.0"},
				ExpectedProblem: problemNoNameServersFound,
			},
			ExtraAttr: []string{"informational"},
		}},
	})
}

// DiagFailDNSResolverPresent tests that when the domain name server (DNS) are
// misconfigured that the network routine reports the correct errors.
func DiagFailDNSResolverPresent(ctx context.Context, s *testing.State) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	params := s.Param().(*dnsResolverPresentParams)
	serviceProps := map[string]interface{}{
		shillconst.ServicePropertyType: "ethernet",
		shillconst.ServicePropertyStaticIPConfig: map[string]interface{}{
			shillconst.IPConfigPropertyNameServers: params.NameServers,
		},
	}

	if _, err := manager.ConfigureServiceForProfile(ctx, shillconst.DefaultProfileObjectPath, serviceProps); err != nil {
		s.Fatal("Failed to configure shill service: ", err)
	}

	if _, err := manager.WaitForServiceProperties(ctx, serviceProps, 5*time.Second); err != nil {
		s.Fatal("Failed to find shill service: ", err)
	}

	mojo := s.FixtValue().(*diag.MojoAPI)
	// After the property change is emitted, Chrome still needs to process it.
	// Since Chrome does not emit a change, poll to test whether the expected
	// problem occurs.
	expectedResult := &diagcommon.RoutineResult{
		Verdict:  diagcommon.VerdictProblem,
		Problems: []uint32{uint32(params.ExpectedProblem)},
	}
	if err := mojo.PollRoutine(ctx, diagcommon.RoutineDNSResolverPresent, expectedResult); err != nil {
		s.Fatal("Failed to poll routine: ", err)
	}
}
