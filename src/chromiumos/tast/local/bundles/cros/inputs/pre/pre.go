// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pre contains preconditions for inputs tests.
package pre

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
	"chromiumos/tast/timing"
)

// StableModels is a list of boards that stable enough and aim to run inputs tests in CQ.
var StableModels = []string{
	// Top VK usage board in 2020 -- convertible, ARM.
	"hana",
	// Another top board -- convertible, x64.
	"snappy",
	// Kukui family, not much usage, but very small tablet.
	"kodama",
	"krane",
	// Convertible chromebook, top usage in 2018 and 2019.
	"cyan",
	// Random boards on the top boards for VK list.
	"bobba360",
	"bobba",
	"kefka",
	"coral",
	"betty",
}

// InputsStableModels is a shortlist of models aiming to run critical inputs tests.
// More information refers to http://b/161415599.
var InputsStableModels = hwdep.Model(StableModels...)

// InputsUnstableModels is a list of models to run inputs tests at 'informational' so that we know once they are stable enough to be promoted to CQ.
// kevin64 is an experimental board does not support nacl, which fails Canvas installation.
// To stabilize the tests, have to exclude entire kevin model as no distinguish between kevin and kevin64.
var InputsUnstableModels = hwdep.SkipOnModel(append(StableModels, "kevin1")...)

// resetTimeout is the timeout duration to trying reset of the current precondition.
const resetTimeout = 30 * time.Second

// defaultIMECode is used for new Chrome instance.
const defaultIMECode = ime.IMEPrefix + string(ime.INPUTMETHOD_XKB_US_ENG)

func inputsPreCondition(name string, dm deviceMode, vkEnabled bool, opts ...chrome.Option) *preImpl {
	return &preImpl{
		name:      name,
		timeout:   resetTimeout + chrome.LoginTimeout,
		vkEnabled: vkEnabled,
		dm:        dm,
		opts:      append(opts, chrome.EnableFeatures("ImeMojoDecoder")),
	}
}

// VKEnabled creates a new precondition can be shared by tests that require an already-started Chromeobject that enables virtual keyboard.
// It uses --enable-virtual-keyboard to force enable virtual keyboard regardless of device ui mode.
var VKEnabled = inputsPreCondition("virtual_keyboard_enabled_pre", notForced, true)

// VKEnabledInGuest creates a new precondition the same as VKEnabled in Guest mode.
var VKEnabledInGuest = inputsPreCondition("virtual_keyboard_enabled_guest_pre", notForced, true, chrome.GuestLogin())

// VKEnabledTablet creates a new precondition for testing virtual keyboard in tablet mode.
// It boots device in tablet mode and force enabled virtual keyboard via chrome flag --enable-virtual-keyboard.
var VKEnabledTablet = inputsPreCondition("virtual_keyboard_enabled_tablet_pre", tabletMode, true)

// VKEnabledTabletWithAssistAutocorrect is similar to VKEnabledTablet, but also with AssistAutoCorrect flag enabled.
var VKEnabledTabletWithAssistAutocorrect = inputsPreCondition("virtual_keyboard_enabled_tablet_assist_autocorrect_pre", tabletMode, true, chrome.ExtraArgs("--enable-features=AssistAutoCorrect"))

// VKEnabledTabletInGuest creates a new precondition the same as VKEnabledTablet in Guest mode.
var VKEnabledTabletInGuest = inputsPreCondition("virtual_keyboard_enabled_tablet_guest_pre", tabletMode, true, chrome.GuestLogin())

// VKEnabledClamshell creates a new precondition for testing virtual keyboard in clamshell mode.
// It uses Chrome API settings.a11y.virtual_keyboard to enable a11y vk instead of --enable-virtual-keyboard.
var VKEnabledClamshell = inputsPreCondition("virtual_keyboard_enabled_clamshell_pre", clamshellMode, true)

// VKEnabledClamshellWithAssistAutocorrect is similar to VKEnabledClamshell, but also with AssistAutoCorrect flag enabled.
var VKEnabledClamshellWithAssistAutocorrect = inputsPreCondition("virtual_keyboard_enabled_clamshell_assist_autocorrect_pre", clamshellMode, true, chrome.ExtraArgs("--enable-features=AssistAutoCorrect"))

// VKEnabledClamshellInGuest creates a new precondition the same as VKEnabledClamshell in Guest mode.
var VKEnabledClamshellInGuest = inputsPreCondition("virtual_keyboard_enabled_clamshell_guest_pre", clamshellMode, true, chrome.GuestLogin())

// NonVKClamshell creates a precondition for testing physical keyboard.
// It forces device to be clamshell mode and vk disabled.
var NonVKClamshell = inputsPreCondition("non_vk_clamshell_pre", clamshellMode, false)

