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
		Func: RequiredClientCertificateForDevice,
		Desc: "Behavior of RequiredClientCertificateForDevice policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		Attr:         []string{"group:enrollment"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService", "tast.cros.policy.RequiredClientCertificateForDeviceService"},
		Timeout:      5 * time.Minute,
	})
}

func RequiredClientCertificateForDevice(ctx context.Context, s *testing.State) {
	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMIsResetAndPowerwash(ctx, s.DUT()); err != nil {
			s.Error("Failed to reset TPM: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()

	certProvDevicePolicy := policy.RequiredClientCertificateForDevice{}
	certProvDevicePolicy.Val = append(certProvDevicePolicy.Val,
		&policy.RequiredClientCertificateForDeviceValue{
			CertProfileId:        "cert_profile_device_1",
			KeyAlgorithm:         "rsa",
			Name:                 "Cert Profile Device 1",
			PolicyVersion:        "policy_version_1",
			RenewalPeriodSeconds: 60 * 60 * 24 * 365,
		})

	attestationPolicy := policy.AttestationEnabledForDevice{Val: true}

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
	pb.AddPolicy(&attestationPolicy)
	pb.AddPolicy(&certProvDevicePolicy)
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

	prc := ps.NewRequiredClientCertificateForDeviceServiceClient(cl.Conn)

	if _, err := prc.TestClientCertificateIsInstalled(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to set RequiredClientCertificateForDevice policy: ", err)
	}
}
