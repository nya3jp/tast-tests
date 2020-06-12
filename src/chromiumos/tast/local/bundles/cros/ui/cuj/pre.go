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
	"chromiumos/tast/timing"
)

const resetTimeout = 30 * time.Second

// PreData is the struct returned by the preconditions.
type PreData struct {
	Chrome *chrome.Chrome
	ARC    *arc.ARC
}

type preImpl struct {
	name            string
	timeout         time.Duration
	cr              *chrome.Chrome
	arc             *arc.ARC
	optinCompleted  bool
	origRunningPkgs map[string]struct{}
}

func (p *preImpl) String() string {
	return p.name
}

func (p *preImpl) Timeout() time.Duration {
	return p.timeout
}

func (p *preImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

	if p.cr != nil && p.arc != nil {
		ctx, cancel := context.WithTimeout(ctx, resetTimeout)
		defer cancel()
		ctx, st := timing.Start(ctx, "reset_"+p.name)
		defer st.End()
		if err := p.resetState(ctx); err != nil {
			p.closeInternal(ctx)
			s.Fatal("Failed to reset: ", err)
		}
		return PreData{Chrome: p.cr, ARC: p.arc}
	}

	func() {
		if p.cr != nil {
			return
		}
		ctx, cancel := context.WithTimeout(ctx, chrome.LoginTimeout)
		defer cancel()
		var err error
		username := s.RequiredVar("ui.cuj_username")
		password := s.RequiredVar("ui.cuj_password")
		p.cr, err = chrome.New(ctx, chrome.GAIALogin(), chrome.Auth(username, password, "gaia-id"), chrome.ARCSupported(),
			chrome.ExtraArgs("--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off"))
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
		chrome.Lock()
	}()

	func() {
		if p.optinCompleted {
			return
		}
		const playStorePackageName = "com.android.vending"
		ctx, cancel := context.WithTimeout(ctx, optin.OptinTimeout)
		defer cancel()

		// Optin to Play Store.
		s.Log("Opting into Play Store")
		tconn, err := p.cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to get the test conn: ", err)
		}
		if err := optin.Perform(ctx, p.cr, tconn); err != nil {
			s.Fatal("Failed to optin to Play Store: ", err)
		}
		p.optinCompleted = true

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
		if p.arc, err = arc.New(ctx, s.OutDir()); err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}

		if p.origRunningPkgs, err = p.runningPackages(ctx); err != nil {
			s.Fatal("Failed to list running packages: ", err)
		}
	}()

	return PreData{Chrome: p.cr, ARC: p.arc}
}

func (p *preImpl) runningPackages(ctx context.Context) (map[string]struct{}, error) {
	tasks, err := p.arc.DumpsysActivityActivities(ctx)
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

func (p *preImpl) resetState(ctx context.Context) error {
	// Stopping the running apps.
	running, err := p.runningPackages(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get running packages")
	}
	for pkg := range running {
		if _, ok := p.origRunningPkgs[pkg]; ok {
			continue
		}
		testing.ContextLogf(ctx, "Stopping package %q", pkg)
		if err := p.arc.Command(ctx, "am", "force-stop", pkg).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to stop %q", pkg)
		}
	}

	// Unlike ARC.preImpl, this does not uninstall apps. This is because we
	// typically want to reuse the same list of applications, and additional
	// installed apps wouldn't affect the test scenarios.
	return p.cr.ResetState(ctx)
}

func (p *preImpl) Close(ctx context.Context, s *testing.PreState) {
	ctx, st := timing.Start(ctx, "close_"+p.name)
	defer st.End()

	p.closeInternal(ctx)
}

func (p *preImpl) closeInternal(ctx context.Context) {
	chrome.Unlock()

	if p.arc != nil {
		if err := p.arc.Close(); err != nil {
			testing.ContextLog(ctx, "Failed to close ARC connection: ", err)
		}
		p.arc = nil
	}

	if p.cr != nil {
		if err := p.cr.Close(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to close Chrome connection: ", err)
		}
		p.cr = nil
	}
}

var loggedInToCUJUser = &preImpl{name: "logged_in_to_cuj_user", timeout: chrome.LoginTimeout + optin.OptinTimeout + arc.BootTimeout + 10*time.Second}

// LoggedInToCUJUser returns a precondition of chrome logged in to the "CUJ"
// test user and activates ARC.
func LoggedInToCUJUser() testing.Precondition {
	return loggedInToCUJUser
}
