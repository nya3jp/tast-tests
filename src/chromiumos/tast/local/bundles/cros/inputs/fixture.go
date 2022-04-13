// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/inputactions"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
)

const (
	resetTimeout    = 30 * time.Second
	postTestTimeout = 5 * time.Second
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "clamshellVK",
		Desc: "Clamshell mode with VK enabled",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(clamshellMode, true, false),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "tabletVK",
		Desc: "Tablet mode with VK enabled",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(tabletMode, true, false),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "tabletVKInGuest",
		Desc: "Tablet mode in guest login with VK enabled",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(tabletMode, true, false, chrome.GuestLogin()),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "clamshellNonVK",
		Desc: "Clamshell mode with VK disabled",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(clamshellMode, false, false),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "tabletNonVK",
		Desc: "Tablet mode with VK disabled",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(tabletMode, false, false),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "tabletNonVKInGuest",
		Desc: "Tablet mode in guest login with VK disabled",
		Contacts: []string{
			"alvinjia@google.com",
			"shengjun@chromium.org",
			"essential-inputs-team@google.com",
		},
		Impl:            inputsFixture(tabletMode, true, false, chrome.GuestLogin()),
		SetUpTimeout:    chrome.LoginTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
}

// FixtureData is the data returned by SetUp and passed to tests.
type FixtureData struct {
	Chrome      *chrome.Chrome
	TestAPIConn *chrome.TestConn
	UserContext *useractions.UserContext
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
	cr        *chrome.Chrome  // underlying Chrome instance
	dm        deviceMode      // device ui mode to test
	vkEnabled bool            // whether virtual keyboard is force enabled
	reset     bool            // whether clean & restart Chrome before test
	fOpts     []chrome.Option // options that are passed to chrome.New
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

	uc, err := inputactions.NewInputsUserContextWithoutState(ctx, "E14s Fixture", s.OutDir(), f.cr, f.tconn, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create new inputs user context")
	}

	chrome.Lock()
	return FixtureData{f.cr, f.tconn, uc}
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
	if err := resetIMEStatus(ctx, f.tconn); err != nil {
		return errors.Wrap(err, "failed resetting ime")
	}
	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
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

func inputsFixture(dm deviceMode, vkEnabled, reset bool, opts ...chrome.Option) testing.FixtureImpl {
	return &inputsFixtureImpl{
		dm:        dm,
		vkEnabled: vkEnabled,
		reset:     reset,
		fOpts:     opts,
	}
}

// resetIMEStatus resets IME input method and settings.
func resetIMEStatus(ctx context.Context, tconn *chrome.TestConn) error {
	// Reset input to default input method.
	activeIME, err := ime.ActiveInputMethod(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get current ime")
	}
	if !activeIME.Equal(ime.DefaultInputMethod) {
		if err := ime.DefaultInputMethod.InstallAndActivate(tconn)(ctx); err != nil {
			return errors.Wrapf(err, "failed to set ime to %q", ime.DefaultInputMethod)
		}
	}

	return nil
}
