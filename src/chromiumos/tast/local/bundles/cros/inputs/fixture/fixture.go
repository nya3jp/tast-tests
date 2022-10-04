// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fixture defines fixtures for inputs tests.
package fixture

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/inputactions"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
)

const (
	resetTimeout    = 30 * time.Second
	preTestTimeout  = 10 * time.Second
	postTestTimeout = 15 * time.Second
)

// List of fixture names for inputs.
const (
	AnyVK                                     = "anyVK"
	AnyVKInGuest                              = "anyVKInGuest"
	ClamshellVK                               = "clamshellVK"
	ClamshellVKRestart                        = "clamshellVKRestart"
	ClamshellNonVKWithDiacriticsOnPKLongpress = "clamshellWithDiacriticsOnPKLongpress"
	ClamshellNonVK                            = "clamshellNonVK"
	ClamshellNonVKInGuest                     = "clamshellNonVKInGuest"
	ClamshellNonVKRestart                     = "clamshellNonVKRestart"
	ClamshellNonVKWithMultiwordSuggest        = "clamshellNonVKWithMultiwordSuggest"
	TabletVK                                  = "tabletVK"
	TabletVKRestart                           = "tabletVKRestart"
	TabletVKInGuest                           = "tabletVKInGuest"
	TabletVKWithMultitouch                    = "tabletVKWithMultitouch"
	// Lacros fixtures.
	LacrosAnyVK                                     = "lacrosAnyVK"
	LacrosAnyVKInGuest                              = "lacrosAnyVKInGuest"
	LacrosClamshellVK                               = "lacrosClamshellVK"
	LacrosClamshellNonVK                            = "lacrosClamshellNonVK"
	LacrosClamshellNonVKInGuest                     = "lacrosClamshellNonVKInGuest"
	LacrosClamshellNonVKRestart                     = "lacrosClamshellNonVKRestart"
	LacrosClamshellNonVKWithMultiwordSuggest        = "lacrosClamshellNonVKWithMultiwordSuggest"
	LacrosClamshellNonVKWithDiacriticsOnPKLongpress = "lacrosClamshellWithDiacriticsOnPKLongpress"
	LacrosTabletVK                                  = "lacrosTabletVK"
	LacrosTabletVKInGuest                           = "lacrosTabletVKInGuest"
	LacrosTabletVKRestart                           = "lacrosTabletVKRestart"
	LacrosTabletVKWithMultitouch                    = "lacrosTabletVKWithMultitouch"
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
		Impl:            inputsFixture(notForced, true, false, browser.TypeAsh),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
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
		Impl:            inputsFixture(notForced, true, false, browser.TypeAsh, chrome.GuestLogin()),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
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
		Impl:            inputsFixture(clamshellMode, true, false, browser.TypeAsh),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: ClamshellVKRestart,
		Desc: "Clamshell mode with A11y VK enabled, restarting chrome session for every test",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(clamshellMode, true, true, browser.TypeAsh),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
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
		Impl:            inputsFixture(clamshellMode, false, false, browser.TypeAsh),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: ClamshellNonVKRestart,
		Desc: "Clamshell mode with VK disabled, restarting chrome session for every test",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(clamshellMode, false, true, browser.TypeAsh),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
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
		Impl:            inputsFixture(clamshellMode, false, false, browser.TypeAsh, chrome.ExtraArgs("--enable-features=AssistMultiWord")),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: ClamshellNonVKWithDiacriticsOnPKLongpress,
		Desc: "Clamshell mode with diacritics",
		Contacts: []string{
			"jhtin@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(clamshellMode, false, false, browser.TypeAsh, chrome.ExtraArgs("--enable-features=DiacriticsOnPhysicalKeyboardLongpress")),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
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
		Impl:            inputsFixture(clamshellMode, false, false, browser.TypeAsh, chrome.GuestLogin()),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
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
		Impl:            inputsFixture(tabletMode, true, false, browser.TypeAsh),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: TabletVKRestart,
		Desc: "Tablet mode with VK enabled, restarting chrome session for every test",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(tabletMode, true, true, browser.TypeAsh),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
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
		Impl:            inputsFixture(tabletMode, true, false, browser.TypeAsh, chrome.GuestLogin()),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: TabletVKWithMultitouch,
		Desc: "Tablet mode with VK and multitouch enabled",
		Contacts: []string{
			"michellegc@google.com",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(tabletMode, true, false, browser.TypeAsh, chrome.ExtraArgs("--enable-features=VirtualKeyboardMultitouch")),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
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
		Impl:            inputsFixture(notForced, true, false, browser.TypeLacros),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
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
		Impl:            inputsFixture(notForced, true, false, browser.TypeLacros, chrome.GuestLogin()),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
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
		Impl:            inputsFixture(clamshellMode, true, false, browser.TypeLacros),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
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
		Impl:            inputsFixture(clamshellMode, false, false, browser.TypeLacros),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: LacrosClamshellNonVKRestart,
		Desc: "Lacros variant: clamshell mode with VK disabled, restarting chrome session for every test",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(clamshellMode, false, true, browser.TypeLacros),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
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
		Impl:            inputsFixture(clamshellMode, false, false, browser.TypeLacros, chrome.ExtraArgs("--enable-features=AssistMultiWord")),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: LacrosClamshellNonVKWithDiacriticsOnPKLongpress,
		Desc: "Lacros variant: clamshell mode with VK disabled and diacritics on PK longpress",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(clamshellMode, false, false, browser.TypeLacros, chrome.ExtraArgs("--enable-features=DiacriticsOnPhysicalKeyboardLongpress")),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
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
		Impl:            inputsFixture(clamshellMode, false, false, browser.TypeLacros, chrome.GuestLogin()),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
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
		Impl:            inputsFixture(tabletMode, true, false, browser.TypeLacros),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
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
		Impl:            inputsFixture(tabletMode, true, false, browser.TypeLacros, chrome.GuestLogin()),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: LacrosTabletVKRestart,
		Desc: "Lacros variant: tablet mode with VK enabled restarting chrome session for every test",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(tabletMode, true, true, browser.TypeLacros),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: LacrosTabletVKWithMultitouch,
		Desc: "Lacros variant: tablet mode with VK and multitouch enabled",
		Contacts: []string{
			"michellegc@google.com",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(tabletMode, true, false, browser.TypeLacros, chrome.ExtraArgs("--enable-features=VirtualKeyboardMultitouch")),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
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
	cr          *chrome.Chrome  // Underlying Chrome instance
	dm          deviceMode      // Device ui mode to test
	vkEnabled   bool            // Whether virtual keyboard is force enabled
	restart     bool            // Whether restart the fixture after each test
	browserType browser.Type    // Whether Ash or Lacros is used for test
	fOpts       []chrome.Option // Options that are passed to chrome.New
	tconn       *chrome.TestConn
	recorder    *uiauto.ScreenRecorder
	uc          *useractions.UserContext
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

	if f.vkEnabled && f.dm != clamshellMode {
		// Force enable tablet VK by default. Even the device is actually in clamshell mode but not explicitly mentioned.
		opts = append(opts, chrome.VKEnabled())
	}

	cr, err := browserfixt.NewChrome(ctx, f.browserType, lacrosfixt.NewConfig(), opts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	f.cr = cr

	f.tconn, err = f.cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test API connection: ", err)
	}

	if f.vkEnabled && f.dm == clamshellMode {
		// Enable a11y virtual keyboard.
		if err := vkb.NewContext(f.cr, f.tconn).EnableA11yVirtualKeyboard(true)(ctx); err != nil {
			s.Fatal("Failed to enable a11y virtual keyboard: ", err)
		}
	}

	uc, err := inputactions.NewInputsUserContextWithoutState(ctx, "", s.OutDir(), f.cr, f.tconn, nil)
	if err != nil {
		s.Fatal("Failed to create new inputs user context: ", err)
	}
	f.uc = uc

	chrome.Lock()
	return FixtData{f.cr, f.tconn, f.uc, f.browserType}
}

func (f *inputsFixtureImpl) PreTest(ctx context.Context, s *testing.FixtTestState) {
	f.uc.SetTestName(s.TestName())

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

func (f *inputsFixtureImpl) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// Hide virtual keyboard in case it is still on screen.
	if f.vkEnabled {
		if err := vkb.NewContext(f.cr, f.tconn).HideVirtualKeyboard()(ctx); err != nil {
			s.Log("Failed to hide virtual keyboard: ", err)
		}
	}

	// Do nothing if the recorder is not initialized.
	if f.recorder != nil {
		f.recorder.StopAndSaveOnError(ctx, filepath.Join(s.OutDir(), "record.webm"), s.HasError)
	}
}

func (f *inputsFixtureImpl) Reset(ctx context.Context) error {
	if f.restart {
		return errors.New("Intended error to trigger fixture restart")
	}
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

func inputsFixture(dm deviceMode, vkEnabled, restart bool, browserType browser.Type, opts ...chrome.Option) testing.FixtureImpl {
	return &inputsFixtureImpl{
		dm:          dm,
		vkEnabled:   vkEnabled,
		restart:     restart,
		browserType: browserType,
		fOpts:       opts,
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
