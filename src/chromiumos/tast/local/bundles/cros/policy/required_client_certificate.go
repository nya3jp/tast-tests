// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RequiredClientCertificate,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of RequiredClientCertificateForDevice/User policies, check if a certificate is issued when the respective policy is set",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"pmarko@google.com",         // Feature owner
			"miersh@google.com",         // Feature owner
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      fixture.FakeDMSEnrolled,
		Timeout:      6 * time.Minute,
	})
}

func RequiredClientCertificate(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*fakedms.FakeDMS)

	attestationPolicy := &policy.AttestationEnabledForDevice{Val: true}
	lacrosPolicy := &policy.LacrosAvailability{Val: "lacros_primary"}

	chromeOpts := lacrosOpts
	chromeOpts = append(chromeOpts, chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}))
	chromeOpts = append(chromeOpts, chrome.DMSPolicy(fdms.URL))
	chromeOpts = append(chromeOpts, chrome.KeepEnrollment())

	for _, param := range []struct {
		name   string        // name is the subtest name.
		policy policy.Policy // policy is the policy we test.
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
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			cr, err := chrome.New(ctx, chromeOpts...)
			if err != nil {
				s.Fatal("Chrome login failed: ", err)
			}

			pb := newPolicyBlobWithAffiliation()

			// After this point, IsUserAffiliated flag should be updated.
			if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
				s.Fatal("Failed to serve and refresh: ", err)
			}

			// We should add policy value in the middle of 2 ServeBlobAndRefresh calls to be sure
			// that IsUserAffiliated flag is updated and policy handler is triggered.
			pb.AddPolicies([]policy.Policy{param.policy, attestationPolicy, lacrosPolicy})

			// After this point, the policy handler should be triggered.
			if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
				s.Fatal("Failed to serve and refresh: ", err)
			}

			// Restart Chrome to trigger fetching of the certificate.
			cr, err = chrome.New(ctx, chromeOpts...)
			if err != nil {
				s.Fatal("Chrome login failed: ", err)
			}

			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create Test API connection: ", err)
			}

			s.Log("Checking that certificate is visible in Lacros")
			func() {
				// TODO(neis): Provide LaunchPrimaryBrowser or similar.
				l, err := lacros.LaunchFromShelf(ctx, tconn, "/usr/local/lacros-chrome")
				if err != nil {
					s.Fatal("Failed to launch lacros: ", err)
				}
				defer l.Close(ctx)
				if err = checkCertificateVisibleInBrowserSettings(ctx, tconn, l.Browser()); err != nil {
					s.Fatal("Failed to find certificate: ", err)
				}
			}()

			s.Log("Checking that certificate is visible in Ash")
			if err = checkCertificateVisibleInBrowserSettings(ctx, tconn, cr.Browser()); err != nil {
				s.Fatal("Failed to find certificate: ", err)
			}
		})
	}
}

func newPolicyBlobWithAffiliation() *fakedms.PolicyBlob {
	affiliationIds := []string{"default_affiliation_id"}
	pb := fakedms.NewPolicyBlob()
	pb.DeviceAffiliationIds = affiliationIds
	pb.UserAffiliationIds = affiliationIds
	return pb
}

// TODO(neis): Provide GetLacrosInfo(lacrosmode) or similar, which includes these standard flags.
var lacrosOpts = []chrome.Option{
	chrome.EnableFeatures("LacrosSupport", "LacrosPrimary", "ForceProfileMigrationCompletion"),
	chrome.ExtraArgs("--lacros-selection=rootfs", "--disable-lacros-keep-alive", "--disable-login-lacros-opening"),
	chrome.LacrosExtraArgs("--remote-debugging-port=0"),
}

func checkCertificateVisibleInBrowserSettings(ctx context.Context, tconn *chrome.TestConn, br *browser.Browser) error {
	conn, err := br.NewConn(ctx, "chrome://settings/certificates")
	if err != nil {
		return err
	}
	defer conn.Close()

	// We may have to reload the page for the certificate to show up.
	ui := uiauto.New(tconn)
	return testing.Poll(ctx, func(ctx context.Context) error {
		node := nodewith.Role(role.StaticText).Name("org-TastTest")
		if err := ui.WithTimeout(3 * time.Second).WaitUntilExists(node)(ctx); err != nil {

			//if err := reloadActiveTab(ctx, br); err != nil {
			if err := ui.LeftClick(nodewith.Name("Reload").Role(role.Button).Focusable().First())(ctx); err != nil {
				return testing.PollBreak(err)
			}
			return err // Try again after reloading.
		}
		return nil
	}, nil)
}

//// XXX Why does this not work for Lacros? Test API connection creation times out.
//func reloadActiveTab(ctx context.Context, br *browser.Browser) error {
//	tconn, err := br.TestAPIConn(ctx)
//	if err != nil {
//		return errors.Wrap(err, "failed to create Test API connection")
//	}
//	return tconn.Eval(ctx, "chrome.tabs.reload()", nil)
//}
