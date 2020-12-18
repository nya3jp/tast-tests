// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
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
		SetUpTimeout:    chrome.LoginTimeout + optin.OptinTimeout + arc.BootTimeout + 10*time.Second,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		Vars:            []string{"ui.cuj_username", "ui.cuj_password"},
	})
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
	ctx, cancel := context.WithTimeout(ctx, chrome.LoginTimeout)
	defer cancel()
	var err error
	username := s.RequiredVar("ui.cuj_username")
	password := s.RequiredVar("ui.cuj_password")
	f.cr, err = chrome.New(ctx, chrome.GAIALogin(), chrome.Auth(username, password, "gaia-id"), chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	chrome.Lock()

	func() {
		const playStorePackageName = "com.android.vending"
		ctx, cancel := context.WithTimeout(ctx, optin.OptinTimeout)
		defer cancel()

		// Optin to Play Store.
		s.Log("Opting into Play Store")
		tconn, err := f.cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to get the test conn: ", err)
		}
		if err := optin.Perform(ctx, f.cr, tconn); err != nil {
			s.Fatal("Failed to optin to Play Store: ", err)
		}

		s.Log("Waiting for Playstore shown")
		if err := ash.WaitForVisible(ctx, tconn, playStorePackageName); err != nil {
			s.Fatal("Failed to wait for the playstore: ", err)
		}

		if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
			s.Fatal("Failed to close Play Store: ", err)
		}
	}()

	func() {
		ctx, cancel := context.WithTimeout(ctx, arc.BootTimeout)
		defer cancel()

		var err error
		if f.arc, err = arc.New(ctx, s.OutDir()); err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}

		if f.origRunningPkgs, err = f.runningPackages(ctx); err != nil {
			s.Fatal("Failed to list running packages: ", err)
		}
	}()

	return FixtureData{Chrome: f.cr, ARC: f.arc}
}

func (f *loggedInToCUJUserFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()

	if f.arc != nil {
		if err := f.arc.Close(); err != nil {
			testing.ContextLog(ctx, "Failed to close ARC connection: ", err)
		}
	}

	if f.cr != nil {
		if err := f.cr.Close(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to close Chrome connection: ", err)
		}
	}
}

func (f *loggedInToCUJUserFixture) Reset(ctx context.Context) error {
	// Stopping the running apps.
	running, err := f.runningPackages(ctx)
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

func (f *loggedInToCUJUserFixture) runningPackages(ctx context.Context) (map[string]struct{}, error) {
	tasks, err := f.arc.DumpsysActivityActivities(ctx)
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
