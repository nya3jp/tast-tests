// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChromeLoginAPI,
		Desc: "Behavior of chrome.login API",
		Contacts: []string{
			"jityao@google.com", // Test author
			"chromeos-commercial-identity@google.com",
		},
		Attr:         []string{"group:enrollment"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService", "tast.cros.policy.ChromeLoginAPIService"},
		Timeout:      6 * time.Minute,
	})
}

func ChromeLoginAPI(ctx context.Context, s *testing.State) {
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

	// Enroll with no policies set.
	const enrollPolicy = `{
		"policy_user":"tast-user@managedchrome.com",
		"managed_users":["*"],
		"invalidation_source":16,
		"invalidation_name":"test_policy",
		"device_affiliation_ids":["default"],
		"user_affiliation_ids":["default"]
	}`

	if _, err := pc.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
		PolicyJson: []byte(enrollPolicy),
	}); err != nil {
		s.Fatal("Failed to enroll using chrome: ", err)
	}
	defer pc.StopChromeAndFakeDMS(ctx, &empty.Empty{})

	// Add the MGS after enrolling, since the enrollment screen is skipped when an MGS is present.
	// FakeDMS does not support public account policies which are required for MGS, so we create our own policy blob.
	const updatePolicy = `{
		"google/chromeos/device": {
			"device_login_screen_extensions.device_login_screen_extensions":["oclffehlkdgibkainkilopaalpdobkan"],
			"device_local_accounts.account":[{"account_id": "foo@bar.com", "type": 0}]
		},
		"google/chromeos/publicaccount": {},
		"policy_user":"tast-user@managedchrome.com",
		"managed_users":["*"],
		"invalidation_source":16,
		"invalidation_name":"test_policy",
		"device_affiliation_ids":["default"],
		"user_affiliation_ids":["default"]
	}`

	if _, err := pc.UpdatePolicies(ctx, &ps.UpdatePoliciesRequest{
		PolicyJson: []byte(updatePolicy),
	}); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	prc := ps.NewChromeLoginAPIServiceClient(cl.Conn)

	if _, err := prc.TestLaunchManagedGuestSession(ctx, &ps.TestLaunchManagedGuestSessionRequest{
		ExtensionID: "oclffehlkdgibkainkilopaalpdobkan",
	}); err != nil {
		s.Error("Could not launch Managed Guest Session: ", err)
	}
}
