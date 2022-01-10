// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mgs

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

// accountID indicates that the account for the managed guest session.
const accountID = "foo@bar.com"

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            fixture.ManagedGuestSession,
		Desc:            "Fixture to log into a managed guest session",
		Contacts:        []string{"alston.huang@cienet.com", "chromeos-perfmetrics-eng@google.com"},
		Impl:            &guestSessionFixture{},
		SetUpTimeout:    chrome.ManagedUserLoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PostTestTimeout: 15 * time.Second,
		Parent:          fixture.FakeDMSEnrolled,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     fixture.ManagedGuestSessionWithExtensions,
		Desc:     "Fixture to log into a managed guest session with extensions installed",
		Contacts: []string{"alston.huang@cienet.com", "chromeos-perfmetrics-eng@google.com"},
		Impl: &guestSessionFixture{
			extensions: []string{apps.Drive.ID},
		},
		SetUpTimeout:    chrome.ManagedUserLoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PostTestTimeout: 15 * time.Second,
		Parent:          fixture.FakeDMSEnrolled,
	})
}

// FixtData embeds fixtures.FixtData with session login time.
type FixtData struct {
	*fixtures.FixtData
	// loginTime is the time duration about session login time.
	loginTime time.Duration
}

// LoginTime returns the duration of the login session.
func (f FixtData) LoginTime() time.Duration {
	if f.loginTime == 0 {
		panic("LoginTime has not been recorded")
	}
	return f.loginTime
}

type guestSessionFixture struct {
	// cr is a connection to an already-started Chrome instance that loads policies from FakeDMS.
	cr *chrome.Chrome
	// fdms is the already running DMS server from the parent fixture.
	fdms *fakedms.FakeDMS

	// extraOpts contains extra options passed to Chrome.
	extraOpts []chrome.Option
	// extensions contains extensions to be installed to the session.
	extensions []string
}

func (g *guestSessionFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	fdms, ok := s.ParentValue().(*fakedms.FakeDMS)
	if !ok {
		s.Fatal("Parent is not a FakeDMS fixture")
	}
	g.fdms = fdms

	reader, err := syslog.NewReader(ctx)
	if err != nil {
		s.Fatal("Failed to open syslog reader: ", err)
	}
	defer reader.Close()

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx,
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepState())
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	policies := DefaultPolicies(accountID)

	pb := fakedms.NewPolicyBlob()
	if err := pb.AddPolicies(policies); err != nil {
		s.Fatal("Failed to add policies: ", err)
	}
	if err := pb.AddPublicAccountPolicy(accountID, &policy.ExtensionInstallForcelist{
		Val: g.extensions,
	}); err != nil {
		s.Fatal("Failed to add public account policy: ", err)
	}
	if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		s.Fatal("Failed to serve policies: ", err)
	}

	// Close the previous Chrome instance.
	if err := cr.Close(ctx); err != nil {
		s.Fatal("Failed to close Chrome connection: ", err)
	}

	// Restart Chrome, forcing Devtools to be available on the login screen.
	opts := []chrome.Option{
		chrome.NoLogin(),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepState(),
		chrome.ExtraArgs("--force-devtools-available"),
	}
	opts = append(opts, g.extraOpts...)

	cr, err = chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Chrome startup failed: ", err)
	}

	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		s.Fatal("Failed to connect to session manager: ", err)
	}

	sw, err := sm.WatchSessionStateChanged(ctx, "started")
	if err != nil {
		s.Fatal("Failed to watch for D-Bus signals: ", err)
	}
	defer sw.Close(cleanupCtx)

	loginScreenBGURL := chrome.ExtensionBackgroundPageURL(LoginScreenExtensionID)
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(loginScreenBGURL))
	if err != nil {
		s.Fatal("Failed to connect to login screen background page: ", err)
	}

	startTime := time.Now()
	if err := conn.Eval(ctx, `new Promise((resolve, reject) => {
		chrome.login.launchManagedGuestSession(() => {
			if (chrome.runtime.lastError) {
				reject(new Error(chrome.runtime.lastError.message));
				return;
			}
			resolve();
		});
	})`, nil); err != nil {
		s.Fatal("Failed to launch MGS: ", err)
	}

	var loginTime time.Duration
	select {
	case <-sw.Signals:
		loginTime = time.Since(startTime)
	case <-ctx.Done():
		s.Fatal("Timeout before getting SessionStateChanged signal: ", err)
	}

	g.cr = cr

	chrome.Lock()

	return &FixtData{fixtures.NewFixtData(g.cr, g.fdms), loginTime}
}

func (g *guestSessionFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()

	if g.cr != nil {
		if err := g.cr.Close(ctx); err != nil {
			s.Error("Failed to close Chrome connection: ", err)
		}
		g.cr = nil
	}
}

func (g *guestSessionFixture) Reset(ctx context.Context) error {
	// Check the connection to Chrome.
	if err := g.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}

	// The policy blob has already been cleared.
	if err := policyutil.RefreshChromePolicies(ctx, g.cr); err != nil {
		return errors.Wrap(err, "failed to clear policies")
	}

	// Reset Chrome state.
	if err := g.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}

	return nil
}

func (g *guestSessionFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}
func (g *guestSessionFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	tconn, err := g.cr.TestAPIConn(ctx)
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
