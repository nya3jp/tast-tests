// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
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
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SpellCheckServiceEnabled,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of SpellCheckServiceEnabled policy",
		Contacts: []string{
			"phweiss@google.com", // Test author
			"pmarko@google.com",  // Policy owner
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
		Data: []string{"spell_checking.html"},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.SpellCheckServiceEnabled{}, pci.VerifiedFunctionalityUI),
		},
	})
}

func SpellCheckServiceEnabled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Setup and start webserver (implicitly provides data form above).
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.SpellCheckServiceEnabled
		// wantRestriction is whether the relevant buttons should be disabled.
		wantRestriction restriction.Restriction
		// wantSettingsCheck is the desired state of the settings toggle.
		wantSettingsCheck checked.Checked
		// wantContextCheck states whether the context menu checkmark should be there.
		wantContextCheck checked.Checked
	}{
		{
			name:              "allow",
			value:             &policy.SpellCheckServiceEnabled{Val: true},
			wantRestriction:   restriction.Disabled,
			wantSettingsCheck: checked.True,
			wantContextCheck:  checked.True,
		},
		{
			name:              "disallow",
			value:             &policy.SpellCheckServiceEnabled{Val: false},
			wantRestriction:   restriction.Disabled,
			wantSettingsCheck: checked.False,
			// "" means that there is no checkmark.
			wantContextCheck: "",
		},
		{
			name:              "unset",
			value:             &policy.SpellCheckServiceEnabled{Stat: policy.StatusUnset},
			wantRestriction:   restriction.None,
			wantSettingsCheck: checked.False,
			// "" means that there is no checkmark.
			wantContextCheck: "",
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Setup the browser for lacros tests after the policy was set.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Inside ChromeOS settings, check that the button is restricted and set to the correct value.
			if err := policyutil.OSSettingsPage(ctx, cr, "osSyncSetup").
				SelectNode(ctx, nodewith.
					Role(role.ToggleButton).
					NameStartingWith("Enhanced spell check")).
				Restriction(param.wantRestriction).
				Checked(param.wantSettingsCheck).
				Verify(); err != nil {
				s.Error("Unexpected OS settings state: ", err)
			}

			if param.wantRestriction == restriction.Disabled {
				// Check for the enterprise icon.
				if err := policyutil.OSSettingsPage(ctx, cr, "osSyncSetup").
					SelectNode(ctx, nodewith.
						Role(role.Image).
						NameStartingWith("Enhanced spell check")).
					Verify(); err != nil {
					s.Error("Unexpected OS settings state: ", err)
				}
			}

			// Open the browser and navigate to a page that contains an input field with the word "missspelled".
			url := server.URL + "/spell_checking.html"
			conn, err := br.NewConn(ctx, url)
			if err != nil {
				s.Fatal("Failed to connect to the browser: ", err)
			}
			defer conn.Close()

			textfield := nodewith.Role(role.InlineTextBox).Name("missspelled")

			ui := uiauto.New(tconn)
			if err := ui.RightClick(textfield)(ctx); err != nil {
				s.Fatal("Failed to right click text field: ", err)
			}
			if err := ui.LeftClick(nodewith.Role(role.MenuItem).Name("Spell check"))(ctx); err != nil {
				s.Fatal("Failed to left click 'Spell check >': ", err)
			}

			menuItem, err := ui.Info(ctx, nodewith.ClassName("MenuItemView").Name("Use enhanced spell check"))
			if err != nil {
				s.Fatal("Failed to get info for MenuItemCheckBox: ", err)
			}

			if param.wantRestriction != menuItem.Restriction {
				s.Errorf("Menu item in wrong restriction state: want=%s, actual=%s",
					param.wantRestriction, menuItem.Restriction)
			}

			// If the checkmark is there, menuItem.Checked is checked.True (="true"),
			// otherwise it is "".
			if param.wantContextCheck != menuItem.Checked {
				s.Errorf("Menu item in wrong checking state: want=%s, actual=%s", param.
					wantContextCheck, menuItem.Checked)
			}
		})
	}
}
