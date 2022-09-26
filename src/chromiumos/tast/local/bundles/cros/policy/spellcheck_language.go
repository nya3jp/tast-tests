// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
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
		},
		SoftwareDeps: []string{"chrome", "lacros"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.LacrosPolicyLoggedIn,
		Timeout:      3 * time.Minute,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.SpellcheckLanguage{}, pci.VerifiedFunctionalityUI),
		},
	})
}

type spellcheckLanguageWordTestCase struct {
	languageName string // language to test the misspelled word.
	misspelled   string // word with an intentional typo typed as an input.
	correct      string // expected suggestion to be shown by the spell checker.
}

func SpellcheckLanguage(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Error("Failed to create Test API connection: ", err)
	}

	// Update policy. We are not testing the case where the policy is unset
	// because it has no effect in the default spell check user settings.
	// The default behavior is verified by the SpellCheckEnabled test.
	pol := &policy.SpellcheckLanguage{Val: []string{"es", "fr", "de"}}
	if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{pol}); err != nil {
		s.Error("Failed to update policy: ", err)
	}

	// The default language in the DUT is English US. Therefore, the test
	// chooses different languages to test this policy. To avoid false
	// positives, the test will check for the text marker highlight and for
	// the correct suggestion in the same language.
	languages := []spellcheckLanguageWordTestCase{
		{"Spanish", "tronpeta", "trompeta"},    // [es]
		{"French", "tougours", "toujours"},     // [fr]
		{"German", "Sauarkraut", "Sauerkraut"}, // [de]
	}

	for _, l := range languages {
		s.Run(ctx, "lang_"+l.languageName, func(ctx context.Context, s *testing.State) {
			if err := verifySpellcheckForLanguage(ctx, s, cr, tconn, l); err != nil {
				s.Error("Failed to verify spellcheck: ", err)
			}
		})
	}
}

func verifySpellcheckForLanguage(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *browser.TestConn, language spellcheckLanguageWordTestCase) error {

	ui := uiauto.New(tconn)

	// Verify that the language is included for spell check
	appConn, err := apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/osLanguages/input")
	if err != nil {
		return errors.Wrap(err, "failed to open the OS settings page")
	}
	defer appConn.Close()

	if err := ui.WaitUntilExists(nodewith.
		Name("Remove " + language.languageName))(ctx); err != nil {
		return errors.Wrapf(err, "failed to find language %s in settings", language.languageName)
	}

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Open lacros browser.
	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, browser.TypeLacros)
	if err != nil {
		return errors.Wrap(err, "failed to open the browser")
	}
	defer closeBrowser(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+language.languageName)

	// Open a data URI of a page containing a textarea.
	conn, err := br.NewConn(ctx, "data:text/html, <textarea aria-label='textarea'/>")
	if err != nil {
		return errors.Wrap(err, "failed to open page with text area")
	}
	defer conn.Close()

	textArea := nodewith.Name("textarea")
	if err := ui.LeftClick(textArea)(ctx); err != nil {
		return errors.Wrap(err, "failed to focus text area")
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "could not create input keyboard")
	}
	defer kb.Close()

	// Check for the spelling marker after the misspelled word is typed.
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
		return errors.Wrap(err, "could not observe spelling marker changes")
	}

	// Type misspelled word into keyboard to generate a spelling marker.
	if err := kb.Type(ctx, language.misspelled); err != nil {
		return errors.Wrap(err, "could not type on keyboard")
	}

	enabled := false
	// The spelling marker has a delay, while the spellchecker process the word.
	// Poll will return a false error to give time to the marker to show.
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
			return errors.Wrapf(err, "spell check text marker is not enabled for language %s", language.languageName)
		}
		return errors.Wrap(err, "could not evaluate spellCheckEnabled")
	}

	suggestionItem := nodewith.Role(role.MenuItem).Name(language.correct)
	if err := uiauto.Combine("Check for spelling check suggestion",
		ui.DoubleClick(textArea),
		ui.RightClick(textArea),
		ui.LeftClick(suggestionItem),
	)(ctx); err != nil {
		return errors.Wrapf(err, "failed to find expected suggestion %s", language.correct)
	}
	return nil
}
