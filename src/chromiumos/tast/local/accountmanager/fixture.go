// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package accountmanager

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
)

const resetTimeout = 30 * time.Second

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "loggedInToChromeAndArc",
		Desc: "Logged in using real Gaia account. ARC is booted with disabling sync flags. ArcAccountRestrictions feature is enabled",
		Contacts: []string{
			"anastasiian@chromium.org", "team-dent@google.com",
		},
		Impl:            &accountManagerTestFixture{},
		SetUpTimeout:    chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout + 2*time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		Vars: []string{
			"ui.gaiaPoolDefault",
		},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "loggedInToLacros",
		Desc: "Logged in using real Gaia account + with Lacros enabled. ARC is booted with disabling sync flags. ArcAccountRestrictions feature is enabled",
		Contacts: []string{
			"anastasiian@chromium.org", "team-dent@google.com",
		},
		Impl: lacrosfixt.NewFixture(lacrosfixt.Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
				chrome.EnableFeatures("ArcAccountRestrictions"),
				chrome.ARCSupported(),
				chrome.ExtraArgs(arc.DisableSyncFlags()...),
				chrome.ExtraArgs("--disable-lacros-keep-alive"),
			}, nil
		}),
		SetUpTimeout:    chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout + 2*time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		Vars: []string{
			"ui.gaiaPoolDefault",
			lacrosfixt.LacrosDeployedBinary,
		},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "loggedInToChromeAndArcWithLacros",
		Desc: "Logged in using real Gaia account + with Lacros enabled. ARC is booted with disabling sync flags. ArcAccountRestrictions feature is enabled",
		Contacts: []string{
			"anastasiian@chromium.org", "team-dent@google.com",
		},
		Impl:            &accountManagerTestFixture{},
		Parent:          "loggedInToLacros",
		SetUpTimeout:    chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout + 2*time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		Vars: []string{
			"ui.gaiaPoolDefault",
		},
	})
}

// Verify that *FixtureData implements lacrosfixt.FixtValue interface.
var _ lacrosfixt.FixtValue = (*FixtureData)(nil)

// FixtureData is the struct returned by the preconditions.
type FixtureData struct {
	cr         *chrome.Chrome
	ARC        *arc.ARC
	LacrosFixt lacrosfixt.FixtValue
}

// Chrome gets the CrOS-chrome instance.
// Implement chrome.HasChrome and lacrosfixt.FixtValue interface.
func (f FixtureData) Chrome() *chrome.Chrome {
	return f.cr
}

// TestAPIConn gets the CrOS-chrome test connection.
// Implement lacrosfixt.FixtValue interface.
func (f FixtureData) TestAPIConn() *chrome.TestConn {
	return f.LacrosFixt.TestAPIConn()
}

// Mode gets the mode used to get the lacros binary.
// Implement lacrosfixt.FixtValue interface.
func (f FixtureData) Mode() lacrosfixt.SetupMode {
	return f.LacrosFixt.Mode()
}

// LacrosPath gets the root directory for lacros-chrome.
// Implement lacrosfixt.FixtValue interface.
func (f FixtureData) LacrosPath() string {
	return f.LacrosFixt.LacrosPath()
}

// Options used to launch a CrOS chrome.
// Implement lacrosfixt.FixtValue interface.
func (f FixtureData) Options() []chrome.Option {
	return nil
}

// UserTmpDir returns the path to be used for Lacros's user data directory.
// This directory will be wiped on every reset call.
// We used to use generic tmp directory, and kept it until whole Tast run
// completes, but Lacros user data consumes more disk than other cases,
// and we hit out-of-diskspace on some devices which has very limited disk
// space. To avoid that problem, the user data will be wiped for each
// test run.
// Implement lacrosfixt.FixtValue interface.
func (f FixtureData) UserTmpDir() string {
	return f.LacrosFixt.UserTmpDir()
}

type accountManagerTestFixture struct {
	cr  *chrome.Chrome
	arc *arc.ARC
	// Whether chrome is created by parent fixture.
	useParentChrome bool
}

func (f *accountManagerTestFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	var cr *chrome.Chrome
	var lacrosFixt lacrosfixt.FixtValue

	if s.ParentValue() != nil {
		lacrosFixt = s.ParentValue().(lacrosfixt.FixtValue)
		cr = lacrosFixt.Chrome()
		f.useParentChrome = true
	} else {
		chromeLoginCtx, cancel := context.WithTimeout(ctx, chrome.LoginTimeout)
		defer cancel()

		opts := []chrome.Option{
			chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
			chrome.EnableFeatures("ArcAccountRestrictions"),
			chrome.ARCSupported(),
			chrome.ExtraArgs(arc.DisableSyncFlags()...),
		}

		var err error
		cr, err = chrome.New(chromeLoginCtx, opts...)
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
	}

	const playStorePackageName = "com.android.vending"
	optinCtx, cancel := context.WithTimeout(ctx, optin.OptinTimeout+time.Minute)
	defer cancel()

	// Optin to Play Store.
	s.Log("Opting into Play Store")
	tconn, err := cr.TestAPIConn(optinCtx)
	if err != nil {
		s.Fatal("Failed to get the test conn: ", err)
	}
	if err := optin.PerformAndClose(optinCtx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store and Close: ", err)
	}

	// Setup ARC.
	var a *arc.ARC
	if a, err = arc.New(ctx, s.OutDir()); err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}

	if !f.useParentChrome {
		chrome.Lock()
	}

	f.cr = cr
	f.arc = a
	cr = nil
	return FixtureData{cr: f.cr, ARC: f.arc, LacrosFixt: lacrosFixt}
}

func (f *accountManagerTestFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := f.arc.Close(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to close ARC connection: ", err)
	}

	if !f.useParentChrome {
		chrome.Unlock()
		if err := f.cr.Close(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to close Chrome connection: ", err)
		}
	}
}

func (f *accountManagerTestFixture) Reset(ctx context.Context) error {
	if !f.useParentChrome {
		if err := f.cr.ResetState(ctx); err != nil {
			return errors.Wrap(err, "failed to reset chrome")
		}
	}
	return nil
}

func (f *accountManagerTestFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {

}

func (f *accountManagerTestFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {

}
