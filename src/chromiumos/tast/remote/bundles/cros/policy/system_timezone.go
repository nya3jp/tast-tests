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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SystemTimezone,
		Desc: "Behavior of SystemTimezone policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		Attr:         []string{"group:enrollment"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService", "tast.cros.policy.SystemTimezoneService"},
		Timeout:      7 * time.Minute,
	})
}

func SystemTimezone(ctx context.Context, s *testing.State) {
	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMIsResetAndPowerwash(ctx, s.DUT()); err != nil {
			s.Error("Failed to reset TPM: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	for _, param := range []struct {
		name     string                // name is the subtest name.
		policy   policy.SystemTimezone // policy is the policy we test.
		timezone string                // timezone is a short string of the timezone set by the policy.
	}{
		{
			name:     "Berlin",
			policy:   policy.SystemTimezone{Val: "Europe/Berlin"},
			timezone: "CEST",
		},
		{
			name:     "Los Angeles",
			policy:   policy.SystemTimezone{Val: "America/Los_Angeles"},
			timezone: "PDT",
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

			pc := ps.NewPolicyServiceClient(cl.Conn)

			pb := fakedms.NewPolicyBlob()
			pb.AddPolicy(&param.policy)

			pJSON, err := json.Marshal(pb)
			if err != nil {
				s.Fatal("Failed to serialize policies: ", err)
			}

			if _, err := pc.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
				PolicyJson: pJSON,
			}); err != nil {
				s.Fatal("Failed to enroll using chrome: ", err)
			}
			pc.StopChromeAndFakeDMS(ctx, &empty.Empty{})

			psc := ps.NewSystemTimezoneServiceClient(cl.Conn)

			if _, err = psc.TestSystemTimezone(ctx, &ps.TestSystemTimezoneRequest{
				Timezone: param.timezone,
			}); err != nil {
				s.Error("Failed to set SystemTimezone policy : ", err)
			}
		})
	}
}
