// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixtures

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"time"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "lacrosPolicyLoggedIn",
		Desc:            "Lacros Chrome logged into a user session",
		Contacts:        []string{"mohamedaomar@google.com", "chromeos-commercial-remote-management@google.com"},
		Impl:            &policyLacrosFixture{},
		SetUpTimeout:    chrome.ManagedUserLoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PostTestTimeout: 15 * time.Second,
		Parent:          "lacrosStartedByDataWithFDMS",
	})
}

type policyLacrosFixture struct {
	// cr is a connection to an already-started Chrome instance that loads policies from FakeDMS.
	cr *chrome.Chrome
	// fdms is the already running DMS server from the parent fixture.
	fdms *fakedms.FakeDMS
}

func (p *policyLacrosFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	fdms, ok := s.ParentValue().(*fakedms.FakeDMS)
	if !ok {
		s.Fatal("Parent is not a FakeDMS fixture")
	}
	p.fdms = fdms

	cr, ok := s.ParentValue().(*chrome.Chrome)
	if !ok {
		s.Fatal("Parent is not a Chrome fixture")
	}
	p.cr = cr

	return &FixtData{p.fdms, p.cr}
}

func (p *policyLacrosFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()

	if p.cr == nil {
		s.Fatal("Chrome not yet started")
	}

	if err := p.cr.Close(ctx); err != nil {
		s.Error("Failed to close Chrome connection: ", err)
	}

	p.cr = nil
}

func (p *policyLacrosFixture) Reset(ctx context.Context) error {
	// Check the connection to Chrome.
	if err := p.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}

	// The policy blob has already been cleared.
	if err := policyutil.RefreshChromePolicies(ctx, p.cr); err != nil {
		return errors.Wrap(err, "failed to clear policies")
	}

	// Reset Chrome state.
	if err := p.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}

	return nil
}

func (p *policyLacrosFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}
func (p *policyLacrosFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	tconn, err := p.cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create TestAPI connection: ", err)
	}

	policies, err := policyutil.PoliciesFromDUT(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain policies from Chrome: ", err)
	}

	b, err := json.MarshalIndent(policies, "", "  ")
	if err != nil {
		s.Fatal("Failed to marshal policies: ", err)
	}

	// Dump all policies as seen by Chrome to the tests OutDir.
	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), PolicyFileDump), b, 0644); err != nil {
		s.Error("Failed to dump policies to file: ", err)
	}
}
