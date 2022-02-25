// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SpellcheckLanguage,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that the SpellcheckLanguage policy is correctly applied for each language including text markers and word suggestions",
		Contacts: []string{
			"eariassoto@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome", "lacros"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.LacrosPolicyLoggedIn,
	})
}

type spellcheckLanguageWordTestCase struct {
	languageName string // languageName is the language to test the word with the intentional typo.
	misspelled   string // misspelled is a word with an intentional typo to be typed as an input.
	correct      string // correct is the expected suggestion to be shown by the spell checker.
}

func SpellcheckLanguage(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Error("Failed to create Test API connection: ", err)
	}

	// Update policy. We are not testing the case where the policy is unset because
	// it has no effect in the default spell check user settings. The default behavior
	// is verified by the SpellCheckEnabled test.
	pol := &policy.SpellcheckLanguage{Val: []string{"es", "fr", "de"}}
	if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{pol}); err != nil {
		s.Error("Failed to update policy: ", err)
	}

	// Open lacros browser.
	br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), browser.TypeLacros)
	if err != nil {
		s.Error("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	// The default language in the DUT is English US. Therefore, the test chooses different languages
	// to test this policy. To avoid false positive, this test will not only check for the text marker
	// highlight, but will also verify that the spell check will give the user a suggested word in the
	// language that is under test.
	languages := []spellcheckLanguageWordTestCase{
		{"Spanish", "tronpeta", "trompeta"},    // [es]
		{"French", "tougours", "toujours"},     // [fr]
		{"German", "Sauarkraut", "Sauerkraut"}, // [de]
	}

	for _, l := range languages {
		s.Run(ctx, "lang_"+l.languageName, verifySpellcheckForLanguage(cr, tconn, br, l))
	}
}

func verifySpellcheckForLanguage(cr *chrome.Chrome, tconn *browser.TestConn, br *browser.Browser, language spellcheckLanguageWordTestCase) func(ctx context.Context, s *testing.State) {
	return func(ctx context.Context, s *testing.State) {
		// Verify that the language is included for spell check
		if err := policyutil.SettingsPage(ctx, cr, br, "languages").
			SelectNode(ctx, nodewith.
				Name(language.languageName).
				Role(role.ToggleButton)).
			Restriction(restriction.Disabled).
			Checked(checked.True).
			Verify(); err != nil {
			s.Errorf("Unexpected settings state for language %v: %v", language.languageName, err)
		}

		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()

		// Open a data URI of a page containing a textarea.
		conn, err := br.NewConn(ctx, "data:text/html, <textarea aria-label='textarea'/>")
		if err != nil {
			s.Error("Failed to open page with text area: ", err)
		}
		defer conn.Close()

		ui := uiauto.New(tconn)

		textArea := nodewith.Name("textarea")
		if err := ui.WaitUntilExists(textArea)(ctx); err != nil {
			s.Error("Failed to find text area: ", err)
		}

		if err := ui.LeftClick(textArea)(ctx); err != nil {
			s.Error("Failed to focus text area: ", err)
		}

		kb, err := input.Keyboard(ctx)
		if err != nil {
			s.Error("Could not create input keyboard: ", err)
		}
		defer kb.Close()

		// Check if Spell Check is enabled by checking that the spelling marker is added after text is typed.
		jsGlobalVar := "spellCheckEnabled_" + language.correct
		if err := tconn.Call(ctx, nil, `(name) => {
			// Set global variable.
			window[name] = false;

			let observer = chrome.automation.addTreeChangeObserver('textMarkerChanges', (treeChange) => {
				if (!treeChange.target.markers || treeChange.target.markers.length == 0) {
					return;
				}

				if (treeChange.target.markers[0].flags.spelling) {
					window[name] = true;
				}

				chrome.automation.removeTreeChangeObserver(observer);
			});
		}`, jsGlobalVar); err != nil {
			s.Error("Could not observe spelling marker changes: ", err)
		}

		// Type misspelled word into keyboard to generate a spelling marker.
		if err := kb.Type(ctx, language.misspelled+" "); err != nil {
			s.Error("Could not type on keyboard: ", err)
		}

		enabled := false
		// We need this error to let Poll execute until the condition is true, as we
		// expect, or ultimately fail after the polling timeout.
		noSpellcheckErr := errors.New("spellCheckEnabled evaluated to false")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := tconn.Eval(ctx, jsGlobalVar, &enabled); err != nil {
				return testing.PollBreak(err)
			}
			if !enabled {
				return noSpellcheckErr
			}
			return nil
		}, &testing.PollOptions{Interval: 10 * time.Millisecond, Timeout: 3 * time.Second}); err != nil {
			if errors.Is(err, noSpellcheckErr) {
				s.Errorf("Spell check text marker is not enabled for language %v: %v", language.languageName, err)
			} else {
				s.Error("Could not evaluate spellCheckEnabled: ", err)
			}
		}

		// The triple click is needed because the first double click will select the space
		// that was written last. The additional left click will select the entire text area input.
		suggestionItem := nodewith.Role(role.MenuItem).Name(language.correct)
		if err := uiauto.Combine("Check for spelling check suggestion",
			ui.DoubleClick(textArea),
			ui.LeftClick(textArea),
			ui.RightClick(textArea),
			ui.WaitUntilExists(suggestionItem),
			ui.LeftClick(suggestionItem),
		)(ctx); err != nil {
			s.Errorf("Failed to find expected suggestion %v: %v", language.correct, err)
		}

		if err := uiauto.Combine("Clear text area",
			kb.TypeSequenceAction([]string{"Ctrl", "a"}),
			kb.TypeKeyAction(input.KEY_DELETE),
		)(ctx); err != nil {
			s.Error("Failed to clear text area: ", err)
		}
	}
}
