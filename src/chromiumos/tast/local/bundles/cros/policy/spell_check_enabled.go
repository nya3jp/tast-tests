// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
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
		Desc:         "Checks that the SpellCheckEnabled policy is correctly reflected on the Chrome Settings page",
		Contacts: []string{
			"jityao@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome", "lacros"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.LacrosPolicyLoggedIn,
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
		name  string
		value *policy.SpellcheckEnabled
	}{
		{
			name:  "enabled",
			value: &policy.SpellcheckEnabled{Val: true},
		},
		{
			name:  "disabled",
			value: &policy.SpellcheckEnabled{Val: false},
		},
		{
			name:  "unset",
			value: &policy.SpellcheckEnabled{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open lacros browser.
			br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), browser.TypeLacros)
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			conn, err := br.NewConn(ctx, "")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}

			// Open Language settings page.
			if err := conn.Navigate(ctx, "chrome://settings/languages"); err != nil {
				s.Fatal("Failed to open Language settings page: ", err)
			}
			defer conn.Close()

			// Create a uiauto.Context with default timeout.
			ui := uiauto.New(tconn)

			spellCheckFinder := nodewith.Name("Spell check").Role(role.ToggleButton)
			if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(spellCheckFinder)(ctx); err != nil {
				s.Fatal("Failed to find Spell check toggle: ", err)
			}

			var info *uiauto.NodeInfo
			if info, err = ui.WithTimeout(5*time.Second).Info(ctx, spellCheckFinder); err != nil {
				s.Fatal("Failed to get node info for Spell check toggle: ", err)
			}

			if param.value.Stat != policy.StatusSet {
				// Policy is unset, button should be enabled.
				if info.Restriction != restriction.None {
					b, _ := json.Marshal(info)
					s.Fatalf("Unexpected Spell check toggle state, expected enabled, got %s", b)
				}
			} else if param.value.Val {
				// Policy is enabled, button should be disabled and checked.
				if info.Checked != checked.True || info.Restriction != restriction.Disabled {
					b, _ := json.Marshal(info)
					s.Fatalf("Unexpected Spell check toggle state, expected checked and disabled, got %s", b)
				}
			} else {
				// Policy is disabled, button should be disabled and unchecked.
				if info.Checked != checked.False || info.Restriction != restriction.Disabled {
					b, _ := json.Marshal(info)
					s.Fatalf("Unexpected Spell check toggle state, expected unchecked and disabled, got %s", b)
				}
			}

			// Spell check is enabled if policy is enabled or unset.
			expected := param.value.Stat == policy.StatusUnset || param.value.Val
			actual := spellCheckEnabled(ctx, s, tconn, conn, ui)

			if actual != expected {
				s.Fatalf("Unexpected spellcheck enabled status, expected %v, got %v", expected, actual)
			}
		})
	}
}

func spellCheckEnabled(parent context.Context, s *testing.State, tconn *browser.TestConn, conn *browser.Conn, ui *uiauto.Context) bool {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()

	// Open a data URI of a page containing a textarea.
	if err := conn.Navigate(ctx, "data:text/html, <html><body><textarea aria-label='textarea'/>"); err != nil {
		s.Fatal("Failed to open page with textarea: ", err)
	}
	defer conn.Close()

	textArea := nodewith.Name("textarea")
	if err := uiauto.Combine("Focus textarea",
		ui.WaitUntilExists(textArea),
		ui.LeftClick(textArea),
	)(ctx); err != nil {
		s.Fatal("Failed to focus text area: ", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Wait 1 second to allow observer to be set up.
		testing.Sleep(ctx, 1*time.Second)

		kb, err := input.Keyboard(ctx)
		if err != nil {
			s.Fatal("Could not check create input keyboard: ", err)
		}
		defer kb.Close()

		// Type misspelled word into keyboard to generate a spelling marker.
		if err := kb.Type(ctx, "abc "); err != nil {
			s.Fatal("Could not type on keyboard: ", err)
		}
	}()

	// Check if Spell Check is enabled by checking that the spelling marker is added after text is typed.
	enabled := false
	if err := tconn.Call(ctx, &enabled, `(timeout) => {
		let observer;
		let observerPromise = new Promise((resolve) => {
			observer = chrome.automation.addTreeChangeObserver('textMarkerChanges', (treeChange) => {
				if (!treeChange.target.markers || treeChange.target.markers.length == 0) {
					resolve(false);
					return;
				}

				// Check that node change is a spelling marker.
				resolve(treeChange.target.markers[0].flags.spelling);
			});
		});

		return Promise.race([
			observerPromise,
			// Fail if no change was observed after timeout is reached.
			new Promise((resolve) => {
				setTimeout(() => resolve(false), timeout);
			})
		]).then((result) => {
			if (observer) {
				chrome.automation.removeTreeChangeObserver(observer);
			}

			return result;
		}).catch((err) => {
			if (observer) {
				chrome.automation.removeTreeChangeObserver(observer);
			}
			throw err;
		});
	}`, 3000); err != nil {
		s.Fatal("Could not observe spelling marker changes: ", err)
	}

	wg.Wait()

	return enabled
}
