// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package accountmanager

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros/launcher"
	"chromiumos/tast/testing"
)

const resetTimeout = 30 * time.Second

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "loggedInToChromeAndArc",
		Desc: "Logged in using real Gaia account. ARC is booted with disabling sync flags",
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
		Desc: "Logged in using real Gaia account + with Lacros enabled. ARC is booted with disabling sync flags",
		Contacts: []string{
			"anastasiian@chromium.org", "team-dent@google.com",
		},
		Impl: launcher.NewFixture(launcher.Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
				chrome.ARCSupported(),
				chrome.ExtraArgs(arc.DisableSyncFlags()...),
			}, nil
		}),
		SetUpTimeout:    chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout + 2*time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		Vars: []string{
			"ui.gaiaPoolDefault",
			launcher.LacrosDeployedBinary,
		},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "loggedInToChromeAndArcWithLacros",
		Desc: "Logged in using real Gaia account + with Lacros enabled. ARC is booted with disabling sync flags",
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

// FixtureData is the struct returned by the preconditions.
type FixtureData struct {
	Chrome     *chrome.Chrome
	ARC        *arc.ARC
	LacrosFixt launcher.FixtValue
}

type accountManagerTestFixture struct {
	cr  *chrome.Chrome
	arc *arc.ARC
	// Whether chrome is created by parent fixture.
	useParentChrome bool
}

func (f *accountManagerTestFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	var cr *chrome.Chrome
	var lacrosFixt launcher.FixtValue

	if s.ParentValue() != nil {
		lacrosFixt = s.ParentValue().(launcher.FixtValue)
		cr = lacrosFixt.Chrome()
		f.useParentChrome = true
	} else {
		func() {
			ctx, cancel := context.WithTimeout(ctx, chrome.LoginTimeout)
			defer cancel()

			opts := []chrome.Option{
				chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
				chrome.ARCSupported(),
				chrome.ExtraArgs(arc.DisableSyncFlags()...),
			}

			var err error
			cr, err = chrome.New(ctx, opts...)

			if err != nil {
				s.Fatal("Failed to start Chrome: ", err)
			}
		}()
		defer func() {
			if cr != nil {
				if err := cr.Close(ctx); err != nil {
					s.Error("Failed to close Chrome: ", err)
				}
			}
		}()
	}

	const playStorePackageName = "com.android.vending"
	ctx, cancel := context.WithTimeout(ctx, optin.OptinTimeout+time.Minute)
	defer cancel()

	// Optin to Play Store.
	s.Log("Opting into Play Store")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get the test conn: ", err)
	}
	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store and Close: ", err)
	}

	// Setup ARC.
	var a *arc.ARC
	func() {
		ctx, cancel := context.WithTimeout(ctx, arc.BootTimeout)
		defer cancel()

		var err error
		if a, err = arc.New(ctx, s.OutDir()); err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}

	}()

	f.cr = cr
	f.arc = a
	cr = nil
	return FixtureData{Chrome: f.cr, ARC: f.arc, LacrosFixt: lacrosFixt}
}

func (f *accountManagerTestFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := f.arc.Close(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to close ARC connection: ", err)
	}

	if !f.useParentChrome {
		if err := f.cr.Close(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to close Chrome connection: ", err)
		}
	}
}

func (f *accountManagerTestFixture) Reset(ctx context.Context) error {
	return nil
}

func (f *accountManagerTestFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {

}

func (f *accountManagerTestFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {

}
