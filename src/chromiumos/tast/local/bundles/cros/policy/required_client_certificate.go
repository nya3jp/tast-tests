// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RequiredClientCertificate,
		Desc: "Behavior of RequiredClientCertificateForDevice/User policies, check if a certificate is issued when the respective policy is set",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"pmarko@google.com",         // Feature owner
			"miersh@google.com",         // Feature owner
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "fakeDMSEnrolled",
		Timeout:      3 * time.Minute,
	})
}

func RequiredClientCertificate(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*fakedms.FakeDMS)

	attestationPolicy := &policy.AttestationEnabledForDevice{Val: true}

	for _, param := range []struct {
		name   string        // name is the subtest name.
		policy policy.Policy // policy is the policy we test.
		slot   int           // slot is the slot where the certificate is placed.
	}{
		{
			name: "device",
			policy: &policy.RequiredClientCertificateForDevice{
				Val: []*policy.RequiredClientCertificateForDeviceValue{
					{
						CertProfileId:        "cert_profile_device_1",
						KeyAlgorithm:         "rsa",
						Name:                 "Cert Profile Device 1",
						PolicyVersion:        "policy_version_1",
						RenewalPeriodSeconds: 60 * 60 * 24 * 365,
					},
				},
			},
			slot: 0, // Device certificates will be placed in slot 0.
		},
		{
			name: "user",
			policy: &policy.RequiredClientCertificateForUser{
				Val: []*policy.RequiredClientCertificateForUserValue{
					{
						CertProfileId:        "cert_profile_user_1",
						KeyAlgorithm:         "rsa",
						Name:                 "Cert Profile User 1",
						PolicyVersion:        "policy_version_1",
						RenewalPeriodSeconds: 60 * 60 * 24 * 365,
					},
				},
			},
			slot: 1, // User certificates will take the next free slot which is 1.
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {

			// Start Chrome.
			cr, err := chrome.New(ctx,
				chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
				chrome.DMSPolicy(fdms.URL),
				chrome.KeepEnrollment())
			if err != nil {
				s.Fatal("Chrome login failed: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy, attestationPolicy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Close the previous Chrome instance.
			if err := cr.Close(ctx); err != nil {
				s.Error("Failed to close Chrome connection: ", err)
			}

			// Reatart Chrome to trigger fetching of the certificate.
			cr, err = chrome.New(ctx,
				chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
				chrome.DMSPolicy(fdms.URL),
				chrome.KeepEnrollment())
			if err != nil {
				s.Fatal("Chrome login failed: ", err)
			}

			// Wait until the certificate is installed.
			if err := testing.Poll(ctx, func(ctx context.Context) error {

				// The argument --slot is for the system slot which will have
				// the certificate.
				out, err := testexec.CommandContext(ctx, "pkcs11-tool", "--module", "libchaps.so", "--slot", strconv.Itoa(param.slot), "--list-objects").Output()
				if err != nil {
					return errors.Wrap(err, "failed to get certificate list")
				}
				outStr := string(out)

				// Currently the policy_testserver.py serves a hardcoded certificate,
				// with TastTest as the issuer. So we look for that string to
				// identify the correct certificate.
				if !strings.Contains(outStr, "TastTest") {
					return errors.New("certificate not installed")
				}

				return nil

			}, nil); err != nil {
				s.Error("Could not verify that client certificate was installed: ", err)
			}

			if err := cr.Close(ctx); err != nil {
				s.Error("Failed to close Chrome connection: ", err)
			}
		})
	}
}
