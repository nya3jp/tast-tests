// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/imesettings"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         InputMethodShelf,
		Desc:         "Verifies that user can toggle shelf option and switch inut method",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			Name:              "stable",
			ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			ExtraAttr:         []string{"group:input-tools-upstream"},
		}, {
			Name:              "unstable",
			ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
			ExtraAttr:         []string{"informational"},
		}},
	})
}

func InputMethodShelf(ctx context.Context, s *testing.State) {
	const (
		searchKeyword   = "japanese"                           // Keyword used to search input method.
		inputMethodName = "Japanese with US keyboard"          // Input method should be displayed after search.
		inputMethodCode = string(ime.INPUTMETHOD_NACL_MOZC_US) // Input method code of the input method.
	)

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
	screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
	if err != nil {
		s.Log("Failed to create ScreenRecorder: ", err)
	}

	defer uiauto.ScreenRecorderStopSaveRelease(ctx, screenRecorder, filepath.Join(s.OutDir(), "InputMethodShelf.webm"))

	if screenRecorder != nil {
		screenRecorder.Start(ctx, tconn)
	}


	// Add IME for testing.
	imeCode := ime.IMEPrefix + inputMethodCode

	settings, err := imesettings.LaunchAtInputsSettingsPage(ctx, tconn, cr)
	if err != nil {
		s.Fatal("Failed to launch OS settings and land at inputs setting page: ", err)
	}

	ui := uiauto.New(tconn)
	imeMenuTrayButtonFinder := nodewith.Name("IME menu button").Role(role.Button)
	jpOptionFinder := nodewith.Name("Japanese with US keyboard").Role(role.CheckBox)
	usOptionFinder := nodewith.Name("English (US)").Role(role.CheckBox)

	if err := uiauto.Combine("toggle show input options in shelf",
		// Show input options in shelf should be disabled by default.
		settings.ShowInputOptionsInShelfShouldBe(cr, tconn, false),
		// IME tray should be hidden by default.
		ui.WaitUntilGone(imeMenuTrayButtonFinder),

		// Add second IME.
		func(context.Context) error {
			s.Logf("Add second input method: %s", imeCode)
			return ime.AddInputMethod(ctx, tconn, imeCode)
		},
		// Show input options in shelf is enabled automatically.
		settings.ShowInputOptionsInShelfShouldBe(cr, tconn, true),
		// IME tray should be displayed after adding second IME.
		ui.WaitUntilExists(imeMenuTrayButtonFinder),

		// Select JP input method from IME tray.
		ui.LeftClickUntil(imeMenuTrayButtonFinder, ui.WithTimeout(3*time.Second).WaitUntilExists(jpOptionFinder)),
		ui.LeftClick(jpOptionFinder),
		func(ctx context.Context) error {
			return ime.WaitForInputMethodMatches(ctx, tconn, imeCode, 10*time.Second)
		},
		// Select US input method from IME tray.
		ui.LeftClick(imeMenuTrayButtonFinder),
		ui.LeftClick(usOptionFinder),
		func(ctx context.Context) error {
			return ime.WaitForInputMethodMatches(ctx, tconn, ime.IMEPrefix+string(ime.INPUTMETHOD_XKB_US_ENG), 10*time.Second)
		},

		// Toggle off the option. IME tray should be gone.
		settings.ToggleShowInputOptionsInShelf(),
		ui.WaitUntilGone(imeMenuTrayButtonFinder),
	)(ctx); err != nil {
		s.Fatal("Failed to verify input options in shelf: ", err)
	}
}
