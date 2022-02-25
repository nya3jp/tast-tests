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
		Func:         SpellCheckLanguage,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that the SpellCheckLanguage policy is correctly applied",
		Contacts: []string{
			"eariassoto@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome", "lacros"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.LacrosPolicyLoggedIn,
	})
}

type mispelledWordTestCase struct {
	correct   string // correct is the word to test the spell check suggestion.
	mispelled string // mispelled is the intentional word with a typo.
}

func SpellCheckLanguage(ctx context.Context, s *testing.State) {
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

	name := "enable"
	pol := &policy.SpellcheckLanguage{Val: []string{"es", "fr", "de"}}
	s.Run(ctx, name, func(ctx context.Context, s *testing.State) {
		// Update policies.
		if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{pol}); err != nil {
			s.Error("Failed to update policies: ", err)
		}

		// Open lacros browser.
		br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), browser.TypeLacros)
		if err != nil {
			s.Error("Failed to open the browser: ", err)
		}
		defer closeBrowser(cleanupCtx)

		defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+name)

		languages := []string{
			"Spanish", // [es]
			"French",  // [fr]
			"German",  // [de]
		}
		for _, lang := range languages {
			// Verify that the language is included for spell check
			if err := policyutil.SettingsPage(ctx, cr, br, "languages").
				SelectNode(ctx, nodewith.
					Name(lang).
					Role(role.ToggleButton)).
				Restriction(restriction.Disabled).
				Checked(checked.True).
				Verify(); err != nil {
				s.Errorf("Unexpected settings state for language %v: %v", lang, err)
			}
		}

		// The default spell checker can highlight mispelled words in other languages. To be sure that
		// the language is being properly spell check, we are looking for the suggestion in the original language
		words := []mispelledWordTestCase{
			{"trompeta", "tronpeta"},     // [es]
			{"toujours", "tougours"},     // [fr]
			{"Sauerkraut", "Sauarkraut"}, // [de]
		}

		err = spellCheckWithSuggestionEnabled(ctx, tconn, br, words)
		if err != nil {
			s.Error("Failed to check spell check language status: ", err)
		}
	})
}

func spellCheckWithSuggestionEnabled(ctx context.Context, tconn *browser.TestConn, br *browser.Browser, words []mispelledWordTestCase) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Open a data URI of a page containing a textarea.
	conn, err := br.NewConn(ctx, "data:text/html, <html><body><textarea aria-label='textarea'/>")
	if err != nil {
		return errors.Wrap(err, "failed to open page with textarea")
	}
	defer conn.Close()

	ui := uiauto.New(tconn)

	textArea := nodewith.Name("textarea")
	if err := ui.WaitUntilExists(textArea)(ctx); err != nil {
		return errors.Wrap(err, "failed to find text area")
	}

	if err := ui.LeftClick(textArea)(ctx); err != nil {
		return errors.Wrap(err, "failed to focus text area")
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "could not create input keyboard")
	}
	defer kb.Close()

	for _, w := range words {
		// Check if Spell Check is enabled by checking that the spelling marker is added after text is typed.
		if err := tconn.Call(ctx, nil, `(name) => {
			// Set global variable.
			window["spellCheckEnabled_" + name] = false;

			let observer = chrome.automation.addTreeChangeObserver('textMarkerChanges', (treeChange) => {
				if (!treeChange.target.markers || treeChange.target.markers.length == 0) {
					return;
				}

				if (treeChange.target.markers[0].flags.spelling) {
					window["spellCheckEnabled_" + name] = true;
				}

				chrome.automation.removeTreeChangeObserver(observer);
			});
		}`, w.correct); err != nil {
			return errors.Wrap(err, "could not observe spelling marker changes")
		}

		// Type misspelled word into keyboard to generate a spelling marker.
		if err := kb.Type(ctx, w.mispelled+" "); err != nil {
			return errors.Wrap(err, "could not type on keyboard")
		}

		enabled := false
		falseErr := errors.New("spellCheckEnabled evaluated to false")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := tconn.Eval(ctx, "spellCheckEnabled_"+w.correct, &enabled); err != nil {
				return testing.PollBreak(err)
			}
			if !enabled {
				return falseErr
			}
			return nil
		}, &testing.PollOptions{Interval: 10 * time.Millisecond, Timeout: 1 * time.Second}); err != nil {
			if !errors.Is(err, falseErr) {
				return errors.Wrap(err, "could not evaluate spellCheckEnabled")
			}
		}

		// The triple click is needed because the first double click will select the space
		// that was written last. The additional left click will select the entire text area input
		suggestionItem := nodewith.Role(role.MenuItem).Name(w.correct)
		if err := uiauto.Combine("Check for spelling check suggestion",
			ui.DoubleClick(textArea),
			ui.LeftClick(textArea),
			ui.RightClick(textArea),
			ui.WaitUntilExists(suggestionItem),
			ui.LeftClick(suggestionItem),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to find suggestion")
		}

		if err := uiauto.Combine("Clear text area",
			kb.TypeSequenceAction([]string{"Ctrl", "a"}),
			kb.TypeKeyAction(input.KEY_DELETE),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to clear text area")
		}
	}
	return nil
}
