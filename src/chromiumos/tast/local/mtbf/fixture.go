// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mtbf implements a library used for MTBF testing.
package mtbf

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
)

const (
	// AccountPool is runtime variable name for credentials to log into a Chrome user session.
	AccountPool = "mtbf.accountPool"

	// chromeLoggedInReuseFixture is a fixture name that will be registered to tast.
	chromeLoggedInReuseFixture = "mtbfChromeLogInReuse"

	// chromeLoggedInReuseLacrosFixture is a fixture name that will be registered to tast.
	chromeLoggedInReuseLacrosFixture = "mtbfChromeLogInReuseLacros"

	// LoginReuseFixture is a fixture name that will be registered to tast.
	LoginReuseFixture = "mtbfLoginReuseCleanTabs"

	// LoginReuseLacrosFixture is a fixture name that will be registered to tast.
	LoginReuseLacrosFixture = "mtbfLoginReuseCleanTabsLacros"
)

// LoginReuseOptions returns the login option for MTBF tests.
func LoginReuseOptions(accountPool string) []chrome.Option {
	return []chrome.Option{
		chrome.KeepState(),
		chrome.ARCSupported(),
		chrome.GAIALoginPool(accountPool),
		chrome.ExtraArgs(arc.DisableSyncFlags()...),
		chrome.TryReuseSession(),
	}
}

// LoginReuseLacrosOptions returns the login option for MTBF lacros tests.
func LoginReuseLacrosOptions(accountPool string) ([]chrome.Option, error) {
	lacrosOpts, err := lacrosfixt.NewConfig(
		// Consecutive calls the lacros.Launch/lacros.Connect -> (*lacros.Lacros).Close -> lacros.Launch is flaky,
		// also, MTBF tests reuse last session and depend on browser tabs being cleared,
		// which requires calling lacros.Launch/lacros.Connect -> (*lacros.Lacros).Close in PreTest, making the following test unable to launch Lacros.
		// We enable KeepAlive to ensure Lacros keeps running in the background to make it be able to launch anytime.
		lacrosfixt.KeepAlive(true),
		lacrosfixt.ChromeOptions(chrome.ExtraArgs("--lacros-availability-ignore")),
	).Opts()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create configs for lacros")
	}
	return append(LoginReuseOptions(accountPool), lacrosOpts...), nil
}

func loginReuseOptionsCallBack(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
	return LoginReuseOptions(s.RequiredVar(AccountPool)), nil
}

func loginReuseLacrosOptionsCallBack(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
	return LoginReuseLacrosOptions(s.RequiredVar(AccountPool))
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            chromeLoggedInReuseFixture,
		Desc:            "Reuse the existing user login session and boot ARC",
		Contacts:        []string{"xliu@cienet.com", "alfredyu@cienet.com", "abergman@google.com"},
		Impl:            arc.NewMtbfArcBootedFixture(loginReuseOptionsCallBack),
		SetUpTimeout:    chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		PreTestTimeout:  arc.PreTestTimeout,
		PostTestTimeout: arc.PostTestTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{AccountPool},
	})

	testing.AddFixture(&testing.Fixture{
		Name:            chromeLoggedInReuseLacrosFixture,
		Desc:            "Reuse the existing user login session",
		Contacts:        []string{"xliu@cienet.com", "alfredyu@cienet.com", "abergman@google.com"},
		Impl:            arc.NewMtbfArcBootedFixture(loginReuseLacrosOptionsCallBack),
		SetUpTimeout:    chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		PreTestTimeout:  arc.PreTestTimeout,
		PostTestTimeout: arc.PostTestTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{AccountPool},
	})

	testing.AddFixture(&testing.Fixture{
		Name:           LoginReuseFixture,
		Desc:           "Reuse the existing user login session and clean chrome tabs",
		Contacts:       []string{"xliu@cienet.com", "alfredyu@cienet.com", "abergman@google.com"},
		Parent:         chromeLoggedInReuseFixture,
		Impl:           &mtbfCleanTabsFixture{browserType: browser.TypeAsh},
		PreTestTimeout: 4 * clearTabsTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:           LoginReuseLacrosFixture,
		Desc:           "Reuse the existing user login session and clean chrome tabs with lacros variation",
		Contacts:       []string{"xliu@cienet.com", "alfredyu@cienet.com", "abergman@google.com"},
		Parent:         chromeLoggedInReuseLacrosFixture,
		Impl:           &mtbfCleanTabsFixture{browserType: browser.TypeLacros},
		PreTestTimeout: 4 * clearTabsTimeout,
	})
}

const clearTabsTimeout = 10 * time.Second

// FixtValue holds information made available to tests that specify this Fixture.
type FixtValue struct {
	cr *chrome.Chrome
	// ARC enables interaction with an already-started ARC environment.
	// It cannot be closed by tests.
	ARC *arc.ARC
}

// Chrome gets the CrOS-chrome instance.
// Implements the chrome.HasChrome interface.
func (f FixtValue) Chrome() *chrome.Chrome { return f.cr }

