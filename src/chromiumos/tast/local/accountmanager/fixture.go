// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package accountmanager

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/logsaver"
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
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return lacrosfixt.NewConfig(lacrosfixt.ChromeOptions(
				chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
				chrome.EnableFeatures("ArcAccountRestrictions"),
				chrome.ARCSupported(),
				chrome.ExtraArgs(arc.DisableSyncFlags()...))).Opts()
		}),
		SetUpTimeout:    chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout + 2*time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		Vars:            []string{"ui.gaiaPoolDefault"},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "loggedInToChromeAndArcWithLacros",
		Desc: "Logged in using real Gaia account + with Lacros enabled. ARC is booted with disabling sync flags. ArcAccountRestrictions feature is enabled",
		Contacts: []string{
			"anastasiian@chromium.org", "team-dent@google.com",
		},
		Impl:            &accountManagerTestFixture{isLacros: true},
		SetUpTimeout:    chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout + 2*time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		Vars:            []string{"ui.gaiaPoolDefault"},
	})
}

// FixtureData is the struct returned by the preconditions.
type FixtureData struct {
	cr  *chrome.Chrome
	ARC *arc.ARC
}

// Chrome gets the CrOS-chrome instance.
// Implements the chrome.HasChrome interface.
func (f FixtureData) Chrome() *chrome.Chrome {
	return f.cr
}

type accountManagerTestFixture struct {
	cr       *chrome.Chrome
	arc      *arc.ARC
	isLacros bool
	// Marker for per-test log.
	logMarker *logsaver.Marker
}

func (f *accountManagerTestFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	chromeLoginCtx, cancel := context.WithTimeout(ctx, chrome.LoginTimeout)
	defer cancel()

	opts := []chrome.Option{
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.EnableFeatures("ArcAccountRestrictions"),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...),
	}

	if f.isLacros {
		var err error
		opts, err = lacrosfixt.NewConfig(lacrosfixt.ChromeOptions(opts...)).Opts()
		if err != nil {
			s.Fatal("Failed to get lacros options: ", err)
		}
	}

	cr, err := chrome.New(chromeLoginCtx, opts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	logMarker, err := logsaver.NewMarker(cr.LogFilename())
	if err != nil {
		s.Error("Failed to start the log saver: ", err)
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

	chrome.Lock()

	if logMarker != nil {
		if err := logMarker.Save(filepath.Join(s.OutDir(), "chrome.fixture.log")); err != nil {
			s.Error("Failed to store fixture log data: ", err)
		}
	}

	f.cr = cr
	f.arc = a
	cr = nil
	return FixtureData{cr: f.cr, ARC: f.arc}
}

func (f *accountManagerTestFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := f.arc.Close(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to close ARC connection: ", err)
	}

	chrome.Unlock()
	if err := f.cr.Close(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to close Chrome connection: ", err)
	}
}

func (f *accountManagerTestFixture) Reset(ctx context.Context) error {
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed to reset chrome")
	}

	return nil
}

func (f *accountManagerTestFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	if f.logMarker != nil {
		s.Log("A log marker is already created but not cleaned up")
		f.logMarker = nil
	}

	logMarker, err := logsaver.NewMarker(f.cr.LogFilename())
	if err != nil {
		s.Error("Failed to start the log saver: ", err)
	} else {
		f.logMarker = logMarker
	}
}

func (f *accountManagerTestFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if f.logMarker != nil {
		if err := f.logMarker.Save(filepath.Join(s.OutDir(), "chrome.log")); err != nil {
			s.Error("Failed to store per-test log data: ", err)
		}
		f.logMarker = nil
	}
}