// The PreData object is made available to users of this precondition via:
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		d := s.PreValue().(pre.PreData)
//		...
//	}
type PreData struct { // NOLINT
	Chrome      *chrome.Chrome
	TestAPIConn *chrome.TestConn
}

// deviceMode describes the device UI mode it boots in.
type deviceMode int

const (
	notForced deviceMode = iota
	tabletMode
	clamshellMode
)

// preImpl implements testing.PreCondition.
type preImpl struct {
	name      string          // testing.PreCondition.String
	timeout   time.Duration   // testing.PreCondition.Timeout
	cr        *chrome.Chrome  // underlying Chrome instance
	dm        deviceMode      // device ui mode to test
	vkEnabled bool            // Whether virtual keyboard is force enabled
	opts      []chrome.Option // Options that should be passed to chrome.New
	tconn     *chrome.TestConn
}

func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return p.timeout }

// Prepare is called by the test framework at the beginning of every test using this precondition.
// It returns a *chrome.Chrome that can be used by tests.
func (p *preImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

	if p.cr != nil {
		err := func() error {
			// Dump error if failed to reuse Chrome instance.
			defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, p.tconn)

			ctx, cancel := context.WithTimeout(ctx, resetTimeout)
			defer cancel()
			ctx, st := timing.Start(ctx, "reset_"+p.name)
			defer st.End()
			if err := p.cr.Responded(ctx); err != nil {
				return errors.Wrap(err, "existing Chrome connection is unusable")
			}

			// Hide virtual keyboard in case it is still on screen.
			if p.vkEnabled {
				if err := vkb.NewContext(p.cr, p.tconn).HideVirtualKeyboard()(ctx); err != nil {
					return errors.Wrap(err, "failed to hide virtual keyboard")
				}
			}

			if err := ResetIMEStatus(ctx, p.tconn); err != nil {
				return errors.Wrap(err, "failed resetting ime")
			}

			if err := p.cr.ResetState(ctx); err != nil {
				return errors.Wrap(err, "failed resetting existing Chrome session")
			}

			return nil
		}()
		if err == nil {
			s.Log("Reusing existing Chrome session")
			return PreData{p.cr, p.tconn}
		}
		s.Log("Failed to reuse existing Chrome session: ", err)
		chrome.Unlock()
		p.closeInternal(ctx, s)
	}

	ctx, cancel := context.WithTimeout(ctx, chrome.LoginTimeout)
	defer cancel()

	var err error

	opts := p.opts

	switch p.dm {
	case tabletMode:
		opts = append(opts, chrome.ExtraArgs("--force-tablet-mode=touch_view"))
	case clamshellMode:
		opts = append(opts, chrome.ExtraArgs("--force-tablet-mode=clamshell"))
	}

	if p.vkEnabled && p.dm != clamshellMode {
		// Force enable tablet VK by default. Even the device is actually in clamshell mode but not explicitly mentioned.
		opts = append(opts, chrome.VKEnabled())
	}

	if p.cr, err = chrome.New(ctx, opts...); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	p.tconn, err = p.cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get test API connection")
	}

	if p.vkEnabled && p.dm == clamshellMode {
		// Enable a11y virtual keyboard.
		if err := vkb.NewContext(p.cr, p.tconn).EnableA11yVirtualKeyboard(true)(ctx); err != nil {
			return errors.Wrap(err, "failed to enable a11y virtual keyboard")
		}
	}

	chrome.Lock()

	return PreData{p.cr, p.tconn}
}

// ResetIMEStatus resets IME input method and settings.
func ResetIMEStatus(ctx context.Context, tconn *chrome.TestConn) error {
	// Reset input to default input method.
	currentIME, err := ime.GetCurrentInputMethod(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get current ime")
	}
	if currentIME != defaultIMECode {
		if err := ime.SetCurrentInputMethod(ctx, tconn, defaultIMECode); err != nil {
			return errors.Wrapf(err, "failed to set ime to %s", defaultIMECode)
		}
	}

	return nil
}

// Close is called by the test framework after the last test that uses this precondition.
func (p *preImpl) Close(ctx context.Context, s *testing.PreState) {
	ctx, st := timing.Start(ctx, "close_"+p.name)
	defer st.End()

	chrome.Unlock()
	p.closeInternal(ctx, s)
}

// closeInternal closes and resets p.cr if non-nil.
func (p *preImpl) closeInternal(ctx context.Context, s *testing.PreState) {
	if p.cr == nil {
		return
	}
	if err := p.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	p.cr = nil
	p.tconn = nil
}