type mtbfCleanTabsFixture struct {
	fixtValue   *FixtValue
	browserType browser.Type
}

func (f *mtbfCleanTabsFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	parentValue := s.ParentValue()

	f.fixtValue = &FixtValue{
		cr:  parentValue.(*arc.PreData).Chrome,
		ARC: parentValue.(*arc.PreData).ARC,
	}

	return f.fixtValue
}

func (f *mtbfCleanTabsFixture) TearDown(ctx context.Context, s *testing.FixtState) {}

func (f *mtbfCleanTabsFixture) Reset(ctx context.Context) error { return nil }

func (f *mtbfCleanTabsFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	br := f.fixtValue.cr.Browser()
	if f.browserType == browser.TypeLacros {
		tconn, err := f.fixtValue.cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to get test API connection: ", err)
		}

		var closeBrowser func(context.Context) error
		if br, closeBrowser, err = PrepareLacros(ctx, f.fixtValue.cr, tconn); err != nil {
			s.Fatal("Failed to prepare lacros: ", err)
		}
		defer closeBrowser(ctx)

		// Maximize the lacros window to avoid the node location issue.
		// TODO(b/236799853): remove this once the lacros node location issue fixed.
		s.Log("Maximize the lacros window")
		lacrosWindow, err := ash.FindOnlyWindow(ctx, tconn, func(w *ash.Window) bool {
			return w.IsVisible && w.IsActive && strings.HasPrefix(w.Name, "ExoShellSurface")
		})
		if err != nil {
			s.Fatal("Failed to find lacros window: ", err)
		}
		if err := ash.SetWindowStateAndWait(ctx, tconn, lacrosWindow.ID, ash.WindowStateMaximized); err != nil {
			s.Fatal("Failed to maximize the lacros window : ", err)
		}
	}

	if err := closeExistingAndLeftOffTabs(ctx, br); err != nil {
		s.Fatal("Failed to close existing and left-off tab(s): ", err)
	}
}

func (f *mtbfCleanTabsFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

// PrepareLacros prepares the Lacros browser for MTBF tests.
// It connects to existing Lacros if the Lacros is running, set up a new Lacros browser otherwise.
func PrepareLacros(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) (*browser.Browser, func(context.Context) error, error) {
	lacrosApp, err := apps.Lacros(ctx, tconn)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get lacros app")
	}

	lacrosRunning, err := ash.AppRunning(ctx, tconn, lacrosApp.ID)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to check if Lacros is not running before launch")
	}

	if lacrosRunning {
		l, err := lacros.Connect(ctx, tconn)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to connect to lacros")
		}
		return l.Browser(), l.Close, nil
	}

	return browserfixt.SetUp(ctx, cr, browser.TypeLacros)
}

// closeExistingAndLeftOffTabs closes the existing and left-off tabs.
func closeExistingAndLeftOffTabs(ctx context.Context, br *browser.Browser) error {
	btconn, err := br.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get test API connection")
	}

	for {
		tabsCnt, err := countExistingTabs(ctx, btconn)
		if err != nil {
			return err
		}
		if tabsCnt > 0 {
			// The last session did not clanup properly, remove existing tabs can ensure tabs are cleaned.
			testing.ContextLogf(ctx, "Removing %d existing page(s)", tabsCnt)
			return removeExistingTabs(ctx, btconn)
		}

		// Depending on the settings, Chrome might open all left-off pages automatically from last session,
		// which the left-off pages might casues test case fail.
		// Launch Chrome browser by open a blank page to bring up all left-off pages to further remove them.
		testing.ContextLog(ctx, "Opening empty Chrome tab to bring up left-off page(s)")
		conn, err := br.NewConn(ctx, "", browser.WithNewWindow())
		if err != nil {
			return errors.Wrap(err, "failed to launch Chrome browser")
		}
		defer conn.Close()

		// After the first iteration, an empty Chrome tab will be opened,
		// i.e. at least one target will be found, which guarantees this isn't an infinity loop.
	}
}

func countExistingTabs(ctx context.Context, tconn *chrome.TestConn) (int, error) {
	execCtx, cancel := context.WithTimeout(ctx, clearTabsTimeout)
	defer cancel()

	tabs, err := browser.AllTabs(execCtx, tconn)
	if err != nil {
		return 0, err
	}

	for _, tab := range tabs {
		testing.ContextLogf(ctx, "Found an existing tab: %+v", tab)
	}

	return len(tabs), nil
}

func removeExistingTabs(ctx context.Context, tconn *chrome.TestConn) error {
	execCtx, cancel := context.WithTimeout(ctx, clearTabsTimeout)
	defer cancel()

	// If there exist unsave changes on web page, e.g. media content is playing or online document is editing,
	// "leave site" prompt will prevent the tab from closing and block the process,
	// therefore, a short context is required.
	if err := browser.CloseAllTabs(execCtx, tconn); err != nil {
		return errors.Wrap(err, "failed to remove tabs")
	}

	return nil
}
