// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package clipboardhistory

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// TODO check this timeout
// resetTimeout is the timeout duration for trying to reset the current fixture.
const resetTimeout = 30 * time.Second

// NewClipboardHistoryFixture TODO.
func NewClipboardHistoryFixture(browserType browser.Type) testing.FixtureImpl {
	return &clipboardHistoryFixture{
		browserType: browserType,
	}
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "clipboardHistory",
		Desc:            "TODO",
		Contacts:        []string{"ckincaid@chromium.org", "multipaste-eng+test@google.com"},
		Impl:            NewClipboardHistoryFixture(browser.TypeAsh),
		SetUpTimeout:    resetTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
		Parent:          "chromeLoggedIn",
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "clipboardHistoryLacros",
		Desc:            "TODO",
		Contacts:        []string{"ckincaid@chromium.org", "multipaste-eng+test@google.com"},
		Impl:            NewClipboardHistoryFixture(browser.TypeLacros),
		SetUpTimeout:    resetTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
		Parent:          "lacros",
	})
}

type clipboardHistoryFixture struct {
	browserType  browser.Type
	closeBrowser func(ctx context.Context) error
	kb           *input.KeyboardEventWriter
}

// FixtData holds information made available to tests that specify this Fixture.
type FixtData struct {
	// Chrome is the running chrome instance.
	Chrome *chrome.Chrome

	// TestConn is a connection to the test extension.
	TestConn *chrome.TestConn

	// UI is a context for automating UI actions.
	UI *uiauto.Context

	// Keyboard is a writer for performing keystroke actions.
	Keyboard *input.KeyboardEventWriter

	// Browser is an instance of the ash or Lacros Chrome browser.
	Browser *browser.Browser
}

func (f *clipboardHistoryFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	// It is fine to use an existing Chrome instance as long as tests using this
	// fixture explicitly add or remove any clipboard history items they require
	// to be present or absent.
	cr := s.ParentValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// TODO check this timeout
	ui := uiauto.New(tconn).WithTimeout(10 * time.Second)

	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, f.browserType)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	f.closeBrowser = closeBrowser

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	f.kb = kb

	return &FixtData{
		Chrome:   cr,
		TestConn: tconn,
		UI:       ui,
		Keyboard: kb,
		Browser:  br,
	}
}

func (f *clipboardHistoryFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	f.kb.Close()
	f.closeBrowser(ctx)
}

func (f *clipboardHistoryFixture) Reset(ctx context.Context) error {
	return nil
}

func (f *clipboardHistoryFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *clipboardHistoryFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}
