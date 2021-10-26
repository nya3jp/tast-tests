// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
			policies: []policy.Policy{&policy.LacrosAvailability{Val: "side_by_side"}},
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
	policies []policy.Policy
}

func (p *persistentFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	fdms, ok := s.ParentValue().(*fakedms.FakeDMS)
	if !ok {
		s.Fatal("Parent is not a FakeDMS fixture")
	}

	p.fdms = fdms

	p.fdms.SetPersistentPolicies(p.policies)

	// Write the policy blob with persistent values set as the one set by FakeDMS is the default.
	p.fdms.WritePolicyBlob(fakedms.NewPolicyBlob())

	// Forward fdms to children.
	return fdms
}

func (p *persistentFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	// Clear all persistent settings.
	p.fdms.SetPersistentPolicies([]policy.Policy{})
}

func (p *persistentFixture) Reset(ctx context.Context) error {
	return nil
}
func (p *persistentFixture) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (p *persistentFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}
