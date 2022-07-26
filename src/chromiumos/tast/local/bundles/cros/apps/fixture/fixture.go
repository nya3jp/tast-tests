// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fixture defines fixtures for Essential apps tests.
package fixture

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/testing"
)

const (
	resetTimeout    = 30 * time.Second
	preTestTimeout  = 10 * time.Second
	postTestTimeout = 15 * time.Second
)

// List of fixture names for Essential Apps.
const (
	LoggedIn                               = "loggedIn"
	LoggedInJP                             = "loggedInJP"
	LoggedInGuest                          = "loggedInGuest"
	ArcBootedWithGalleryPhotosImageFeature = "arcBootedWithGalleryPhotosImageFeature"
	LacrosLoggedIn                         = "lacrosLoggedIn"
	LacrosLoggedInJP                       = "lacrosLoggedInJP"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            LoggedIn,
		Desc:            "Logged into a user session for essential apps",
		Contacts:        []string{"shengjun@chromium.org"},
		Impl:            eaFixture(browser.TypeAsh),
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            LoggedInJP,
		Desc:            "Logged into a user session for essential apps in Japanese language",
		Contacts:        []string{"shengjun@chromium.org"},
		Impl:            eaFixture(browser.TypeAsh, chrome.Region("jp")),
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            LoggedInGuest,
		Desc:            "Logged into a guest user session for essential apps",
		Contacts:        []string{"shengjun@chromium.org"},
		Impl:            eaFixture(browser.TypeAsh, chrome.GuestLogin()),
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     ArcBootedWithGalleryPhotosImageFeature,
		Desc:     "ARC is booted with the MediaAppPhotosIntegrationImage feature flag enabled",
		Contacts: []string{"bugsnash@chromium.org", "shengjun@google.com"},
		Vars:     []string{"ui.gaiaPoolDefault"},
		Impl: arc.NewArcBootedWithPlayStoreFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.EnableFeatures("MediaAppPhotosIntegrationImage:minPhotosVersionForImage/1.0"),
				chrome.ExtraArgs(arc.DisableSyncFlags()...),
				chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault"))}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + arc.BootTimeout + ui.StartTimeout,
		ResetTimeout:    arc.ResetTimeout,
		PostTestTimeout: arc.PostTestTimeout,
		TearDownTimeout: arc.ResetTimeout,
	})

	// LacrosLoggedIn is a fixture to bring up Lacros as a primary browser
	// from the rootfs partition by default.
	// It pre-installs essential apps.
	testing.AddFixture(&testing.Fixture{
		Name:            LacrosLoggedIn,
		Desc:            "Logged into a user session with Lacros for essential apps",
		Contacts:        []string{"alvinjia@google.com", "shengjun@chromium.org"},
		Impl:            eaFixture(browser.TypeLacros),
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
		SetUpTimeout:    chrome.LoginTimeout + time.Minute,
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
		Impl:            eaFixture(browser.TypeLacros, chrome.Region("jp")),
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
		SetUpTimeout:    chrome.LoginTimeout + time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
}

// FixtData is the data returned by SetUp and passed to tests.
type FixtData struct {
	Chrome      *chrome.Chrome
	TestAPIConn *chrome.TestConn
	BrowserType browser.Type
}

// fixtureImpl implements testing.FixtureImpl.
type fixtureImpl struct {
	cr          *chrome.Chrome  // Underlying Chrome instance
	browserType browser.Type    // Whether Ash or Lacros is used for test
	fOpts       []chrome.Option // Options that are passed to chrome.New
	tconn       *chrome.TestConn
	recorder    *uiauto.ScreenRecorder
}

func (f *fixtureImpl) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	var opts []chrome.Option
	// If there's a parent fixture and the fixture supplies extra options, use them.
	if extraOpts, ok := s.ParentValue().([]chrome.Option); ok {
		opts = append(opts, extraOpts...)
	}
	opts = append(opts, f.fOpts...)
	opts = append(opts, chrome.EnableWebAppInstall())

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
	return FixtData{f.cr, f.tconn, f.browserType}
}

func (f *fixtureImpl) PreTest(ctx context.Context, s *testing.FixtTestState) {
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

func (f *fixtureImpl) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// Do nothing if the recorder is not initialized.
	if f.recorder != nil {
		f.recorder.StopAndSaveOnError(ctx, filepath.Join(s.OutDir(), "record.webm"), s.HasError)
	}
}

func (f *fixtureImpl) Reset(ctx context.Context) error {
	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}
	return nil
}

func (f *fixtureImpl) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()
	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	f.cr = nil
	f.tconn = nil
}

func eaFixture(browserType browser.Type, opts ...chrome.Option) testing.FixtureImpl {
	return &fixtureImpl{
		browserType: browserType,
		fOpts:       opts,
	}
}
