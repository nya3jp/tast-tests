// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fixtures contains fixtures useful for Kiosk mode tests.
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
	"chromiumos/tast/local/chrome/chromeproc"
	"chromiumos/tast/local/kioskmode"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     "kioskLoggedIn",
		Desc:     "Kiosk mode started with default app setup, DUT is enrolled",
		Contacts: []string{"kamilszarek@google.com", "alt-modalities-stability@google.com"},
		Impl: &kioskFixture{
			autoLaunchKioskAppID: kioskmode.WebKioskAccountID,
		},
		SetUpTimeout:    chrome.ManagedUserLoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PostTestTimeout: 15 * time.Second,
		Parent:          "fakeDMSEnrolled",
	})
}

type kioskFixture struct {
	// cr is a connection to an already-started Chrome instance that loads policies from FakeDMS.
	cr *chrome.Chrome
	// fdms is the already running DMS server from the parent fixture.
	fdms *fakedms.FakeDMS
	// extraOpts contains extra options passed to Chrome.
	extraOpts []chrome.Option
	// autoLaunchKioskAppID is a preselected Kiosk app ID used for autolaunch.
	autoLaunchKioskAppID string
	// oldPID is a pid of the Chrome started in Kiosk mode.
	oldPID int
}

// FixtData is returned by the fixtures and used by tests to interact with Chrome and FakeDMS.
type FixtData struct {
	// FakeDMS is an already running DMS  server.
	FakeDMS *fakedms.FakeDMS
	// Chrome is a connection to an already-started Chrome instance that loads policies from FakeDMS.
	Chrome *chrome.Chrome
}

func (p *kioskFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	fdms, ok := s.ParentValue().(*fakedms.FakeDMS)
	if !ok {
		s.Fatal("Parent is not a fakeDMSEnrolled fixture")
	}

	p.fdms = fdms

	func(ctx context.Context) {
		initialOpts := []chrome.Option{
			chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
			chrome.DMSPolicy(fdms.URL),
			chrome.CustomLoginTimeout(chrome.ManagedUserLoginTimeout),
			chrome.KeepEnrollment(),
		}
		// Start the first Chrome instance that will fetch policies from the
		// FakeDMS.
		testing.ContextLog(ctx, "kioskLoggedIn - starting first Chrome instance")
		cr, err := chrome.New(ctx, initialOpts...)
		if err != nil {
			s.Fatal("Chrome login failed: ", err)
		}

		// Prepare setup for Kiosk mode with autolaunch - set needed policies.
		if err := kioskmode.SetAutolaunch(ctx, fdms, cr, p.autoLaunchKioskAppID); err != nil {
			s.Fatal("Failed to update policies with Kiosk configuration: ", err)
		}

		// Close the first Chrome instance.
		defer cr.Close(ctx)
	}(ctx)

	// Reader will be used to check if Kiosk has started successfully.
	reader, err := syslog.NewReader(ctx, syslog.Program(syslog.Chrome))
	if err != nil {
		s.Fatal("Failed to start log reader: ", err)
	}
	defer reader.Close()

	opts := []chrome.Option{
		chrome.DMSPolicy(fdms.URL),
		chrome.NoLogin(),
		chrome.KeepEnrollment(),
	}
	opts = append(opts, p.extraOpts...)

	// Restart Chrome with this Kiosk auto starts.
	testing.ContextLog(ctx, "kioskLoggedIn - starting second Chrome instance. Launching Kiosk mode")
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Chrome restart failed: ", err)
	}

	// Make sure Kiosk has successfully started.
	if err := kioskmode.ConfirmKioskStarted(ctx, reader); err != nil {
		cr.Close(ctx)
		s.Fatal("Problem while checking Chrome logs for Kiosk related entries: ", err)
	}

	pid, err := chromeproc.GetRootPID()
	if err != nil {
		cr.Close(ctx)
		s.Fatal("Failed to get root Chrome PID: ", err)
	}

	chrome.Lock()
	p.cr = cr
	p.oldPID = pid
	return &FixtData{p.fdms, p.cr}
}

func (p *kioskFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()

	if p.cr == nil {
		s.Fatal("Chrome not yet started")
	}

	if err := p.cr.Close(ctx); err != nil {
		s.Error("Failed to close Chrome connection: ", err)
	}

	p.cr = nil
}

func (p *kioskFixture) Reset(ctx context.Context) error {
	// Check the connection to Chrome.
	if err := p.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}

	// Get Chrome PID and check if it stayed the same
	pid, err := chromeproc.GetRootPID()
	if err != nil {
		return errors.Wrap(err, "failed to get root Chrome PID")
	}

	if p.oldPID != pid {
		return errors.New("chrome PID while running in Kiosk mode has changed")
	}
	return nil
}

func (p *kioskFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}
func (p *kioskFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
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
	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), fixtures.PolicyFileDump), b, 0644); err != nil {
		s.Error("Failed to dump policies to file: ", err)
	}
}
