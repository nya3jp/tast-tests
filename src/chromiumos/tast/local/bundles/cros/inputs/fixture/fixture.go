// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fixture defines fixtures for inputs tests.
package fixture

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/inputactions"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
)

const (
	resetTimeout    = 30 * time.Second
	postTestTimeout = 5 * time.Second
)

// List of fixture names for inputs.
const (
	AnyVK                              = "anyVK"
	AnyVKInGuest                       = "anyVKInGuest"
	ClamshellVK                        = "clamshellVK"
	ClamshellVKWithAssistAutocorrect   = "clamshellVKWithAssistAutocorrect"
	ClamshellNonVK                     = "clamshellNonVK"
	ClamshellNonVKInGuest              = "clamshellNonVKInGuest"
	ClamshellNonVKWithMultiwordSuggest = "clamshellNonVKWithMultiwordSuggest"
	ClamshellNonVKWithGrammarCheck     = "clamshellNonVKWithGrammarCheck"
	TabletVK                           = "tabletVK"
	TabletVKInGuest                    = "tabletVKInGuest"
	TabletVKWithAssistAutocorrect      = "tabletVKWithAssistAutocorrect"
	TabletVKWithMultipasteSuggestion   = "tabletVKWithMultipasteSuggestion"
	// Lacros fixtures.
	LacrosAnyVK                              = "lacrosAnyVK"
	LacrosAnyVKInGuest                       = "lacrosAnyVKInGuest"
	LacrosClamshellVK                        = "lacrosClamshellVK"
	LacrosClamshellVKWithAssistAutocorrect   = "lacrosClamshellVKWithAssistAutocorrect"
	LacrosClamshellNonVK                     = "lacrosClamshellNonVK"
	LacrosClamshellNonVKInGuest              = "lacrosClamshellNonVKInGuest"
	LacrosClamshellNonVKWithMultiwordSuggest = "lacrosClamshellNonVKWithMultiwordSuggest"
	LacrosClamshellNonVKWithGrammarCheck     = "lacrosClamshellNonVKWithGrammarCheck"
	LacrosTabletVK                           = "lacrosTabletVK"
	LacrosTabletVKInGuest                    = "lacrosTabletVKInGuest"
	LacrosTabletVKWithAssistAutocorrect      = "lacrosTabletVKWithAssistAutocorrect"
	LacrosTabletVKWithMultipasteSuggestion   = "lacrosTabletVKWithMultipasteSuggestion"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: AnyVK,
		Desc: "Any mode with VK enabled",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(notForced, true, false, false),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: AnyVKInGuest,
		Desc: "Any mode in guest login with VK enabled",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(notForced, true, false, false, chrome.GuestLogin()),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: ClamshellVK,
		Desc: "Clamshell mode with A11y VK enabled",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(clamshellMode, true, false, false),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: ClamshellVKWithAssistAutocorrect,
		Desc: "Clamshell mode with A11y VK enabled  and assist autocorrect",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(clamshellMode, true, false, false, chrome.ExtraArgs("--enable-features=AssistAutoCorrect")),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: ClamshellNonVK,
		Desc: "Clamshell mode with VK disabled",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(clamshellMode, false, false, false),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: ClamshellNonVKWithMultiwordSuggest,
		Desc: "Clamshell mode with VK disabled and multiword suggest",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(clamshellMode, false, false, false, chrome.ExtraArgs("--enable-features=AssistMultiWord")),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: ClamshellNonVKWithGrammarCheck,
		Desc: "Clamshell mode with VK disabled and grammar check",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(clamshellMode, false, false, false, chrome.ExtraArgs("--enable-features=OnDeviceGrammarCheck")),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: ClamshellNonVKInGuest,
		Desc: "Clamshell mode in guest login with VK disabled",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(clamshellMode, false, false, false, chrome.GuestLogin()),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: TabletVK,
		Desc: "Tablet mode with VK enabled",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(tabletMode, true, false, false),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: TabletVKWithAssistAutocorrect,
		Desc: "Tablet mode with VK enabled and assist autocorrect",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(tabletMode, true, false, false, chrome.ExtraArgs("--enable-features=AssistAutoCorrect")),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: TabletVKWithMultipasteSuggestion,
		Desc: "Tablet mode with VK enabled and multipaste suggestion",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(tabletMode, true, false, false, chrome.ExtraArgs("--enable-features=VirtualKeyboardMultipasteSuggestion")),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: TabletVKInGuest,
		Desc: "Tablet mode in guest login with VK enabled",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(tabletMode, true, false, false, chrome.GuestLogin()),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	//--------------Lacros Fixtures--------------------------------------------
	testing.AddFixture(&testing.Fixture{
		Name: LacrosAnyVK,
		Desc: "Lacros variant: any mode with VK enabled",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(notForced, true, false, true),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: LacrosAnyVKInGuest,
		Desc: "Lacros variant: any mode in guest login with VK enabled",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(notForced, true, false, true, chrome.GuestLogin()),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: LacrosClamshellVK,
		Desc: "Lacros variant: clamshell mode with A11y VK enabled",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(clamshellMode, true, false, true),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: LacrosClamshellVKWithAssistAutocorrect,
		Desc: "Lacros variant: clamshell mode with A11y VK enabled  and assist autocorrect",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(clamshellMode, true, false, true, chrome.ExtraArgs("--enable-features=AssistAutoCorrect")),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: LacrosClamshellNonVK,
		Desc: "Lacros variant: clamshell mode with VK disabled",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(clamshellMode, false, false, true),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: LacrosClamshellNonVKWithMultiwordSuggest,
		Desc: "Lacros variant: clamshell mode with VK disabled and multiword suggest",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(clamshellMode, false, false, true, chrome.ExtraArgs("--enable-features=AssistMultiWord")),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: LacrosClamshellNonVKWithGrammarCheck,
		Desc: "Lacros variant: clamshell mode with VK disabled and grammar check",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(clamshellMode, false, false, true, chrome.ExtraArgs("--enable-features=OnDeviceGrammarCheck")),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: LacrosClamshellNonVKInGuest,
		Desc: "Lacros variant: clamshell mode in guest login with VK disabled",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(clamshellMode, false, false, true, chrome.GuestLogin()),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: LacrosTabletVK,
		Desc: "Lacros variant: tablet mode with VK enabled",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(tabletMode, true, false, true),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: LacrosTabletVKWithAssistAutocorrect,
		Desc: "Lacros variant: tablet mode with VK enabled and assist autocorrect",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(tabletMode, true, false, true, chrome.ExtraArgs("--enable-features=AssistAutoCorrect")),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: LacrosTabletVKWithMultipasteSuggestion,
		Desc: "Lacros variant: tablet mode with VK enabled and multipaste suggestion",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(tabletMode, true, false, true, chrome.ExtraArgs("--enable-features=VirtualKeyboardMultipasteSuggestion")),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: LacrosTabletVKInGuest,
		Desc: "Lacros variant: tablet mode in guest login with VK enabled",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(tabletMode, true, false, true, chrome.GuestLogin()),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
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

// inputsFixtureImpl implements testing.FixtureImpl.
type inputsFixtureImpl struct {
	cr        *chrome.Chrome  // Underlying Chrome instance
	dm        deviceMode      // Device ui mode to test
	vkEnabled bool            // Whether virtual keyboard is force enabled
	reset     bool            // Whether clean & restart Chrome before test
	isLacros  bool            //Whether use Lacros
	fOpts     []chrome.Option // Options that are passed to chrome.New
	tconn     *chrome.TestConn
}

func (f *inputsFixtureImpl) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	var opts []chrome.Option
	// If there's a parent fixture and the fixture supplies extra options, use them.
	if extraOpts, ok := s.ParentValue().([]chrome.Option); ok {
		opts = append(opts, extraOpts...)
	}
	opts = append(opts, f.fOpts...)

	switch f.dm {
	case tabletMode:
		opts = append(opts, chrome.ExtraArgs("--force-tablet-mode=touch_view"))
	case clamshellMode:
		opts = append(opts, chrome.ExtraArgs("--force-tablet-mode=clamshell"))
	}

	browserType := browser.TypeAsh
	if f.isLacros {
		browserType = browser.TypeLacros
		lacrosOpts, err := lacrosfixt.NewConfig(lacrosfixt.ChromeOptions(opts...)).Opts()
		if err != nil {
			s.Fatal("Failed to get lacros options: ", err)
		}
		opts = append(opts, lacrosOpts...)
	}

	if f.vkEnabled && f.dm != clamshellMode {
		// Force enable tablet VK by default. Even the device is actually in clamshell mode but not explicitly mentioned.
		opts = append(opts, chrome.VKEnabled())
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

	if f.vkEnabled && f.dm == clamshellMode {
		// Enable a11y virtual keyboard.
		if err := vkb.NewContext(f.cr, f.tconn).EnableA11yVirtualKeyboard(true)(ctx); err != nil {
			return errors.Wrap(err, "failed to enable a11y virtual keyboard")
		}
	}

	//TODO(b/229059789): Assign TestName to user context after migration to fixture.
	uc, err := inputactions.NewInputsUserContextWithoutState(ctx, "", s.OutDir(), f.cr, f.tconn, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create new inputs user context")
	}

	chrome.Lock()
	return FixtData{f.cr, f.tconn, uc, browserType}
}

func (f *inputsFixtureImpl) PreTest(ctx context.Context, s *testing.FixtTestState) {

}

func (f *inputsFixtureImpl) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// Hide virtual keyboard in case it is still on screen.
	if f.vkEnabled {
		if err := vkb.NewContext(f.cr, f.tconn).HideVirtualKeyboard()(ctx); err != nil {
			s.Log("Failed to hide virtual keyboard: ", err)
		}
	}
}

func (f *inputsFixtureImpl) Reset(ctx context.Context) error {
	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	if err := resetIMEStatus(ctx, f.tconn); err != nil {
		return errors.Wrap(err, "failed resetting ime")
	}
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}
	return nil
}

func (f *inputsFixtureImpl) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()
	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	f.cr = nil
	f.tconn = nil
}

