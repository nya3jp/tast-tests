// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixtures

import (
	"context"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "chromePolicyLoggedIn",
		Desc:            "Logged into a user session",
		Contacts:        []string{"vsavu@google.com", "chromeos-commercial-stability@google.com"},
		Impl:            &policyChromeFixture{},
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Parent:          "fakeDMS",
	})
}

type policyChromeFixture struct {
	// Chrome is a connection to an already-started Chrome instance that loads policies from FakeDMS.
	cr *chrome.Chrome
	// FakeDMS is the already running DMS server from the parent fixture.
	fdms *fakedms.FakeDMS
}

// FixtData is returned by the fixtures and used by tests to interact with Chrome and FakeDMS.
type FixtData struct {
	// FakeDMS is an already running DMS  server.
	FakeDMS *fakedms.FakeDMS
	// Chrome is a connection to an already-started Chrome instance that loads policies from FakeDMS.
	Chrome *chrome.Chrome

	fixture *policyChromeFixture
}

// ResetChrome will attempt to reset Chrome. If the fast reset fails, Chrome will
// be fully restarted.
func (f *FixtData) ResetChrome(ctx context.Context) error {
	// Clear policy blob.
	if err := f.FakeDMS.WritePolicyBlob(fakedms.NewPolicyBlob()); err != nil {
		return errors.Wrap(err, "failed to clear policies in FakeDMS")
	}

	resetCtx, cancel := context.WithTimeout(ctx, chrome.ResetTimeout)
	defer cancel()

	// Reset Chrome with a timeout to ensure restart has enough time to complete.
	if err := f.fixture.Reset(resetCtx); err != nil {
		testing.ContextLog(ctx, "Chrome reset failed, restarting: ", err)

		chrome.Unlock()
		defer chrome.Lock()

		if err := f.Chrome.Close(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to close existing Chrome session: ", err)
		}

		// Don't leave a broken Chrome behind.
		f.fixture.cr = nil
		f.Chrome = nil

		// Restart Chrome.
		cr, err := chrome.New(ctx,
			chrome.Auth(Username, Password, GaiaID),
			chrome.DMSPolicy(f.FakeDMS.URL))
		if err != nil {
			return errors.Wrap(err, "failed to restart Chrome")
		}

		// Switch to new Chrome instance.
		f.fixture.cr = cr
		f.Chrome = cr
	}

	return nil
}

// Credentials used for authenticating the test user.
const (
	Username = "tast-user@managedchrome.com"
	Password = "test0000"
	GaiaID   = "gaia-id"
)

func (p *policyChromeFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	fdms, ok := s.ParentValue().(*fakedms.FakeDMS)
	if !ok {
		s.Fatal("Parent is not a FakeDMS fixture")
	}

	p.fdms = fdms

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx,
		chrome.Auth(Username, Password, GaiaID),
		chrome.DMSPolicy(fdms.URL))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	p.cr = cr

	chrome.Lock()

	return &FixtData{p.fdms, p.cr, p}
}

func (p *policyChromeFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()

	if p.cr == nil {
		s.Log("Chrome not started before TearDown")
		return
	}

	if err := p.cr.Close(ctx); err != nil {
		s.Error("Failed to close Chrome connection: ", err)
	}

	p.cr = nil
}

func (p *policyChromeFixture) Reset(ctx context.Context) error {
	if p.cr == nil {
		return errors.New("Chrome not started")
	}

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
	// TODO(crbug.com/1049532): Copy policy.json after each test finishes.
}
