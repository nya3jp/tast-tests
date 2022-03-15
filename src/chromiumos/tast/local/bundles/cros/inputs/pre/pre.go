// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pre contains preconditions for inputs tests.
package pre

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/inputactions"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
	"chromiumos/tast/timing"
)

// StableModels is a list of boards that stable enough and aim to run inputs tests in CQ.
var StableModels = []string{
	"betty",
	// Random boards on the top boards for VK list.
	"bobba",
	"bobba360",
	"casta",
	"coral",
	"kefka",
	// Convertible chromebook, top usage in 2018 and 2019.
	"cyan",
	// Top VK usage board in 2020 -- convertible, ARM.
	"hana",
	// Kukui family, not much usage, but very small tablet.
	"kodama",
	"krane",
	"kukui",
	// Another top board -- convertible, x64.
	"snappy",
}

// GrammarEnabledModels is a list boards where Grammar Check is enabled.
var GrammarEnabledModels = []string{
	"betty",
	"octopus",
	"nocturne",
	"hatch",
}

// MultiwordEnabledModels is a subset of boards where multiword suggestions are
// enabled. The multiword feature is enabled on all 4gb boards, with a list of
// 2gb boards having the feature explicitly disabled. See the following link
// for a list of all boards where the feature is disabled.
// https://source.chromium.org/search?q=f:make.defaults%20%22-ondevice_text_suggestions%22&ss=chromiumos&start=31
var MultiwordEnabledModels = []string{
	"betty",
	"octopus",
	"nocturne",
	"hatch",
}

// InputsStableModels is a shortlist of models aiming to run critical inputs tests.
// More information refers to http://b/161415599.
var InputsStableModels = hwdep.Model(StableModels...)

// InputsUnstableModels is a list of models to run inputs tests at 'informational' so that we know once they are stable enough to be promoted to CQ.
// kevin64 is an experimental board does not support nacl, which fails Canvas installation.
// To stabilize the tests, have to exclude entire kevin model as no distinguish between kevin and kevin64.
var InputsUnstableModels = hwdep.SkipOnModel(append(StableModels, "kevin")...)

// resetTimeout is the timeout duration to trying reset of the current precondition.
const resetTimeout = 30 * time.Second

func inputsPreCondition(name string, dm deviceMode, vkEnabled, reset bool, opts ...chrome.Option) *preImpl {
	return &preImpl{
		name:      name,
		timeout:   resetTimeout + chrome.LoginTimeout,
		dm:        dm,
		vkEnabled: vkEnabled,
		reset:     reset,
		opts:      opts,
	}
}

// VKEnabled creates a new precondition can be shared by tests that require an already-started Chromeobject that enables virtual keyboard.
// It uses --enable-virtual-keyboard to force enable virtual keyboard regardless of device ui mode.
var VKEnabled = inputsPreCondition("virtual_keyboard_enabled_pre", notForced, true, false)

// VKEnabledReset is the same setup as VKEnabled.
// It restarts Chrome session and logs in as new user for each test.
var VKEnabledReset = inputsPreCondition("virtual_keyboard_enabled_reset_pre", notForced, true, true)

// VKEnabledInGuest creates a new precondition the same as VKEnabled in Guest mode.
var VKEnabledInGuest = inputsPreCondition("virtual_keyboard_enabled_guest_pre", notForced, true, false, chrome.GuestLogin())

// VKEnabledTablet creates a new precondition for testing virtual keyboard in tablet mode.
// It boots device in tablet mode and force enabled virtual keyboard via chrome flag --enable-virtual-keyboard.
var VKEnabledTablet = inputsPreCondition("virtual_keyboard_enabled_tablet_pre", tabletMode, true, false)

// VKEnabledTabletReset is the same setup as VKEnabledTablet.
// It restarts Chrome session and logs in as new user for each test.
var VKEnabledTabletReset = inputsPreCondition("virtual_keyboard_enabled_tablet_reset_pre", tabletMode, true, true)

// VKEnabledTabletWithAssistAutocorrectReset is similar to VKEnabledTablet, but also with AssistAutoCorrect flag enabled.
// It restarts Chrome session and logs in as new user for each test.
var VKEnabledTabletWithAssistAutocorrectReset = inputsPreCondition("virtual_keyboard_enabled_tablet_assist_autocorrect_pre", tabletMode, true, true, chrome.ExtraArgs("--enable-features=AssistAutoCorrect"))

// VKEnabledTabletInGuest creates a new precondition the same as VKEnabledTablet in Guest mode.
var VKEnabledTabletInGuest = inputsPreCondition("virtual_keyboard_enabled_tablet_guest_pre", tabletMode, true, false, chrome.GuestLogin())

