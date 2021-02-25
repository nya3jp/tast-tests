// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SystemProxySettings,
		Desc: "Test setting the SystemProxySettings policy by checking if the System-proxy daemon and worker processes are running",
		Contacts: []string{
			"acostinas@google.com",
			"hugobenichi@chromium.org",
			"omorsi@chromium.org",
			"pmarko@chromium.org",
		},
		Attr:         []string{"group:enrollment"},
		SoftwareDeps: []string{"reboot", "vm_host", "chrome"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService"},
		Timeout:      12 * time.Minute,
	})
}

func SystemProxySettings(ctx context.Context, s *testing.State) {
	// Gets the number of running process instances of path by counting the pids.
	activeProcessCount := func(path string) int {
		out, err := s.DUT().Command("pidof", path).Output(ctx)
		if err != nil {
			// pidof() returns status 1 when no pid is found for path.
			return 0
		}
		outMain := string(out)
		if len(out) == 0 {
			return 0
		}
		return len(strings.Fields(outMain))
	}

	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
			s.Fatal("Failed to reset TPM: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	for _, param := range []struct {
		name string                     // name is the subtest name
		p    policy.SystemProxySettings // value is the policy value
	}{
		{
			name: "enabled",
			p: policy.SystemProxySettings{
				Val: &policy.SystemProxySettingsValue{
					SystemProxyEnabled:           true,
					SystemServicesPassword:       "",
					SystemServicesUsername:       "",
					PolicyCredentialsAuthSchemes: []string{},
				},
			},
		},
		{
			name: "disabled",
			p: policy.SystemProxySettings{
				Val: &policy.SystemProxySettingsValue{
					SystemProxyEnabled:           false,
					SystemServicesPassword:       "",
					SystemServicesUsername:       "",
					PolicyCredentialsAuthSchemes: []string{},
				},
			},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
				s.Fatal("Failed to reset TPM: ", err)
			}

			cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
			if err != nil {
				s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
			}
			defer cl.Close(ctx)

			pc := ps.NewPolicyServiceClient(cl.Conn)

			pb := fakedms.NewPolicyBlob()
			pb.AddPolicy(&param.p)
			pb.AddPolicy(&policy.ArcEnabled{Val: true})
			pJSON, err := json.Marshal(pb)
			if err != nil {
				s.Fatal("Failed to serialize policies: ", err)
			}

			if _, err := pc.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
				PolicyJson: pJSON,
			}); err != nil {
				s.Fatal("Failed to enroll using chrome: ", err)
			}
			defer pc.StopChromeAndFakeDMS(ctx, &empty.Empty{})

			const (
				mainBinPath   = "/usr/sbin/system_proxy"
				workerBinPath = "/usr/sbin/system_proxy_worker"
			)

			mainProcessCount := activeProcessCount(mainBinPath)
			workerProcessCount := activeProcessCount(workerBinPath)

			if param.p.Val.SystemProxyEnabled {
				// Expect two worker processes: one which tunnels system traffic and one for ARC++ traffic.
				if workerProcessCount != 2 {
					s.Errorf("Unexpected number of worker processes running: %d", workerProcessCount)
				}
			} else {
				if mainProcessCount != 0 {
					s.Error("System-proxy running although disabled by policy")
				}
			}
		})
	}
}
