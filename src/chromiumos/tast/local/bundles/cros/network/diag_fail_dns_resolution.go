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

func init() {
	testing.AddTest(&testing.Test{
		Func:         DiagFailDNSResolution,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Tests that the DNS resolution network diagnostic test fails when the DNS cannot resolve requests",
		Contacts: []string{
			"khegde@chromium.org",            // test maintainer
			"stevenjb@chromium.org",          // network-health tech lead
			"cros-network-health@google.com", // network-health team
		},
		SoftwareDeps: []string{"chrome", "no_qemu"},
		Attr:         []string{"group:mainline"},
		Fixture:      "networkDiagnosticsShillReset",
	})
}

// DiagFailDNSResolution tests that when the domain name server (DNS) cannot
// resolve requests the network diagnostic routine can detect this condition.
func DiagFailDNSResolution(ctx context.Context, s *testing.State) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	serviceProps := map[string]interface{}{
		shillconst.ServicePropertyType: "ethernet",
		shillconst.ServicePropertyStaticIPConfig: map[string]interface{}{
			shillconst.IPConfigPropertyNameServers: []string{"0.0.0.0"},
		},
	}
	if _, err := manager.ConfigureServiceForProfile(ctx, shillconst.DefaultProfileObjectPath, serviceProps); err != nil {
		s.Fatal("Unable to configure shill service: ", err)
	}

	if _, err := manager.WaitForServiceProperties(ctx, serviceProps, 5*time.Second); err != nil {
		s.Fatal("Service not found: ", err)
	}

	mojo := s.FixtValue().(*diag.MojoAPI)
	// After the property change is emitted, Chrome still needs to process it.
	// Since Chrome does not emit a change, poll to test whether the expected
	// problem occurs.
	const problemFailedToResolveHost uint32 = 0
	expectedResult := &diagcommon.RoutineResult{
		Verdict:  diagcommon.VerdictProblem,
		Problems: []uint32{problemFailedToResolveHost},
	}
	if err := mojo.PollRoutine(ctx, diagcommon.RoutineDNSResolution, expectedResult); err != nil {
		s.Fatal("Failed to poll routine: ", err)
	}
}
