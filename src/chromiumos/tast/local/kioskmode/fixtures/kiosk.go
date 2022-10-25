// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fixtures contains fixtures useful for Kiosk mode tests.
package fixtures

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/shirou/gopsutil/v3/process"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash/ashproc"
	"chromiumos/tast/local/kioskmode"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     fixture.KioskLoggedInAsh,
		Desc:     "Kiosk mode started with default app setup, DUT is enrolled",
		Contacts: []string{"kamilszarek@google.com", "alt-modalities-stability@google.com"},
		Impl: &kioskFixture{
			autoLaunchKioskAppID:    kioskmode.WebKioskAccountID,
			useDefaultLocalAccounts: true,
		},
		SetUpTimeout:    chrome.ManagedUserLoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PostTestTimeout: 15 * time.Second,
		Parent:          fixture.FakeDMSEnrolled,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     fixture.KioskLoggedInLacros,
		Desc:     "Kiosk mode started with default app setup, DUT is enrolled and Lacros enabled",
		Contacts: []string{"irfedorova@google.com", "chromeos-kiosk-eng@google.com"},
		Impl: &kioskFixture{
			autoLaunchKioskAppID:    kioskmode.WebKioskAccountID,
			useDefaultLocalAccounts: true,
			extraOpts:               []chrome.Option{chrome.ExtraArgs("--enable-features=LacrosSupport,WebKioskEnableLacros", "--lacros-availability-ignore")},
		},
		SetUpTimeout:    chrome.ManagedUserLoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PostTestTimeout: 15 * time.Second,
		Parent:          fixture.FakeDMSEnrolled,
	})
}

type kioskFixture struct {
	// cr is a connection to an already-started Chrome instance that loads policies from FakeDMS.
	cr *chrome.Chrome
	// fdms is the already running DMS server from the parent fixture.
	fdms *fakedms.FakeDMS
	// useDefaultLocalAccounts enables default local accounts generated in
	// kioskmode.New().
	useDefaultLocalAccounts bool
	// localAccounts is the policy with local accounts configuration that will
	// be applied for Kiosk mode.
	localAccounts *policy.DeviceLocalAccounts
	// autoLaunchKioskAppID is a preselected Kiosk app ID used for autolaunch.
	autoLaunchKioskAppID string
	// extraOpts contains extra options passed to Chrome.
	extraOpts []chrome.Option
	// proc is the root Chrome process. Kept to be used in Reset() checking if
	// Chrome process hasn't restarted.
	proc *process.Process
	// kiosk is a reference to the Kiosk intstance.
	kiosk *kioskmode.Kiosk
}

// KioskFixtData is returned by the fixture.
type KioskFixtData struct {
	// fakeDMS is an already running DMS server.
	fakeDMS *fakedms.FakeDMS
	// chrome is a connection to an already-started Chrome instance that loads policies from FakeDMS.
	chrome *chrome.Chrome
}

// Chrome implements the HasChrome interface.
func (f KioskFixtData) Chrome() *chrome.Chrome {
	if f.chrome == nil {
		panic("Chrome is called with nil chrome instance")
	}
	return f.chrome
}

// FakeDMS implements the HasFakeDMS interface.
func (f KioskFixtData) FakeDMS() *fakedms.FakeDMS {
	if f.fakeDMS == nil {
		panic("FakeDMS is called with nil fakeDMS instance")
	}
	return f.fakeDMS
}

// PolicyFileDump is the filename where the state of policies is dumped after the test ends.
const PolicyFileDump = "policies.json"

func (k *kioskFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	fdms, ok := s.ParentValue().(*fakedms.FakeDMS)
	if !ok {
		s.Fatal("Parent is not a fakeDMSEnrolled fixture")
	}

	k.fdms = fdms

	options := []kioskmode.Option{
		kioskmode.AutoLaunch(k.autoLaunchKioskAppID),
		kioskmode.ExtraChromeOptions(k.extraOpts...),
	}
	if k.useDefaultLocalAccounts {
		options = append(options, kioskmode.DefaultLocalAccounts())
	} else {
		options = append(options, kioskmode.CustomLocalAccounts(k.localAccounts))
	}

	kiosk, cr, err := kioskmode.New(ctx, fdms, options...)
	if err != nil {
		s.Fatal("Failed to create Chrome in kiosk mode: ", err)
	}

	proc, err := ashproc.Root()
	if err != nil {
		if err := kiosk.Close(ctx); err != nil {
			s.Error("There was an error while closing Kiosk: ", err)
		}
		s.Fatal("Failed to get root Chrome PID: ", err)
	}

	chrome.Lock()
	k.cr = cr
	k.proc = proc
	k.kiosk = kiosk
	return &KioskFixtData{k.fdms, k.cr}
}

func (k *kioskFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()

	if k.cr == nil {
		s.Fatal("Chrome not yet started")
	}

	if err := k.kiosk.Close(ctx); err != nil {
		s.Error("There was an error while closing Kiosk: ", err)
	}

	k.cr = nil
}

func (k *kioskFixture) Reset(ctx context.Context) error {
	// Check the connection to Chrome.
	if err := k.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}

	// Check if the root chrome process is still running.
	if r, err := k.proc.IsRunning(); err != nil || !r {
		return errors.New("found root chrome process termination while running in Kiosk mode")
	}
	return nil
}

func (k *kioskFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}
func (k *kioskFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	tconn, err := k.cr.TestAPIConn(ctx)
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
	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), PolicyFileDump), b, 0644); err != nil {
		s.Error("Failed to dump policies to file: ", err)
	}
}
