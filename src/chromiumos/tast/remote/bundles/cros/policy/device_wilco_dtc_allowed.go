// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/services/cros/wilco"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DeviceWilcoDtcAllowed,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test setting the DeviceWilcoDtcAllowed policy by checking if the Wilco DTC Support Daemon is running",
		Contacts: []string{
			"chromeos-oem-services@google.com", // Use team email for tickets.
			"bkersting@google.com",
			"lamzin@google.com",
		},
		Attr:         []string{"group:enrollment"},
		SoftwareDeps: []string{"reboot", "vm_host", "wilco", "chrome"},
		ServiceDeps: []string{
			"tast.cros.hwsec.OwnershipService",
			"tast.cros.policy.PolicyService",
			"tast.cros.wilco.WilcoService",
		},
		Timeout: 12 * time.Minute,
	})
}

// DeviceWilcoDtcAllowed tests DeviceWilcoDtcAllowed policy.
// TODO(b/189457904): remove once policy.DeviceWilcoDtcAllowedEnrolled will be stable enough.
func DeviceWilcoDtcAllowed(ctx context.Context, s *testing.State) {
	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT(), s.RPCHint()); err != nil {
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
			if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT(), s.RPCHint()); err != nil {
				s.Fatal("Failed to reset TPM: ", err)
			}

			cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
			if err != nil {
				s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
			}
			defer cl.Close(ctx)

			wilcoClient := wilco.NewWilcoServiceClient(cl.Conn)
			policyClient := ps.NewPolicyServiceClient(cl.Conn)

			pb := policy.NewBlob()
			pb.AddPolicy(&param.p)
			// wilco_dtc and wilco_dtc_supportd only run for affiliated users
			pb.DeviceAffiliationIds = []string{"default"}
			pb.UserAffiliationIds = []string{"default"}

			pJSON, err := json.Marshal(pb)
			if err != nil {
				s.Fatal("Failed to serialize policies: ", err)
			}

			if _, err := policyClient.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
				PolicyJson: pJSON,
			}); err != nil {
				s.Fatal("Failed to enroll using chrome: ", err)
			}
			defer policyClient.StopChromeAndFakeDMS(ctx, &empty.Empty{})

			if status, err := wilcoClient.GetStatus(ctx, &empty.Empty{}); err != nil {
				s.Fatal("Could not get running status: ", err)
			} else if running := status.WilcoDtcSupportdPid != 0; running != param.p.Val {
				s.Errorf("Unexpected Wilco DTC Supportd running state: got %t; want %t", running, param.p.Val)
			}
		})
	}
}
