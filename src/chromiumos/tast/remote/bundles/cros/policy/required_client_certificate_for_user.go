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
		Func: RequiredClientCertificateForUser,
		Desc: "Behavior of RequiredClientCertificateForUser policy, check if a certificate is issued when the policy is set",
		Contacts: []string{
			"pmarko@google.com",         // Feature owner
			"miersh@google.com",         // Feature owner
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		Attr:         []string{"group:enrollment"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService", "tast.cros.policy.ClientCertificateService"},
		Timeout:      6 * time.Minute,
	})
}

func RequiredClientCertificateForUser(ctx context.Context, s *testing.State) {
	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
			s.Error("Failed to reset TPM: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

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
	pb.AddPolicy(&policy.AttestationEnabledForDevice{Val: true})
	pb.AddPolicy(&policy.AttestationEnabledForUser{Val: true})
	pb.AddPolicy(&policy.RequiredClientCertificateForUser{
		Val: []*policy.RequiredClientCertificateForUserValue{
			{
				CertProfileId:        "cert_profile_user_1",
				KeyAlgorithm:         "rsa",
				Name:                 "Cert Profile User 1",
				PolicyVersion:        "policy_version_1",
				RenewalPeriodSeconds: 60 * 60 * 24 * 365,
			},
		},
	})
	pb.DeviceAffiliationIds = []string{"default"} // Required for issuing the certificate.
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

	prc := ps.NewClientCertificateServiceClient(cl.Conn)

	// Device certificates will be placed in slot 0, user certificates will take the next free slot which is 1.
	if _, err := prc.TestClientCertificateIsInstalled(ctx, &ps.TestClientCertificateIsInstalledRequest{
		Slot: 1,
	}); err != nil {
		s.Error("Could not verify that client certificate was installed: ", err)
	}
}
