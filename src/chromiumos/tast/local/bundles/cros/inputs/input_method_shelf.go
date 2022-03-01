// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/inputactions"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/imesettings"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         InputMethodShelf,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Verifies that user can toggle shelf option and switch inut method",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			ExtraAttr:         []string{"group:input-tools-upstream"},
		}, {
			Name:              "informational",
			ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
			ExtraAttr:         []string{"informational"},
		}},
	})
}

func InputMethodShelf(ctx context.Context, s *testing.State) {
	inputMethod := ime.JapaneseWithUSKeyboard

	cr, err := chrome.New(ctx, chrome.EnableFeatures("LanguageSettingsUpdate2"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	uc, err := inputactions.NewInputsUserContext(ctx, s, cr, tconn, nil)
	if err != nil {
		s.Fatal("Failed to initiate inputs user context: ", err)
	}

	settings, err := imesettings.LaunchAtInputsSettingsPage(ctx, tconn, cr)
	if err != nil {
		s.Fatal("Failed to launch OS settings and land at inputs setting page: ", err)
	}

	ui := uiauto.New(tconn)
	imeMenuTrayButtonFinder := nodewith.Name("IME menu button").Role(role.Button)
	jpOptionFinder := nodewith.Name("Japanese with US keyboard").Role(role.CheckBox)
	usOptionFinder := nodewith.Name("English (US)").Role(role.CheckBox)

	validateShelfDisabledByDefaultAction := uiauto.Combine("validate that setting is disabled by default",
		// Show input options in shelf should be disabled by default.
		settings.WaitUntilToggleOption(cr, string(imesettings.ShowInputOptionsInShelf), false),
		// IME tray should be hidden by default.
		ui.WaitUntilGone(imeMenuTrayButtonFinder),
	)

	if err := uiauto.UserAction(
		"validate that setting is disabled by default",
		validateShelfDisabledByDefaultAction,
		uc,
		&useractions.UserActionCfg{
			Tags: []useractions.ActionTag{useractions.ActionTagOSSettings, useractions.ActionTagIMEShelf},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to validate that input options in shelf is disabled by default: ", err)
	}

	activateAction := uiauto.Combine("input options in shelf is enabled automatically by adding second IME",
		// Add second IME.
		inputMethod.Install(tconn),
		// Show input options in shelf is enabled automatically.
		settings.WaitUntilToggleOption(cr, string(imesettings.ShowInputOptionsInShelf), true),
		// IME tray should be displayed after adding second IME.
		ui.WaitUntilExists(imeMenuTrayButtonFinder),
	)

	if err := uiauto.UserAction(
		"input options in shelf is enabled by adding second IME",
		activateAction,
		uc,
		&useractions.UserActionCfg{
			Tags: []useractions.ActionTag{useractions.ActionTagOSSettings, useractions.ActionTagIMEShelf},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to validate that input options in shelf is enabled by adding second IME: ", err)
	}

	changeIMEAction := uiauto.Combine("change input method via IME tray",
		// Select JP input method from IME tray.
		ui.LeftClickUntil(imeMenuTrayButtonFinder, ui.WithTimeout(3*time.Second).WaitUntilExists(jpOptionFinder)),
		ui.LeftClick(jpOptionFinder),
		func(ctx context.Context) error {
			fullyQualifiedIMEID, err := inputMethod.FullyQualifiedIMEID(ctx, tconn)
			if err != nil {
				return errors.Wrapf(err, "failed to get fully qualified IME ID of %q", inputMethod)
			}
			return ime.WaitForInputMethodMatches(ctx, tconn, fullyQualifiedIMEID, 10*time.Second)
		},
		// Select US input method from IME tray.
		ui.LeftClick(imeMenuTrayButtonFinder),
		ui.LeftClick(usOptionFinder),
		func(ctx context.Context) error {
			return ime.WaitForInputMethodMatches(ctx, tconn, ime.ChromeIMEPrefix+ime.EnglishUS.ID, 10*time.Second)
		},
	)

	if err := uiauto.UserAction(
		"user can change input method via IME tray",
		changeIMEAction,
		uc,
		&useractions.UserActionCfg{
			Tags: []useractions.ActionTag{useractions.ActionTagIMEShelf},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to change input method via IME tray: ", err)
	}

	toggleOffAction := uiauto.Combine("toggle off the option in OS setting",
		// Toggle off the option. IME tray should be gone.
		settings.ToggleShowInputOptionsInShelf(cr, false),
		ui.WaitUntilGone(imeMenuTrayButtonFinder),
	)

	if err := uiauto.UserAction(
		"user can disable IME tray in OS settings",
		toggleOffAction,
		uc,
		&useractions.UserActionCfg{
			Tags: []useractions.ActionTag{useractions.ActionTagOSSettings, useractions.ActionTagIMEShelf},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to disable IME tray in OS settings: ", err)
	}
}
