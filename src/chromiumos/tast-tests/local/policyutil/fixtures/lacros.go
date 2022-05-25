// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixtures

import (
	"context"
	"time"

	"chromiumos/tast-tests/common/fixture"
	"chromiumos/tast-tests/common/policy"
	"chromiumos/tast-tests/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast-tests/local/chrome"
	"chromiumos/tast-tests/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     fixture.LacrosPolicyLoggedIn,
		Desc:     "Fixture for a running FakeDMS with lacros",
		Contacts: []string{"mohamedaomar@google.com", "wtlee@chromium.org", "chromeos-commercial-remote-management@google.com"},
		Impl: &policyChromeFixture{
			extraOptsFunc: func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
				return lacrosfixt.NewConfig().Opts()
			},
		},
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PostTestTimeout: 15 * time.Second,
		Parent:          fixture.PersistentLacros,
	})

	// TODO(b/218907052): Remove fixture after Journeys flag  is enabled by default.
	testing.AddFixture(&testing.Fixture{
		Name:     fixture.LacrosPolicyLoggedInFeatureJourneys,
		Desc:     "Fixture for a running FakeDMS with lacros and enabling the feature flag Journeys",
		Contacts: []string{"rodmartin@google.com", "chromeos-commercial-remote-management@google.com"},
		Impl: &policyChromeFixture{
			extraOptsFunc: func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
				return lacrosfixt.NewConfig(lacrosfixt.ChromeOptions(chrome.LacrosEnableFeatures("Journeys"))).Opts()
			},
		},
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PostTestTimeout: 15 * time.Second,
		Parent:          fixture.PersistentLacros,
	})

	testing.AddFixture(&testing.Fixture{
		Name: fixture.LacrosPolicyLoggedInFeatureChromeLabs,
		Desc: "Fixture for a running FakeDMS with lacros and enabling the feature flag Chrome Labs",
		Impl: &policyChromeFixture{
			extraOptsFunc: func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
				return lacrosfixt.NewConfig(lacrosfixt.ChromeOptions(chrome.LacrosEnableFeatures("ChromeLabs"))).Opts()
			},
		},
		Contacts:        []string{"samicolon@google.com", "chromeos-commercial-remote-management@google.com"},
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PostTestTimeout: 15 * time.Second,
		Parent:          fixture.PersistentLacros,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            fixture.LacrosPolicyLoggedInRealUser,
		Desc:            "Fixture for a running FakeDMS with lacros with a real managed user logged on",
		Contacts:        []string{"anastasiian@chromium.org", "chromeos-commercial-remote-management@google.com"},
		Impl:            &policyRealUserFixture{},
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PostTestTimeout: 15 * time.Second,
		Parent:          fixture.PersistentLacros,
		Vars:            []string{"policy.ManagedUser.accountPool"},
	})
}

type policyRealUserFixture struct {
	// fdms is the already running DMS server from the parent fixture.
	fdms *fakedms.FakeDMS
}

// PolicyRealUserFixtData is returned by the fixtures and used in tests
// by using interface HashFakeDMS to get fakeDMS.
type PolicyRealUserFixtData struct {
	// fakeDMS is an already running DMS server.
	fakeDMS *fakedms.FakeDMS
	// Chrome options to be used for starting Chrome and are set in SetUp().
	opts []chrome.Option
}

// FakeDMS implements the HasFakeDMS interface.
func (f PolicyRealUserFixtData) FakeDMS() *fakedms.FakeDMS {
	if f.fakeDMS == nil {
		panic("FakeDMS is called with nil fakeDMS instance")
	}
	return f.fakeDMS
}

// Opts returns chrome options that were created in SetUp().
func (f PolicyRealUserFixtData) Opts() []chrome.Option {
	return f.opts
}

func (p *policyRealUserFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	fdms, ok := s.ParentValue().(*fakedms.FakeDMS)
	if !ok {
		s.Fatal("Parent is not a FakeDMS fixture")
	}
	p.fdms = fdms

	gaiaCreds, err := chrome.PickRandomCreds(s.RequiredVar("policy.ManagedUser.accountPool"))
	if err != nil {
		s.Fatal("Failed to parse managed user creds: ", err)
	}
	fdms.SetPersistentPolicyUser(&gaiaCreds.User)
	if err := fdms.WritePolicyBlob(policy.NewBlob()); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	opts := []chrome.Option{
		chrome.DMSPolicy(fdms.URL),
		chrome.CustomLoginTimeout(chrome.ManagedUserLoginTimeout),
	}
	extraOpts, err := lacrosfixt.NewConfig(
		lacrosfixt.ChromeOptions(chrome.GAIALogin(gaiaCreds)),
		lacrosfixt.EnableChromeFRE()).Opts()
	if err != nil {
		return errors.Wrap(err, "failed to get extra options")
	}
	opts = append(opts, extraOpts...)

	return &PolicyRealUserFixtData{p.fdms, opts}
}

func (p *policyRealUserFixture) TearDown(ctx context.Context, s *testing.FixtState) {
}

func (p *policyRealUserFixture) Reset(ctx context.Context) error {
	return nil
}

func (p *policyRealUserFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (p *policyRealUserFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
}
