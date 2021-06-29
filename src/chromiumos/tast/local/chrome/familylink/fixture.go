// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package familylink provides Family Link user login functions.
package familylink

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// resetTimeout is the timeout duration of trying to reset the current fixture.
const resetTimeout = 30 * time.Second

// NewFamilyLinkFixture creates a new implementation of the Family Link fixture.
func NewFamilyLinkFixture(parentUser, parentPassword, childUser, childPassword string, isOwner bool, opts ...chrome.Option) testing.FixtureImpl {
	return &familyLinkFixture{
		opts:           opts,
		parentUser:     parentUser,
		parentPassword: parentPassword,
		childUser:      childUser,
		childPassword:  childPassword,
		isOwner:        isOwner,
	}
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     "familyLinkUnicornLogin",
		Desc:     "Supervised Family Link user login with Unicorn account",
		Contacts: []string{"tobyhuang@chromium.org", "cros-families-eng@google.com"},
		Impl:     NewFamilyLinkFixture("unicorn.parentUser", "unicorn.parentPassword", "unicorn.childUser", "unicorn.childPassword", true),
		Vars: []string{
			"unicorn.parentUser",
			"unicorn.parentPassword",
			"unicorn.childUser",
			"unicorn.childPassword",
		},
		SetUpTimeout:    chrome.GAIALoginTimeout + time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "familyLinkUnicornLoginNonOwner",
		Desc:     "Supervised Family Link user login with Unicorn account as second user on device",
		Contacts: []string{"tobyhuang@chromium.org", "cros-families-eng@google.com"},
		Impl:     NewFamilyLinkFixture("unicorn.parentUser", "unicorn.parentPassword", "unicorn.childUser", "unicorn.childPassword", false),
		Vars: []string{
			"ui.gaiaPoolDefault",
			"unicorn.parentUser",
			"unicorn.parentPassword",
			"unicorn.childUser",
			"unicorn.childPassword",
		},
		SetUpTimeout:    chrome.GAIALoginTimeout + time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "familyLinkGellerLogin",
		Desc:     "Supervised Family Link user login with Geller account",
		Contacts: []string{"tobyhuang@chromium.org", "cros-families-eng@google.com"},
		Impl:     NewFamilyLinkFixture("geller.parentUser", "geller.parentPassword", "geller.childUser", "geller.childPassword", true),
		Vars: []string{
			"geller.parentUser",
			"geller.parentPassword",
			"geller.childUser",
			"geller.childPassword",
		},
		SetUpTimeout:    chrome.GAIALoginTimeout + time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "familyLinkUnicornArcLogin",
		Desc:     "Supervised Family Link user login with Unicorn account and ARC support",
		Contacts: []string{"tobyhuang@chromium.org", "cros-families-eng@google.com"},
		Impl:     NewFamilyLinkFixture("arc.parentUser", "arc.parentPassword", "arc.childUser", "arc.childPassword", true, chrome.ARCSupported()),
		Vars: []string{
			"arc.parentUser",
			"arc.parentPassword",
			"arc.childUser",
			"arc.childPassword",
		},
		SetUpTimeout:    chrome.GAIALoginTimeout + arc.BootTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "familyLinkParentArcLogin",
		Desc:     "Non-supervised Family Link user login with regular parent account and ARC support",
		Contacts: []string{"tobyhuang@chromium.org", "cros-families-eng@google.com"},
		Impl:     NewFamilyLinkFixture("arc.parentUser", "arc.parentPassword", "", "", true, chrome.ARCSupported(), chrome.ExtraArgs(arc.DisableSyncFlags()...)),
		Vars: []string{
			"arc.parentUser",
			"arc.parentPassword",
		},
		SetUpTimeout:    chrome.GAIALoginTimeout + arc.BootTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})
}

type familyLinkFixture struct {
	cr             *chrome.Chrome
	opts           []chrome.Option
	parentUser     string
	parentPassword string
	childUser      string
	childPassword  string
	isOwner        bool
}

// FixtData holds information made available to tests that specify this Fixture.
type FixtData struct {
	// Chrome is the running chrome instance.
	Chrome *chrome.Chrome

	// TestConn is a connection to the test extension.
	TestConn *chrome.TestConn
}

func (f *familyLinkFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	parentUser := s.RequiredVar(f.parentUser)
	parentPass := s.RequiredVar(f.parentPassword)
	if len(f.childUser) > 0 && len(f.childPassword) > 0 {
		childUser := s.RequiredVar(f.childUser)
		childPass := s.RequiredVar(f.childPassword)
		f.opts = append(f.opts, chrome.GAIALogin(chrome.Creds{
			User:       childUser,
			Pass:       childPass,
			ParentUser: parentUser,
			ParentPass: parentPass,
		}))
	} else {
		f.opts = append(f.opts, chrome.GAIALogin(chrome.Creds{
			User: parentUser,
			Pass: parentPass,
		}))
	}

	if !f.isOwner {
		func() {
			// Log in and log out to create a user pod on the login screen.
			cr, err := chrome.New(ctx, chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")))
			if err != nil {
				s.Fatal("Chrome login failed: ", err)
			}
			defer cr.Close(ctx)

			if err := upstart.RestartJob(ctx, "ui"); err != nil {
				s.Fatal("Failed to restart ui: ", err)
			}
		}()

		// chrome.KeepState() is needed to show the login screen with a user pod (instead of the OOBE login screen).
		f.opts = append(f.opts, chrome.KeepState())
	}

	cr, err := chrome.New(ctx, f.opts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	f.cr = cr
	fixtData := &FixtData{
		Chrome:   cr,
		TestConn: tconn,
	}
	// Lock chrome after all Setup is complete so we don't block other fixtures.
	chrome.Lock()

	return fixtData
}

func (f *familyLinkFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()

	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	f.cr = nil
}

func (f *familyLinkFixture) Reset(ctx context.Context) error {
	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}
	return nil
}

func (f *familyLinkFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *familyLinkFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}
