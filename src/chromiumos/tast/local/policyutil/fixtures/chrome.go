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
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "chromePolicyLoggedIn",
		Desc:            "Logged into a user session",
		Contacts:        []string{"vsavu@google.com", "chromeos-commercial-remote-management@google.com"},
		Impl:            &policyChromeFixture{},
		SetUpTimeout:    chrome.ManagedUserLoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PostTestTimeout: 15 * time.Second,
		Parent:          "fakeDMS",
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeEnrolledLoggedIn",
		Desc:     "Logged into a user session with enrollment",
		Contacts: []string{"vsavu@google.com", "chromeos-commercial-remote-management@google.com"},
		Impl: &policyChromeFixture{
			extraOpts: []chrome.Option{chrome.KeepEnrollment()},
		},
		SetUpTimeout:    chrome.ManagedUserLoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PostTestTimeout: 15 * time.Second,
		Parent:          "fakeDMSEnrolled",
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeEnrolledLoggedInARC",
		Desc:     "Logged into a user session with enrollment with ARC support",
		Contacts: []string{"vsavu@google.com", "chromeos-commercial-remote-management@google.com"},
		Impl: &policyChromeFixture{
			extraOpts: []chrome.Option{chrome.KeepEnrollment(), chrome.ARCEnabled(),
				chrome.ExtraArgs("--arc-availability=officially-supported")},
			waitForARC: true,
		},
		SetUpTimeout:    chrome.ManagedUserLoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PostTestTimeout: 15 * time.Second,
		Parent:          "fakeDMSEnrolled",
	})
}

type policyChromeFixture struct {
	// cr is a connection to an already-started Chrome instance that loads policies from FakeDMS.
	cr *chrome.Chrome
	// fdms is the already running DMS server from the parent fixture.
	fdms *fakedms.FakeDMS

	// extraOpts contains extra options passed to Chrome.
	extraOpts []chrome.Option

	// waitForARC indicates the fixture needs to wait for ARC before login.
	// Only needs to be set if ARC is enabled.
	waitForARC bool
}

// FixtData is returned by the fixtures and used by tests to interact with Chrome and FakeDMS.
type FixtData struct {
	// FakeDMS is an already running DMS  server.
	FakeDMS *fakedms.FakeDMS
	// Chrome is a connection to an already-started Chrome instance that loads policies from FakeDMS.
	Chrome *chrome.Chrome
}

// Credentials used for authenticating the test user.
const (
	Username = "tast-user@managedchrome.com"
	Password = "test0000"
)

// PolicyFileDump is the filename where the state of policies is dumped after the test ends.
const PolicyFileDump = "policies.json"

func (p *policyChromeFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	fdms, ok := s.ParentValue().(*fakedms.FakeDMS)
	if !ok {
		s.Fatal("Parent is not a FakeDMS fixture")
	}

	p.fdms = fdms

	opts := []chrome.Option{
		chrome.FakeLogin(chrome.Creds{User: Username, Pass: Password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.CustomLoginTimeout(chrome.ManagedUserLoginTimeout),
		chrome.DeferLogin(),
	}
	opts = append(opts, p.extraOpts...)

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Chrome startup failed: ", err)
	}

	if p.waitForARC {
		if arcType, ok := arc.Type(); ok && arcType == arc.Container {
			// The ARC mini instance, created when the login screen is
			// shown, blocks session_manager, preventing it from responding
			// to D-Bus methods. Cloud policy initialisation relies on being
			// able to contact session_manager, otherwise initialisation
			// will time out.
			err = arc.WaitAndroidInit(ctx)
			if err != nil {
				s.Fatal("Failed waiting for Android init: ", err)
			}
		}
	}

	err = cr.ContinueLogin(ctx)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	p.cr = cr

	chrome.Lock()

	return &FixtData{p.fdms, p.cr}
}

func (p *policyChromeFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()

	if p.cr == nil {
		s.Fatal("Chrome not yet started")
	}

	if err := p.cr.Close(ctx); err != nil {
		s.Error("Failed to close Chrome connection: ", err)
	}

	p.cr = nil
}

func (p *policyChromeFixture) Reset(ctx context.Context) error {
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

func (p *policyChromeFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}
func (p *policyChromeFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
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