// VKEnabledTabletWithMultipasteSuggestion is similar to VKEnabledTablet, but also with multipaste-suggestion flag enabled.
// It restarts Chrome session and logs in as new user for each test.
var VKEnabledTabletWithMultipasteSuggestion = inputsPreCondition("virtual_keyboard_enabled_tablet_multipaste_suggestion_pre", tabletMode, true, true, chrome.ExtraArgs("--enable-features=VirtualKeyboardMultipasteSuggestion"))

// VKEnabledClamshell creates a new precondition for testing virtual keyboard in clamshell mode.
// It uses Chrome API settings.a11y.virtual_keyboard to enable a11y vk instead of --enable-virtual-keyboard.
var VKEnabledClamshell = inputsPreCondition("virtual_keyboard_enabled_clamshell_pre", clamshellMode, true, false)

// VKEnabledClamshellReset is the same setup as VKEnabledClamshell.
// It restarts Chrome session and logs in as new user for each test.
var VKEnabledClamshellReset = inputsPreCondition("virtual_keyboard_enabled_clamshell_reset_pre", clamshellMode, true, true)

// VKEnabledClamshellWithAssistAutocorrectReset is similar to VKEnabledClamshell, but also with AssistAutoCorrect flag enabled.
// It restarts Chrome session and logs in as new user for each test.
var VKEnabledClamshellWithAssistAutocorrectReset = inputsPreCondition("virtual_keyboard_enabled_clamshell_assist_autocorrect_pre", clamshellMode, true, true, chrome.ExtraArgs("--enable-features=AssistAutoCorrect"))

// VKEnabledClamshellInGuest creates a new precondition the same as VKEnabledClamshell in Guest mode.
var VKEnabledClamshellInGuest = inputsPreCondition("virtual_keyboard_enabled_clamshell_guest_pre", clamshellMode, true, false, chrome.GuestLogin())

// NonVKClamshell creates a precondition for testing physical keyboard.
// It forces device to be clamshell mode and vk disabled.
var NonVKClamshell = inputsPreCondition("non_vk_clamshell_pre", clamshellMode, false, false)

// NonVKClamshellInGuest creates a precondition for testing physical keyboard in guest mode.
// It forces device to be clamshell mode and vk disabled.
var NonVKClamshellInGuest = inputsPreCondition("non_vk_clamshell_guest_pre", clamshellMode, false, false, chrome.GuestLogin())

// NonVKClamshellReset creates a precondition for testing physical keyboard.
// It forces device to be clamshell mode and vk disabled.
// It restarts Chrome session and logs in as new user for each test.
var NonVKClamshellReset = inputsPreCondition("non_vk_clamshell_reset_pre", clamshellMode, false, true)

// NonVKClamshellWithGrammarCheck creates a precondition for testing physical keyboard, and with OnDeviceGrammarCheck flag enabled.
// It forces device to be clamshell mode and vk disabled.
var NonVKClamshellWithGrammarCheck = inputsPreCondition("non_vk_clamshell_with_grammar_check_pre", clamshellMode, false, false, chrome.ExtraArgs("--enable-features=OnDeviceGrammarCheck"))

// NonVKClamshellWithMultiwordSuggest creates a precondition for testing physical keyboard, and with AssistMultiWord flag enabled.
// It forces device to be clamshell mode and vk disabled.
var NonVKClamshellWithMultiwordSuggest = inputsPreCondition("non_vk_clamshell_with_multiword_suggest_pre", clamshellMode, false, false, chrome.ExtraArgs("--enable-features=AssistMultiWord"))

// The PreData object is made available to users of this precondition via:
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		d := s.PreValue().(pre.PreData)
//		...
//	}
type PreData struct { // NOLINT
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

// preImpl implements testing.PreCondition.
type preImpl struct {
	name      string          // testing.PreCondition.String
	timeout   time.Duration   // testing.PreCondition.Timeout
	cr        *chrome.Chrome  // underlying Chrome instance
	dm        deviceMode      // device ui mode to test
	reset     bool            // Whether clean & restart Chrome before test
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
		if !p.reset {
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
				uc, err := inputactions.NewInputsUserContext(ctx, s, p.cr, p.tconn, nil)
				if err != nil {
					return errors.Wrap(err, "failed to create new inputs user context")
				}
				return PreData{p.cr, p.tconn, uc}
			}
			s.Log("Failed to reuse existing Chrome session: ", err)
		}
		s.Log("Reset Chrome session...It will take a few seconds")
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

	uc, err := inputactions.NewInputsUserContext(ctx, s, p.cr, p.tconn, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create new inputs user context")
	}

	chrome.Lock()

	return PreData{p.cr, p.tconn, uc}
}

// ResetIMEStatus resets IME input method and settings.
func ResetIMEStatus(ctx context.Context, tconn *chrome.TestConn) error {
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
