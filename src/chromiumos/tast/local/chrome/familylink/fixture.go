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
	"chromiumos/tast/testing"
)

// resetTimeout is the timeout duration of trying to reset the current fixture.
const resetTimeout = 30 * time.Second

// NewFamilyLinkFixture creates a new implementation of the Family Link fixture.
func NewFamilyLinkFixture(parentUser, parentPassword, childUser, childPassword string, opts ...chrome.Option) testing.FixtureImpl {
	return &familyLinkFixture{
		opts:           opts,
		parentUser:     parentUser,
		parentPassword: parentPassword,
		childUser:      childUser,
		childPassword:  childPassword,
	}
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "familyLinkUnicornLogin",
		Desc: "Supervised Family Link user login with Unicorn account",
		Impl: NewFamilyLinkFixture("unicorn.parentUser", "unicorn.parentPassword", "unicorn.childUser", "unicorn.childPassword"),
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
		Name: "familyLinkUnicornArcLogin",
		Desc: "Supervised Family Link user login with Unicorn account and ARC support",
		Impl: NewFamilyLinkFixture("arc.parentUser", "arc.parentPassword", "arc.childUser", "arc.childPassword", chrome.ARCSupported()),
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
		Name: "familyLinkParentArcLogin",
		Desc: "Non-supervised Family Link user login with regular parent account and ARC support",
		Impl: NewFamilyLinkFixture("arc.parentUser", "arc.parentPassword", "", "", chrome.ARCSupported(), chrome.ExtraArgs(arc.DisableSyncFlags()...)),
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

	cr, err := chrome.New(ctx, f.opts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	f.cr = cr
	fixData := &FixtData{
		Chrome:   cr,
		TestConn: tconn,
	}
	return fixData
}

func (f *familyLinkFixture) TearDown(ctx context.Context, s *testing.FixtState) {
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
