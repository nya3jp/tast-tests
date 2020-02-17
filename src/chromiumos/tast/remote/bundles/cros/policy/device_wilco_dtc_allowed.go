// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/services/cros/wilco"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DeviceWilcoDtcAllowed,
		Desc: "Test sending GetConfigurationData gRPC request from Wilco DTC VM to the Wilco DTC Support Daemon daemon",
		Contacts: []string{
			"vsavu@chromium.org",  // Test author
			"pmoy@chromium.org",   // wilco_dtc_supportd author
			"lamzin@chromium.org", // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"group:enrollment"},
		SoftwareDeps: []string{"reboot", "vm_host", "wilco", "chrome"},
		ServiceDeps:  []string{"tast.cros.wilco.WilcoService", "tast.cros.policy.PolicyService"},
		Timeout:      12 * time.Minute,
	})
}

func DeviceWilcoDtcAllowed(ctx context.Context, s *testing.State) {
	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMIsResetAndPowerwash(ctx, s.DUT()); err != nil {
			s.Error("Failed to reset TPM: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	for _, param := range []struct {
		name string                       // name is the subtest name.
		p    policy.DeviceWilcoDtcAllowed // value is the policy value.
	}{
		{
			name: "unset",
			p:    policy.DeviceWilcoDtcAllowed{Stat: policy.StatusUnset},
		},
		{
			name: "false",
			p:    policy.DeviceWilcoDtcAllowed{Val: false},
		},
		{
			name: "true",
			p:    policy.DeviceWilcoDtcAllowed{Val: true},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			if err := policyutil.EnsureTPMIsResetAndPowerwash(ctx, s.DUT()); err != nil {
				s.Fatal("Failed to reset TPM: ", err)
			}

			cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
			if err != nil {
				s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
			}
			defer cl.Close(ctx)

			wc := wilco.NewWilcoServiceClient(cl.Conn)
			pc := ps.NewPolicyServiceClient(cl.Conn)

			pb := fakedms.NewPolicyBlob()
			pb.AddPolicy(&param.p)
			// wilco_dtc and wilco_dtc_supportd only run for affiliated users
			pb.DeviceAffiliationIds = []string{"default"}
			pb.UserAffiliationIds = []string{"default"}

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

			if status, err := wc.GetStatus(ctx, &empty.Empty{}); err != nil {
				s.Fatal("Could not get running status: ", err)
			} else if running := status.WilcoDtcSupportdPid != 0; running != param.p.Val {
				s.Errorf("Unexpected Wilco DTC Supportd running state: got %t; want %t", running, param.p.Val)
			}
		})
	}
}
