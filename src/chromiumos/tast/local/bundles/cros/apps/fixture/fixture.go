// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fixture defines fixtures for Essential apps tests.
package fixture

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
)

const (
	resetTimeout    = 30 * time.Second
	preTestTimeout  = 10 * time.Second
	postTestTimeout = 15 * time.Second
)

// List of fixture names for Essential Apps.
const (
	LoggedIn         = "loggedIn"
	LoggedInJP       = "loggedInJP"
	LoggedInGuest    = "loggedInGuest"
	LacrosLoggedIn   = "lacrosLoggedIn"
	LacrosLoggedInJP = "lacrosLoggedInJP"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            LoggedIn,
		Desc:            "Logged into a user session for essential apps",
		Contacts:        []string{"shengjun@chromium.org"},
		Impl:            eaFixture(notForced, false, browser.TypeAsh),
		SetUpTimeout:    preTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            LoggedInJP,
		Desc:            "Logged into a user session for essential apps in Japanese language",
		Contacts:        []string{"shengjun@chromium.org"},
		Impl:            eaFixture(notForced, false, browser.TypeAsh, chrome.Region("jp")),
		SetUpTimeout:    preTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            LoggedInGuest,
		Desc:            "Logged into a guest user session for essential apps",
		Contacts:        []string{"shengjun@chromium.org"},
		Impl:            eaFixture(notForced, false, browser.TypeAsh, chrome.GuestLogin()),
		SetUpTimeout:    preTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// LacrosLoggedIn is a fixture to bring up Lacros as a primary browser
	// from the rootfs partition by default.
	// It pre-installs essential apps.
	testing.AddFixture(&testing.Fixture{
		Name:            LacrosLoggedIn,
		Desc:            "Logged into a user session with Lacros for essential apps",
		Contacts:        []string{"alvinjia@google.com", "shengjun@chromium.org"},
		Impl:            eaFixture(notForced, false, browser.TypeLacros),
		SetUpTimeout:    chrome.LoginTimeout + 1*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// LacrosLoggedInJP is a fixture to bring up Lacros as a primary browser
	// from the rootfs partition by default and it sets the device language
	// to Japanese.
	// It pre-installs essential apps.
	testing.AddFixture(&testing.Fixture{
		Name:            LacrosLoggedInJP,
		Desc:            "Logged into a user session with Lacros for essential apps in Japanese language",
		Contacts:        []string{"alvinjia@google.com", "shengjun@chromium.org"},
		Impl:            eaFixture(notForced, false, browser.TypeLacros, chrome.Region("jp")),
		SetUpTimeout:    chrome.LoginTimeout + 1*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
}

// FixtData is the data returned by SetUp and passed to tests.
type FixtData struct {
	Chrome      *chrome.Chrome
	TestAPIConn *chrome.TestConn
	UserContext *useractions.UserContext
	BrowserType browser.Type
}

// deviceMode describes the device UI mode it boots in.
type deviceMode int

const (
	notForced deviceMode = iota
	tabletMode
	clamshellMode
)

// eaFixtureImpl implements testing.FixtureImpl.
type eaFixtureImpl struct {
	cr          *chrome.Chrome  // Underlying Chrome instance
	dm          deviceMode      // Device ui mode to test
	restart     bool            // Whether restart the fixture after each test
	browserType browser.Type    // Whether Ash or Lacros is used for test
	fOpts       []chrome.Option // Options that are passed to chrome.New
	tconn       *chrome.TestConn
	recorder    *uiauto.ScreenRecorder
	uc          *useractions.UserContext
}

func (f *eaFixtureImpl) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	var opts []chrome.Option
	// If there's a parent fixture and the fixture supplies extra options, use them.
	if extraOpts, ok := s.ParentValue().([]chrome.Option); ok {
		opts = append(opts, extraOpts...)
	}
	opts = append(opts, f.fOpts...)
	opts = append(opts, chrome.EnableWebAppInstall())

	switch f.dm {
	case tabletMode:
		opts = append(opts, chrome.ExtraArgs("--force-tablet-mode=touch_view"))
	case clamshellMode:
		opts = append(opts, chrome.ExtraArgs("--force-tablet-mode=clamshell"))
	}

	if f.browserType == browser.TypeLacros {
		lacrosOpts, err := lacrosfixt.NewConfig(lacrosfixt.Mode(lacros.LacrosPrimary)).Opts()
		if err != nil {
			s.Fatal("Failed to get lacros options: ", err)
		}
		opts = append(opts, lacrosOpts...)
	}

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	f.cr = cr

	f.tconn, err = f.cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get test API connection")
	}

	chrome.Lock()
	return FixtData{f.cr, f.tconn, f.uc, f.browserType}
}

func (f *eaFixtureImpl) PreTest(ctx context.Context, s *testing.FixtTestState) {
	// filepath.Base(s.OutDir()) returns the test name.
	// TODO(b/235164130) use s.TestName once it is available.
	f.uc.SetTestName(filepath.Base(s.OutDir()))

	recorder, err := uiauto.NewScreenRecorder(ctx, f.tconn)
	if err != nil {
		s.Log("Failed to create screen recorder: ", err)
		return
	}
	if err := recorder.Start(ctx, f.tconn); err != nil {
		s.Log("Failed to start screen recorder: ", err)
		return
	}
	f.recorder = recorder
}

func (f *eaFixtureImpl) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// Do nothing if the recorder is not initialized.
	if f.recorder != nil {
		f.recorder.StopAndSaveOnError(ctx, filepath.Join(s.OutDir(), "record.webm"), s.HasError)
	}
}

func (f *eaFixtureImpl) Reset(ctx context.Context) error {
	if f.restart {
		return errors.New("Intended error to trigger fixture restart")
	}
	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}
	return nil
}

func (f *eaFixtureImpl) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()
	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	f.cr = nil
	f.tconn = nil
}

func eaFixture(dm deviceMode, restart bool, browserType browser.Type, opts ...chrome.Option) testing.FixtureImpl {
	return &eaFixtureImpl{
		dm:          dm,
		restart:     restart,
		browserType: browserType,
		fOpts:       opts,
	}
}
