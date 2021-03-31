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
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RequiredClientCertificateFor,
		Desc: "Behavior of RequiredClientCertificateFor* policies, check if a certificate is issued when the respective policy is set",
		Contacts: []string{
			"pmarko@google.com",         // Feature owner
			"miersh@google.com",         // Feature owner
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		Attr:         []string{"group:enrollment"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "fakeDMSEnrolled",
	})
}

func RequiredClientCertificateFor(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*fakedms.FakeDMS)

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx,
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment())
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	defer func(ctx context.Context) {
		// Use cr as a reference to close the last started Chrome instance.
		if err := cr.Close(ctx); err != nil {
			s.Error("Failed to close Chrome connection: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	attestationPolicy := &policy.AttestationEnabledForDevice{Val: true}

	for _, param := range []struct {
		name   string        // name is the subtest name.
		policy policy.Policy // policy is the policy we test.
		slot   int           // slot is the slot where the certificate is placed.
	}{
		{
			name: "Device",
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
			name: "User",
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
			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy, attestationPolicy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Close the previous Chrome instance.
			if err := cr.Close(ctx); err != nil {
				s.Error("Failed to close Chrome connection: ", err)
			}

			// Restart Chrome.
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
				out, err := testexec.CommandContext(ctx, "pkcs11-tool", "--module", "libchaps.so", "--slot", strconv.Itoa(int(param.slot)), "--list-objects").Output()
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

		})
	}
}
