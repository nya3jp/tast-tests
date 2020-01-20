// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/remote/hwsec"
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
			"vsavu@chromium.org", // Test author
			"kathrelkeld@chromium.org",
			"dbeckett@chromium.org",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"reboot", "vm_host", "wilco", "chrome"},
		ServiceDeps:  []string{"tast.cros.wilco.WilcoService", "tast.cros.policy.PolicyService"},
	})
}

func DeviceWilcoDtcAllowed(ctx context.Context, s *testing.State) {
	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.DeviceWilcoDtcAllowed
	}{
		{
			name:  "unset",
			value: &policy.DeviceWilcoDtcAllowed{Stat: policy.StatusUnset},
		},
		{
			name:  "false",
			value: &policy.DeviceWilcoDtcAllowed{Val: false},
		},
		{
			name:  "true",
			value: &policy.DeviceWilcoDtcAllowed{Val: true},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			if err := hwsec.EnsureOwnerIsReset(ctx, s.DUT()); err != nil {
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
			pb.AddPolicy(param.value)

			pJSON, err := pb.ToRawJSON()
			if err != nil {
				s.Fatal("Failed to serialize policies: ", err)
			}

			if _, err := pc.EnrollUsingChrome(ctx, &ps.PolicyBlob{
				PolicyBlob: pJSON,
			}); err != nil {
				s.Fatal("Failed to enroll using chrome: ", err)
			}
			defer pc.StopChrome(ctx, &empty.Empty{})

			if status, err := wc.GetStatus(ctx, &empty.Empty{}); err != nil {
				s.Fatal("Could not get running status: ", err)
			} else if status.WilcoDtcSupportdRunning != param.value.Val {
				s.Errorf("Unexpected Wilco DTC Supportd running state: got %t; want %t", status.WilcoDtcSupportdRunning, param.value.Val)
			}
		})
	}

	if err := hwsec.EnsureOwnerIsReset(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}
}
