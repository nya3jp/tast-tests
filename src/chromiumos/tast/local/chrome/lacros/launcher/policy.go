// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"time"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
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
	fdms       *fakedms.FakeDMS
	lacrosPath string           // Root directory for lacros-chrome, it's dynamically controlled by the lacros.skipInstallation Var.
	tconn      *chrome.TestConn // Test-connection for CrOS-chrome.
}

func (p *policyLacrosFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	p.fdms = s.ParentValue().(*FixtData).FakeDMS
	p.cr = s.ParentValue().(*FixtData).Chrome
	p.lacrosPath = s.ParentValue().(*FixtData).LacrosPath
	p.tconn = s.ParentValue().(*FixtData).TestAPIConn

	return &FixtData{FakeDMS: p.fdms, Chrome: p.cr, LacrosPath: p.lacrosPath, TestAPIConn: p.tconn}
}

func (p *policyLacrosFixture) TearDown(ctx context.Context, s *testing.FixtState) {}

func (p *policyLacrosFixture) cleanUp(ctx context.Context, s *testing.FixtState) {}

func (p *policyLacrosFixture) Reset(ctx context.Context) error {
	return nil
}

func (p *policyLacrosFixture) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (p *policyLacrosFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}
