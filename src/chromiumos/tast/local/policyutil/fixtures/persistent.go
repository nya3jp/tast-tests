// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixtures

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     fixture.PersistentLacros,
		Desc:     "Fixture setting persistent policies needed for Lacros",
		Contacts: []string{"vsavu@google.com", "chromeos-commercial-remote-management@google.com"},
		Impl: &persistentFixture{
			policies: []policy.Policy{&policy.LacrosAvailability{Val: "lacros_only"}},
		},
		SetUpTimeout:    5 * time.Second,
		ResetTimeout:    5 * time.Second,
		TearDownTimeout: 5 * time.Second,
		PostTestTimeout: 5 * time.Second,
		Parent:          fixture.FakeDMS,
	})
	// TODO(crbug.com/1360034): Remove this fixture.
	testing.AddFixture(&testing.Fixture{
		Name:     fixture.PersistentLacrosPrimary,
		Desc:     "Fixture setting persistent policies needed for LacrosPrimary",
		Contacts: []string{"vsavu@google.com", "chromeos-commercial-remote-management@google.com"},
		Impl: &persistentFixture{
			policies: []policy.Policy{&policy.LacrosAvailability{Val: "lacros_primary"}},
		},
		SetUpTimeout:    5 * time.Second,
		ResetTimeout:    5 * time.Second,
		TearDownTimeout: 5 * time.Second,
		PostTestTimeout: 5 * time.Second,
		Parent:          fixture.FakeDMS,
	})
	testing.AddFixture(&testing.Fixture{
		Name:     fixture.PersistentLacrosEnrolled,
		Desc:     "Fixture setting persistent policies needed for Lacros on enrolled device",
		Contacts: []string{"vsavu@google.com", "chromeos-commercial-remote-management@google.com"},
		Impl: &persistentFixture{
			policies: []policy.Policy{&policy.LacrosAvailability{Val: "lacros_only"}},
		},
		SetUpTimeout:    5 * time.Second,
		ResetTimeout:    5 * time.Second,
		TearDownTimeout: 5 * time.Second,
		PostTestTimeout: 5 * time.Second,
		Parent:          fixture.FakeDMSEnrolled,
	})
	testing.AddFixture(&testing.Fixture{
		Name:     fixture.PersistentFamilyLink,
		Desc:     "Fixture setting persistent policy user for a Family Link account",
		Contacts: []string{"xiqiruan@chromium.org", "vsavu@google.com", "chromeos-commercial-remote-management@google.com"},
		Vars: []string{
			"family.unicornEmail",
		},
		Impl: &persistentFixture{
			policyUserVar: "family.unicornEmail",
		},
		SetUpTimeout:    5 * time.Second,
		ResetTimeout:    5 * time.Second,
		TearDownTimeout: 5 * time.Second,
		PostTestTimeout: 5 * time.Second,
		Parent:          fixture.FakeDMS,
	})
	testing.AddFixture(&testing.Fixture{
		Name:     fixture.PersistentFamilyLinkARC,
		Desc:     "Fixture setting persistent policy user for a Family Link account",
		Contacts: []string{"xiqiruan@chromium.org", "vsavu@google.com", "chromeos-commercial-remote-management@google.com"},
		Vars: []string{
			"arc.childUser",
		},
		Impl: &persistentFixture{
			policyUserVar: "arc.childUser",
		},
		SetUpTimeout:    5 * time.Second,
		ResetTimeout:    5 * time.Second,
		TearDownTimeout: 5 * time.Second,
		PostTestTimeout: 5 * time.Second,
		Parent:          fixture.FakeDMS,
	})
	testing.AddFixture(&testing.Fixture{
		Name:     fixture.PersistentProjectorEDU,
		Desc:     "Fixture setting persistent policy user for a managed EDU account",
		Contacts: []string{"tobyhuang@chromium.org", "vsavu@google.com", "chromeos-commercial-remote-management@google.com"},
		Vars: []string{
			"projector.eduEmail",
		},
		Impl: &persistentFixture{
			policyUserVar: "projector.eduEmail",
		},
		SetUpTimeout:    5 * time.Second,
		ResetTimeout:    5 * time.Second,
		TearDownTimeout: 5 * time.Second,
		PostTestTimeout: 5 * time.Second,
		Parent:          fixture.FakeDMS,
	})
}

type persistentFixture struct {
	// fdms is the FakeDMS instance managed by this fixture, coming from its parent.
	fdms *fakedms.FakeDMS

	// policies is the list of persistent policies set for FakeDMS.
	// Keep empty if unused.
	policies []policy.Policy
	// persistentPublicAccountPolicies contains persistent public account policies.
	persistentPublicAccountPolicies map[string][]policy.Policy

	// policyUser is the persistentuser account that used as policyUser in policy blob.
	// Keep nil if unused.
	policyUser *string
	// The policyUserVar is the account variable (i.e. "family.unicornEmail") when using
	// a different account instead of tast-user@managedchrome.com for policy test.
	// It is used to set the value of the policyUser variable above.
	policyUserVar string
}

func (p *persistentFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	fdms, ok := s.ParentValue().(*fakedms.FakeDMS)
	if !ok {
		s.Fatal("Parent is not a FakeDMS fixture")
	}

	p.fdms = fdms

	// Load policyUser from policyUserVar before using.
	if p.policyUserVar != "" {
		policyUser := s.RequiredVar(p.policyUserVar)
		p.policyUser = &policyUser
	}

	p.fdms.SetPersistentPolicies(p.policies)
	p.fdms.SetPersistentPublicAccountPolicies(p.persistentPublicAccountPolicies)
	p.fdms.SetPersistentPolicyUser(p.policyUser)

	// Write the policy blob with persistent values set as the one set by FakeDMS is the default.
	if err := p.fdms.WritePolicyBlob(policy.NewBlob()); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	// Forward fdms to children.
	return fdms
}

func (p *persistentFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	// Clear all persistent settings.
	p.fdms.SetPersistentPolicies([]policy.Policy{})
	p.fdms.SetPersistentPublicAccountPolicies(nil)
	p.fdms.SetPersistentPolicyUser(nil)
}

func (p *persistentFixture) Reset(ctx context.Context) error {
	return nil
}
func (p *persistentFixture) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (p *persistentFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}
