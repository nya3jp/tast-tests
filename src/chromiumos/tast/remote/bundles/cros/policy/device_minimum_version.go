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
		Func: DeviceMinimumVersion,
		Desc: "Behavior of DeviceMinimumVersion policy",
		Contacts: []string{
			"snijhara@google.com", // Test author
			"marcgrimme@google.com",
		},
		Attr:         []string{"group:enrollment"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService", "tast.cros.policy.DeviceMinimumVersionService"},
		Timeout:      7 * time.Minute,
	})
}

func DeviceMinimumVersion(ctx context.Context, s *testing.State) {
	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMIsResetAndPowerwash(ctx, s.DUT()); err != nil {
			s.Error("Failed to reset TPM: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	if err := policyutil.EnsureTPMIsResetAndPowerwash(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	pc := ps.NewPolicyServiceClient(cl.Conn)

	// Start with empty policy and set policy value after logging into the session so that
	// the user can be force logged out as per policy behavior.
	emptyPb := fakedms.NewPolicyBlob()
	emptyJSON, err := json.Marshal(emptyPb)
	if err != nil {
		s.Fatal("Failed to serialize empty policies: ", err)
	}

	if _, err := pc.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
		PolicyJson: emptyJSON,
	}); err != nil {
		s.Fatal("Failed to enroll using chrome: ", err)
	}

	s.Log("Updating policies in session")
	minimumVersionPb := fakedms.NewPolicyBlob()
	minimumVersionPb.AddPolicy(&policy.DeviceMinimumVersion{
		Val: &policy.DeviceMinimumVersionValue{
			Requirements: []*policy.DeviceMinimumVersionValueRequirements{
				{
					AueWarningPeriod: 2,
					ChromeosVersion:  "99999999",
					WarningPeriod:    0,
				},
			},
		},
	})

	minimumVersionJSON, err := json.Marshal(minimumVersionPb)
	if err != nil {
		s.Fatal("Failed to serialize policies: ", err)
	}
	pc.UpdatePolicies(ctx, &ps.UpdatePoliciesRequest{
		PolicyJson: minimumVersionJSON,
	})

	if _, err = pc.StopChromeAndFakeDMS(ctx, &empty.Empty{}); err != nil {
		s.Fatal(err, "failed to close policy service chrome instance")
	}

	dmvc := ps.NewDeviceMinimumVersionServiceClient(cl.Conn)

	if _, err = dmvc.TestUpdateRequiredScreenIsVisible(ctx, &empty.Empty{}); err != nil {
		s.Error("Failed to set DeviceMinimumVersion policy: ", err)
	}
}
