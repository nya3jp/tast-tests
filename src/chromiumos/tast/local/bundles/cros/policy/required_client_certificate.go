// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
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
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.FakeDMSEnrolled,
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			Val: browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.RequiredClientCertificateForUser{}, pci.VerifiedFunctionalityUI),
			pci.SearchFlag(&policy.RequiredClientCertificateForDevice{}, pci.VerifiedFunctionalityUI),
			pci.SearchFlag(&policy.LacrosAvailability{}, pci.VerifiedValue),
		},
	})
}

const certificateName = "TastTest"

func RequiredClientCertificate(ctx context.Context, s *testing.State) {
	browserType := s.Param().(browser.Type)
	fdms := s.FixtValue().(*fakedms.FakeDMS)

	extraPolicies := []policy.Policy{&policy.AttestationEnabledForDevice{Val: true}}
	if browserType == browser.TypeLacros {
		extraPolicies = append(extraPolicies, &policy.LacrosAvailability{Val: "lacros_only"})
	}

	chromeOpts := []chrome.Option{
		chrome.DMSPolicy(fdms.URL), chrome.KeepEnrollment(),
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
	}
	if browserType == browser.TypeLacros {
		var err error
		chromeOpts, err = lacrosfixt.NewConfig(lacrosfixt.Mode(lacros.LacrosOnly), lacrosfixt.ChromeOptions(chromeOpts...)).Opts()
		if err != nil {
			s.Fatal("Failed to compute Chrome options: ", err)
		}
	}

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
			pb.AddPolicies(append([]policy.Policy{param.policy}, extraPolicies...))

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

			if browserType == browser.TypeLacros {
				s.Log("Starting to check that certificate is visible in Lacros")
				func() {
					l, err := lacros.Launch(ctx, tconn)
					if err != nil {
						s.Fatal("Failed to launch Lacros: ", err)
					}
					defer l.Close(ctx)
					if err := checkCertificateVisibleInBrowserSettings(ctx, tconn, l.Browser()); err != nil {
						s.Fatal("Failed to find certificate: ", err)
					}
				}()
			} else {
				s.Log("Starting to check that certificate is visible in Ash")
				// TODO(neis): Remove this once Lacros is the only browser.
				if err := checkCertificateVisibleInBrowserSettings(ctx, tconn, cr.Browser()); err != nil {
					s.Fatal("Failed to find certificate: ", err)
				}
			}

			s.Log("Starting to check that certificate is visible in system settings")
			if err := policyutil.CheckCertificateVisibleInSystemSettings(ctx, tconn, cr, certificateName); err != nil {
				s.Fatal("Failed to find certificate: ", err)
			}
		})
	}
}

func newPolicyBlobWithAffiliation() *policy.Blob {
	affiliationIds := []string{"default_affiliation_id"}
	pb := policy.NewBlob()
	pb.DeviceAffiliationIds = affiliationIds
	pb.UserAffiliationIds = affiliationIds
	return pb
}

// checkCertificateVisibleInBrowserSettings does what its name suggests.
// NOTE: tconn must be a TestConn for Ash.
func checkCertificateVisibleInBrowserSettings(ctx context.Context, tconn *chrome.TestConn, br *browser.Browser) error {
	conn, err := br.NewConn(ctx, "chrome://settings/certificates")
	if err != nil {
		return err
	}
	defer conn.Close()

	// We may have to reload the page for the certificate to show up.
	ui := uiauto.New(tconn)
	return testing.Poll(ctx, func(ctx context.Context) error {
		node := nodewith.Role(role.StaticText).Name("org-" + certificateName)
		if err := ui.WithTimeout(3 * time.Second).WaitUntilExists(node)(ctx); err != nil {
			if err := br.ReloadActiveTab(ctx); err != nil {
				return testing.PollBreak(err)
			}
			return err // Try again after reloading.
		}
		return nil
	}, nil)
}