func inputsFixture(dm deviceMode, vkEnabled, reset, isLacros bool, opts ...chrome.Option) testing.FixtureImpl {
	return &inputsFixtureImpl{
		dm:        dm,
		vkEnabled: vkEnabled,
		reset:     reset,
		isLacros:  isLacros,
		fOpts:     opts,
	}
}

// resetIMEStatus resets IME input method and settings.
func resetIMEStatus(ctx context.Context, tconn *chrome.TestConn) error {
	if err := ime.DefaultInputMethod.Install(tconn)(ctx); err != nil {
		return errors.Wrapf(err, "failed to install default ime %q", ime.DefaultInputMethod)
	}
	// Uninstall all input methods except the default one.
	installedIMEs, err := ime.InstalledInputMethods(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get installed ime list")
	}
	prefix, err := ime.Prefix(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get ime prefix")
	}
	for _, installedIME := range installedIMEs {
		if strings.TrimPrefix(installedIME.ID, prefix) == ime.DefaultInputMethod.ID {
			continue
		}
		if err := ime.RemoveInputMethod(ctx, tconn, installedIME.ID); err != nil {
			return errors.Wrapf(err, "failed to remove %s", installedIME.ID)
		}
	}
	// Reset input to default input method.
	if err := ime.DefaultInputMethod.Activate(tconn)(ctx); err != nil {
		return errors.Wrapf(err, "failed to set ime to %q", ime.DefaultInputMethod)
	}
	if err := ime.DefaultInputMethod.ResetSettings(tconn)(ctx); err != nil {
		return errors.Wrapf(err, "failed to reset ime settings of the default ime %s", ime.DefaultInputMethod)
	}
	return nil
}
