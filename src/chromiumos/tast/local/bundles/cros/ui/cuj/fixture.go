// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const resetTimeout = 30 * time.Second

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "loggedInToCUJUser",
		Desc:            "The main fixture used for UI CUJ tests",
		Contacts:        []string{"mukai@chromium.org"},
		Impl:            &loggedInToCUJUserFixture{},
		SetUpTimeout:    chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout + 2*time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		Vars:            []string{"ui.cuj_username", "ui.cuj_password"},
	})
}

func runningPackages(ctx context.Context, a *arc.ARC) (map[string]struct{}, error) {
	tasks, err := a.DumpsysActivityActivities(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "listing activities failed")
	}

	acts := make(map[string]struct{})
	for _, t := range tasks {
		for _, a := range t.ActivityInfos {
			acts[a.PackageName] = struct{}{}
		}
	}
	return acts, nil
}

// FixtureData is the struct returned by the preconditions.
type FixtureData struct {
	Chrome *chrome.Chrome
	ARC    *arc.ARC
}

type loggedInToCUJUserFixture struct {
	cr              *chrome.Chrome
	arc             *arc.ARC
	origRunningPkgs map[string]struct{}
}

func (f *loggedInToCUJUserFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	var cr *chrome.Chrome

	func() {
		ctx, cancel := context.WithTimeout(ctx, chrome.LoginTimeout)
		defer cancel()
		var err error
		username := s.RequiredVar("ui.cuj_username")
		password := s.RequiredVar("ui.cuj_password")

		var opts = []chrome.Option{
			chrome.GAIALogin(),
			chrome.Auth(username, password, "gaia-id"),
			chrome.ARCSupported(),
			chrome.KeepState(),
			chrome.ExtraArgs(arc.DisableSyncFlags()...),
		}
		cr, err = chrome.New(ctx, opts...)
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
		chrome.Lock()
	}()
	defer func() {
		if cr != nil {
			chrome.Unlock()
			if err := cr.Close(ctx); err != nil {
				s.Error("Failed to close Chrome: ", err)
			}
		}
	}()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	if st, err := arc.GetState(ctx, tconn); err != nil {
		s.Fatal("Failed to get ARC state: ", err)
	} else if st.Provisioned {
		s.Log("ARC is already provisioned. Skipping the setup")
	} else {
		// Optin to Play Store.
		s.Log("Opting into Play Store")
		if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
			s.Fatal("Failed to opt into Play Store: ", err)
		}
	}

	var a *arc.ARC
	func() {
		ctx, cancel := context.WithTimeout(ctx, arc.BootTimeout)
		defer cancel()

		var err error
		if a, err = arc.New(ctx, s.OutDir()); err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}

		if f.origRunningPkgs, err = runningPackages(ctx, a); err != nil {
			if err := a.Close(); err != nil {
				s.Error(ctx, "Failed to close ARC connection: ", err)
			}
			s.Fatal("Failed to list running packages: ", err)
		}
	}()
	f.cr = cr
	f.arc = a
	cr = nil
	return FixtureData{Chrome: f.cr, ARC: f.arc}
}

func (f *loggedInToCUJUserFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()

	if err := f.arc.Close(); err != nil {
		testing.ContextLog(ctx, "Failed to close ARC connection: ", err)
	}

	if err := f.cr.Close(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to close Chrome connection: ", err)
	}
}

func (f *loggedInToCUJUserFixture) Reset(ctx context.Context) error {
	// Stopping the running apps.
	running, err := runningPackages(ctx, f.arc)
	if err != nil {
		return errors.Wrap(err, "failed to get running packages")
	}
	for pkg := range running {
		if _, ok := f.origRunningPkgs[pkg]; ok {
			continue
		}
		testing.ContextLogf(ctx, "Stopping package %q", pkg)
		if err := f.arc.Command(ctx, "am", "force-stop", pkg).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to stop %q", pkg)
		}
	}

	// Unlike ARC.preImpl, this does not uninstall apps. This is because we
	// typically want to reuse the same list of applications, and additional
	// installed apps wouldn't affect the test scenarios.
	if err = f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed to reset chrome")
	}

	// Ensures that there are no toplevel windows left open.
	tconn, err := f.cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the test conn")
	}
	if all, err := ash.GetAllWindows(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to call ash.GetAllWindows")
	} else if len(all) != 0 {
		return errors.Wrapf(err, "toplevel window (%q) stayed open, total %d left", all[0].Name, len(all))
	}

	return nil
}

func (f *loggedInToCUJUserFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *loggedInToCUJUserFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}
