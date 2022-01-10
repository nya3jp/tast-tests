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

// FixtData holds chrome instance and session login time.
type FixtData struct {
	// chrome is a connection to an already-started Chrome instance that loads policies from mgs.
	chrome *chrome.Chrome
	// loginTime is the time duration about session login time.
	loginTime time.Duration
}

// Chrome returns the chrome instance.
func (f FixtData) Chrome() *chrome.Chrome {
	if f.chrome == nil {
		panic("Chrome is called with nil chrome instance")
	}
	return f.chrome
}

// LoginTime returns the duration of the login session.
func (f FixtData) LoginTime() time.Duration {
	if f.loginTime == 0 {
		panic("LoginTime has not been recorded")
	}
	return f.loginTime
}

type guestSessionFixture struct {
	// MGS holds chrome and fakedms instances.
	mgs *MGS
	// extensions contains extensions to be installed to the session.
	extensions []string
}

func (g *guestSessionFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	fdms, ok := s.ParentValue().(*fakedms.FakeDMS)
	if !ok {
		s.Fatal("Parent is not a FakeDMS fixture")
	}

	reader, err := syslog.NewReader(ctx)
	if err != nil {
		s.Fatal("Failed to open syslog reader: ", err)
	}
	defer reader.Close()

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	mgs, cr, err := New(
		ctx,
		fdms,
		DefaultAccount(),
		ExtraPolicies([]policy.Policy{&policy.DeviceLoginScreenExtensions{
			Val: []string{LoginScreenExtensionID},
		}}),
		AddPublicAccountPolicies(MgsAccountID, []policy.Policy{
			&policy.ExtensionInstallForcelist{
				Val: append(g.extensions, InSessionExtensionID),
			},
		}),
		ExtraChromeOptions(
			chrome.KeepState(),
			chrome.ExtraArgs("--force-devtools-available"),
		),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome on Signin screen with default MGS account: ", err)
	}

	g.mgs = mgs

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

	chrome.Lock()

	return &FixtData{cr, loginTime}
}

func (g *guestSessionFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()

	if g.mgs != nil {
		if err := g.mgs.Close(ctx); err != nil {
			s.Error("Failed to close MGS: ", err)
		}
	}
}

func (g *guestSessionFixture) Reset(ctx context.Context) error {
	cr := g.mgs.Chrome()

	// Check the connection to Chrome.
	if err := cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}

	// The policy blob has already been cleared.
	if err := policyutil.RefreshChromePolicies(ctx, cr); err != nil {
		return errors.Wrap(err, "failed to clear policies")
	}

	// Reset Chrome state.
	if err := cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}

	return nil
}

func (g *guestSessionFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}
func (g *guestSessionFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	cr := g.mgs.Chrome()

	tconn, err := cr.TestAPIConn(ctx)
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
