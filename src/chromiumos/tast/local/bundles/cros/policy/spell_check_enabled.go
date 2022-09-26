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
		Func:         SpellCheckEnabled,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that the SpellCheckEnabled policy is correctly applied",
		Contacts: []string{
			"jityao@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome", "lacros"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.LacrosPolicyLoggedIn,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.SpellcheckEnabled{}, pci.VerifiedFunctionalityUI),
		},
	})
}

func SpellCheckEnabled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name                  string
		value                 *policy.SpellcheckEnabled
		wantRestriction       restriction.Restriction
		wantChecked           checked.Checked
		wantSpellCheckEnabled bool
	}{
		{
			name:                  "enabled",
			value:                 &policy.SpellcheckEnabled{Val: true},
			wantRestriction:       restriction.Disabled,
			wantChecked:           checked.True,
			wantSpellCheckEnabled: true,
		},
		{
			name:                  "disabled",
			value:                 &policy.SpellcheckEnabled{Val: false},
			wantRestriction:       restriction.Disabled,
			wantChecked:           checked.False,
			wantSpellCheckEnabled: false,
		},
		{
			name:                  "unset",
			value:                 &policy.SpellcheckEnabled{Stat: policy.StatusUnset},
			wantRestriction:       restriction.None,
			wantChecked:           checked.True,
			wantSpellCheckEnabled: true,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open lacros browser.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, browser.TypeLacros)
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			if err := policyutil.SettingsPage(ctx, cr, br, "languages").
				SelectNode(ctx, nodewith.
					Name("Check for spelling errors when you type text on web pages").
					Role(role.ToggleButton)).
				Restriction(param.wantRestriction).
				Checked(param.wantChecked).
				Verify(); err != nil {
				s.Fatal("Unexpected toggle button state: ", err)
			}

			hasSpellCheck, err := spellCheckEnabled(ctx, param.name, tconn, br)
			if err != nil {
				s.Fatal("Failed to check spell check status: ", err)
			}

			if param.wantSpellCheckEnabled != hasSpellCheck {
				s.Fatalf("Unexpected spellcheck enabled status, expected %v, got %v", param.wantChecked, hasSpellCheck)
			}
		})
	}
}

func spellCheckEnabled(ctx context.Context, name string, tconn *browser.TestConn, br *browser.Browser) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Open a data URI of a page containing a textarea.
	conn, err := br.NewConn(ctx, "data:text/html, <html><body><textarea aria-label='textarea'/>")
	if err != nil {
		return false, errors.Wrap(err, "failed to open page with textarea")
	}
	defer conn.Close()

	ui := uiauto.New(tconn)
	textArea := nodewith.Name("textarea")
	if err := uiauto.Combine("Focus textarea",
		ui.WaitUntilExists(textArea),
		ui.LeftClick(textArea),
	)(ctx); err != nil {
		return false, errors.Wrap(err, "failed to focus text area")
	}

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
	}`, name); err != nil {
		return false, errors.Wrap(err, "could not observe spelling marker changes")
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return false, errors.Wrap(err, "could not create input keyboard")
	}
	defer kb.Close()

	// Type misspelled word into keyboard to generate a spelling marker.
	if err := kb.Type(ctx, "aaaaa "); err != nil {
		return false, errors.Wrap(err, "could not type on keyboard")
	}

	enabled := false
	falseErr := errors.New("spellCheckEnabled evaluated to false")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := tconn.Eval(ctx, "spellCheckEnabled_"+name, &enabled); err != nil {
			return testing.PollBreak(err)
		}
		if !enabled {
			return falseErr
		}
		return nil
	}, &testing.PollOptions{Interval: 10 * time.Millisecond, Timeout: 1 * time.Second}); err != nil {
		if errors.Is(err, falseErr) {
			return false, nil
		}
		return false, errors.Wrap(err, "could not evaluate spellCheckEnabled")
	}

	return enabled, nil
}
