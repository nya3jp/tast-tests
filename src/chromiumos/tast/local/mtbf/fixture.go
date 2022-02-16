// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mtbf implements a library used for MTBF testing.
package mtbf

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/testing"
)

const (
	// AccountPool is runtime variable name for credentials to log into a Chrome user session.
	AccountPool = "mtbf.accountPool"

	// chromeLoggedInReuseFixture is a fixture name that will be registered to tast.
	chromeLoggedInReuseFixture = "mtbfChromeLogInReuse"

	// LoginReuseFixture is a fixture name that will be registered to tast.
	LoginReuseFixture = "mtbfLoginReuseCleanTabs"
)

// LoginReuseOptions returns the login option for MTBF tests.
func LoginReuseOptions(accountPool string) []chrome.Option {
	return []chrome.Option{
		// The old filesapp (apps.Files) has memory leak issue, use the new one (apps.FilesSWA) is essential in MTBF tests.
		// FilesSWA is the default Files App if login into DUT manually, however the default config (config.NewConfig) of `EnableFilesAppSWA` is false,
		// therefore, need to use `EnableFilesAppSWA()`.
		chrome.EnableFilesAppSWA(),
		// Only the legacy launcher is being exercised in MTBF tests currently.
		chrome.DisableFeatures("ProductivityLauncher"),
		chrome.KeepState(),
		chrome.ARCSupported(),
		chrome.GAIALoginPool(accountPool),
		chrome.ExtraArgs(arc.DisableSyncFlags()...),
		chrome.TryReuseSession(),
	}
}

func optionsCallBack(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
	return LoginReuseOptions(s.RequiredVar(AccountPool)), nil
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            chromeLoggedInReuseFixture,
		Desc:            "Reuse the existing user login session",
		Contacts:        []string{"xliu@cienet.com"},
		Impl:            arc.NewMtbfArcBootedFixture(optionsCallBack),
		SetUpTimeout:    chrome.GAIALoginTimeout + arc.BootTimeout + optin.OptinTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		PostTestTimeout: arc.PostTestTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{AccountPool},
	})

	testing.AddFixture(&testing.Fixture{
		Name:           LoginReuseFixture,
		Desc:           "Reuse the existing user login session and clean chrome tabs",
		Contacts:       []string{"xliu@cienet.com"},
		Parent:         chromeLoggedInReuseFixture,
		Impl:           &mtbfCleanTabsFixture{},
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
	fixtValue *FixtValue
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
	if err := closeExistingAndLeftOffTabs(ctx, f.fixtValue.cr); err != nil {
		s.Fatal("Failed to close existing and left-off tab(s): ", err)
	}
}

func (f *mtbfCleanTabsFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

// closeExistingAndLeftOffTabs closes the existing and left-off tabs.
func closeExistingAndLeftOffTabs(ctx context.Context, cr *chrome.Chrome) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get test API connection")
	}

	for {
		tabsCnt, err := countExistingTabs(ctx, tconn)
		if err != nil {
			return err
		}
		if tabsCnt > 0 {
			// The last session did not clanup properly, remove existing tabs can ensure tabs are cleaned.
			testing.ContextLogf(ctx, "Removing %d existing page(s)", tabsCnt)
			return removeExistingTabs(ctx, tconn)
		}

		// Depending on the settings, Chrome might open all left-off pages automatically from last session,
		// which the left-off pages might casues test case fail.
		// Launch Chrome browser by open a blank page to bring up all left-off pages to further remove them.
		testing.ContextLog(ctx, "Opening empty Chrome tab to bring up left-off page(s)")
		conn, err := cr.NewConn(ctx, "", browser.WithNewWindow())
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

	queryTabs := `(async () => {
		const tabs = await tast.promisify(chrome.tabs.query)({});
		return tabs.filter((tab) => tab.id);
	})()`

	var tabs []struct {
		Title string `json:"title"`
		URL   string `json:"url"`
	}
	if err := tconn.Eval(execCtx, queryTabs, &tabs); err != nil {
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

	expr := `(async () => {
		const tabs = await tast.promisify(chrome.tabs.query)({});
		await tast.promisify(chrome.tabs.remove)(tabs.filter((tab) => tab.id).map((tab) => tab.id));
	})()`

	// If there exist unsave changes on web page, e.g. media content is playing or online document is editing,
	// "leave site" prompt will prevent the tab from closing and block the process,
	// therefore, a short context is required.
	if err := tconn.Eval(execCtx, expr, nil); err != nil {
		return errors.Wrap(err, "failed to remove tabs")
	}

	return nil
}
